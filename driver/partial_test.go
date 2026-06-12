package driver

import "testing"

func TestPartialNonPartial(t *testing.T) {
	pa := newPartialAssembler()
	entry := &logEntry{
		Source:   "stdout",
		TimeNano: 1000,
		Line:     []byte("complete line"),
		Partial:  false,
	}

	line, source, timeNano, ok := pa.Add(entry)
	if !ok {
		t.Fatal("expected complete")
	}
	if string(line) != "complete line" {
		t.Errorf("line = %q", string(line))
	}
	if source != "stdout" {
		t.Errorf("source = %q", source)
	}
	if timeNano != 1000 {
		t.Errorf("timeNano = %d", timeNano)
	}
}

func TestPartialReassembly(t *testing.T) {
	pa := newPartialAssembler()

	// First partial
	_, _, _, ok := pa.Add(&logEntry{
		Source:   "stdout",
		TimeNano: 1000,
		Line:     []byte("part1"),
		Partial:  true,
		PartialLogMetadata: &partialLogMetadata{
			ID:      "abc",
			Ordinal: 0,
			Last:    false,
		},
	})
	if ok {
		t.Fatal("should not be complete yet")
	}

	// Second partial
	_, _, _, ok = pa.Add(&logEntry{
		Source:   "stdout",
		TimeNano: 2000,
		Line:     []byte("part2"),
		Partial:  true,
		PartialLogMetadata: &partialLogMetadata{
			ID:      "abc",
			Ordinal: 1,
			Last:    false,
		},
	})
	if ok {
		t.Fatal("should not be complete yet")
	}

	// Last partial
	line, source, timeNano, ok := pa.Add(&logEntry{
		Source:   "stdout",
		TimeNano: 3000,
		Line:     []byte("part3"),
		Partial:  true,
		PartialLogMetadata: &partialLogMetadata{
			ID:      "abc",
			Ordinal: 2,
			Last:    true,
		},
	})
	if !ok {
		t.Fatal("expected complete")
	}
	if string(line) != "part1part2part3" {
		t.Errorf("line = %q, want %q", string(line), "part1part2part3")
	}
	if source != "stdout" {
		t.Errorf("source = %q", source)
	}
	if timeNano != 1000 {
		t.Errorf("timeNano = %d, want 1000 (first partial)", timeNano)
	}
}

func TestPartialOutOfOrder(t *testing.T) {
	pa := newPartialAssembler()

	// Send ordinal 2, then 0, then 1 (last)
	pa.Add(&logEntry{
		Line: []byte("C"), Partial: true, Source: "stdout", TimeNano: 1000,
		PartialLogMetadata: &partialLogMetadata{ID: "x", Ordinal: 2, Last: false},
	})
	pa.Add(&logEntry{
		Line: []byte("A"), Partial: true, Source: "stdout", TimeNano: 2000,
		PartialLogMetadata: &partialLogMetadata{ID: "x", Ordinal: 0, Last: false},
	})
	line, _, _, ok := pa.Add(&logEntry{
		Line: []byte("B"), Partial: true, Source: "stdout", TimeNano: 3000,
		PartialLogMetadata: &partialLogMetadata{ID: "x", Ordinal: 1, Last: true},
	})

	if !ok {
		t.Fatal("expected complete")
	}
	if string(line) != "ABC" {
		t.Errorf("line = %q, want %q (sorted by ordinal)", string(line), "ABC")
	}
}

// A group exceeding the byte cap is flushed as-is instead of buffering
// the container's output without bound.
func TestPartialFlushesOversizeGroup(t *testing.T) {
	pa := newPartialAssembler()
	chunk := make([]byte, 16*1024)

	var flushed []byte
	parts := 0
	for range 2 * maxPartialBytes / len(chunk) {
		line, _, _, ok := pa.Add(&logEntry{
			Source:  "stdout",
			Line:    chunk,
			Partial: true,
			PartialLogMetadata: &partialLogMetadata{
				ID:      "big",
				Ordinal: int32(parts),
				Last:    false,
			},
		})
		parts++
		if ok {
			flushed = line
			break
		}
	}
	if flushed == nil {
		t.Fatalf("group never flushed after %d parts (%d bytes buffered)",
			parts, parts*len(chunk))
	}
	if len(flushed) > maxPartialBytes+len(chunk) {
		t.Errorf("flushed %d bytes, cap is %d", len(flushed), maxPartialBytes)
	}
}

// Groups whose Last entry never arrives must not accumulate forever.
func TestPartialEvictsStaleGroups(t *testing.T) {
	pa := newPartialAssembler()
	for i := range maxPartialGroups + 5 {
		pa.Add(&logEntry{
			Source:  "stdout",
			Line:    []byte("x"),
			Partial: true,
			PartialLogMetadata: &partialLogMetadata{
				ID:      string(rune('a' + i)),
				Ordinal: 0,
				Last:    false,
			},
		})
	}
	if len(pa.groups) > maxPartialGroups {
		t.Errorf("%d groups buffered, cap is %d", len(pa.groups), maxPartialGroups)
	}
}
