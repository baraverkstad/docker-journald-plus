package driver

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// encodeVarint encodes a uint64 as a protobuf varint.
func encodeVarint(v uint64) []byte {
	var buf [10]byte
	n := 0
	for v >= 0x80 {
		buf[n] = byte(v) | 0x80
		v >>= 7
		n++
	}
	buf[n] = byte(v)
	return buf[:n+1]
}

// encodeTag encodes a protobuf field tag.
func encodeTag(fieldNumber uint64, wireType uint64) []byte {
	return encodeVarint(fieldNumber<<3 | wireType)
}

// encodeString encodes a protobuf string/bytes field.
func encodeString(fieldNumber uint64, s string) []byte {
	var buf []byte
	buf = append(buf, encodeTag(fieldNumber, 2)...)
	buf = append(buf, encodeVarint(uint64(len(s)))...)
	buf = append(buf, s...)
	return buf
}

// encodeVarintField encodes a protobuf varint field.
func encodeVarintField(fieldNumber uint64, v uint64) []byte {
	var buf []byte
	buf = append(buf, encodeTag(fieldNumber, 0)...)
	buf = append(buf, encodeVarint(v)...)
	return buf
}

// buildLogEntry builds a protobuf-encoded LogEntry.
func buildLogEntry(source string, timeNano int64, line string, partial bool) []byte {
	var msg []byte
	if source != "" {
		msg = append(msg, encodeString(1, source)...)
	}
	if timeNano != 0 {
		msg = append(msg, encodeVarintField(2, uint64(timeNano))...)
	}
	if line != "" {
		msg = append(msg, encodeString(3, line)...)
	}
	if partial {
		msg = append(msg, encodeVarintField(4, 1)...)
	}
	return msg
}

// wrapWithLength wraps a protobuf message with a 4-byte big-endian length prefix.
func wrapWithLength(msg []byte) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(msg)))
	return append(buf[:], msg...)
}

func TestDecodeBasicEntry(t *testing.T) {
	msg := buildLogEntry("stdout", 1234567890, "hello world", false)
	data := wrapWithLength(msg)

	dec := newLogEntryDecoder(bytes.NewReader(data))
	var entry logEntry
	if err := dec.decode(&entry); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if entry.Source != "stdout" {
		t.Errorf("source = %q, want %q", entry.Source, "stdout")
	}
	if entry.TimeNano != 1234567890 {
		t.Errorf("timeNano = %d, want %d", entry.TimeNano, 1234567890)
	}
	if string(entry.Line) != "hello world" {
		t.Errorf("line = %q, want %q", string(entry.Line), "hello world")
	}
	if entry.Partial {
		t.Error("partial = true, want false")
	}
	if entry.PartialLogMetadata != nil {
		t.Error("partialLogMetadata is not nil")
	}
}

func TestDecodePartialEntry(t *testing.T) {
	// Build partial metadata sub-message
	var metaMsg []byte
	metaMsg = append(metaMsg, encodeVarintField(1, 0)...)   // last = false
	metaMsg = append(metaMsg, encodeString(2, "abc123")...) // id
	metaMsg = append(metaMsg, encodeVarintField(3, 2)...)   // ordinal = 2

	// Build full entry with partial metadata
	var msg []byte
	msg = append(msg, encodeString(1, "stderr")...)
	msg = append(msg, encodeVarintField(2, 9999)...)
	msg = append(msg, encodeString(3, "partial line")...)
	msg = append(msg, encodeVarintField(4, 1)...) // partial = true
	// field 5: partial_log_metadata (wire type 2)
	msg = append(msg, encodeTag(5, 2)...)
	msg = append(msg, encodeVarint(uint64(len(metaMsg)))...)
	msg = append(msg, metaMsg...)

	data := wrapWithLength(msg)

	dec := newLogEntryDecoder(bytes.NewReader(data))
	var entry logEntry
	if err := dec.decode(&entry); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if entry.Source != "stderr" {
		t.Errorf("source = %q, want %q", entry.Source, "stderr")
	}
	if !entry.Partial {
		t.Error("partial = false, want true")
	}
	if entry.PartialLogMetadata == nil {
		t.Fatal("partialLogMetadata is nil")
	}
	if entry.PartialLogMetadata.Last {
		t.Error("last = true, want false")
	}
	if entry.PartialLogMetadata.ID != "abc123" {
		t.Errorf("id = %q, want %q", entry.PartialLogMetadata.ID, "abc123")
	}
	if entry.PartialLogMetadata.Ordinal != 2 {
		t.Errorf("ordinal = %d, want %d", entry.PartialLogMetadata.Ordinal, 2)
	}
}

func TestDecodeMultipleEntries(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 3; i++ {
		msg := buildLogEntry("stdout", int64(i+1)*1000, "line", false)
		buf.Write(wrapWithLength(msg))
	}

	dec := newLogEntryDecoder(&buf)
	for i := 0; i < 3; i++ {
		var entry logEntry
		if err := dec.decode(&entry); err != nil {
			t.Fatalf("decode entry %d: %v", i, err)
		}
		if entry.TimeNano != int64(i+1)*1000 {
			t.Errorf("entry %d: timeNano = %d, want %d", i, entry.TimeNano, int64(i+1)*1000)
		}
	}
}
