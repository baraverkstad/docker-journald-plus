package driver

import (
	"sort"
)

// maxPartialBytes caps per-group buffering; an oversize group is flushed
// as a complete line so a container writing without newlines cannot OOM
// the plugin. Remaining parts flush as separate entries.
const maxPartialBytes = 1 << 20

// maxPartialGroups caps in-flight groups; dockerd interleaves at most a
// few partial IDs per stream, so more indicates groups whose Last entry
// never arrived. The oldest is dropped.
const maxPartialGroups = 16

// partialAssembler reassembles Docker partial log messages.
// Docker splits lines >16KB into multiple entries sharing a partial ID.
type partialAssembler struct {
	groups map[string]*partialGroup
	seq    int
}

type partialGroup struct {
	source   string
	timeNano int64
	seq      int
	bytes    int
	parts    []partialPart
}

type partialPart struct {
	ordinal int32
	data    []byte
}

func newPartialAssembler() *partialAssembler {
	return &partialAssembler{
		groups: make(map[string]*partialGroup),
	}
}

// Add processes a log entry. Returns a complete line and true if the entry
// completed a message, or nil and false if it was buffered as a partial.
func (pa *partialAssembler) Add(entry *logEntry) (line []byte, source string, timeNano int64, complete bool) {
	// Not a partial -- return as-is
	if !entry.Partial {
		return entry.Line, entry.Source, entry.TimeNano, true
	}

	meta := entry.PartialLogMetadata
	if meta == nil {
		// Partial flag set but no metadata -- treat as complete
		return entry.Line, entry.Source, entry.TimeNano, true
	}

	id := meta.ID
	g, ok := pa.groups[id]
	if !ok {
		pa.evictStale()
		pa.seq++
		g = &partialGroup{
			source:   entry.Source,
			timeNano: entry.TimeNano,
			seq:      pa.seq,
		}
		pa.groups[id] = g
	}

	g.parts = append(g.parts, partialPart{
		ordinal: meta.Ordinal,
		data:    append([]byte(nil), entry.Line...),
	})
	g.bytes += len(entry.Line)

	if !meta.Last && g.bytes < maxPartialBytes {
		return nil, "", 0, false
	}

	// Assemble complete message
	sort.Slice(g.parts, func(i, j int) bool {
		return g.parts[i].ordinal < g.parts[j].ordinal
	})

	var total int
	for _, p := range g.parts {
		total += len(p.data)
	}
	assembled := make([]byte, 0, total)
	for _, p := range g.parts {
		assembled = append(assembled, p.data...)
	}

	source = g.source
	timeNano = g.timeNano
	delete(pa.groups, id)

	return assembled, source, timeNano, true
}

// evictStale drops the oldest group when the group cap is reached; its
// Last entry never arrived, so it would never complete anyway.
func (pa *partialAssembler) evictStale() {
	if len(pa.groups) < maxPartialGroups {
		return
	}
	var oldestID string
	oldestSeq := pa.seq + 1
	for id, g := range pa.groups {
		if g.seq < oldestSeq {
			oldestSeq = g.seq
			oldestID = id
		}
	}
	delete(pa.groups, oldestID)
}
