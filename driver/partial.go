package driver

import (
	"sort"
)

// partialAssembler reassembles Docker partial log messages.
// Docker splits lines >16KB into multiple entries sharing a partial ID.
type partialAssembler struct {
	groups map[string]*partialGroup
}

type partialGroup struct {
	source   string
	timeNano int64
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
		g = &partialGroup{
			source:   entry.Source,
			timeNano: entry.TimeNano,
		}
		pa.groups[id] = g
	}

	g.parts = append(g.parts, partialPart{
		ordinal: meta.Ordinal,
		data:    append([]byte(nil), entry.Line...),
	})

	if !meta.Last {
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
