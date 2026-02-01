package driver

import (
	"encoding/binary"
	"fmt"
	"io"
)

// logEntry represents a single log entry from the Docker daemon.
// This mirrors the protobuf LogEntry from moby/daemon/logger/internal/logdriver.
type logEntry struct {
	Source              string
	TimeNano            int64
	Line                []byte
	Partial             bool
	PartialLogMetadata  *partialLogMetadata
}

type partialLogMetadata struct {
	Last    bool
	ID      string
	Ordinal int32
}

// logEntryDecoder reads length-prefixed protobuf log entries from a reader.
type logEntryDecoder struct {
	r      io.Reader
	lenBuf [4]byte
	buf    []byte
}

func newLogEntryDecoder(r io.Reader) *logEntryDecoder {
	return &logEntryDecoder{
		r:   r,
		buf: make([]byte, 1024),
	}
}

// decode reads and decodes the next log entry.
//
// Wire format: 4-byte big-endian uint32 length, then protobuf bytes.
// Protobuf fields (from entry.proto):
//
//	field 1 (string): source
//	field 2 (int64):  time_nano
//	field 3 (bytes):  line
//	field 4 (bool):   partial
//	field 5 (message): partial_log_metadata
//	  field 1 (bool):   last
//	  field 2 (string):  id
//	  field 3 (int32):   ordinal
func (d *logEntryDecoder) decode(entry *logEntry) error {
	// Read message length
	if _, err := io.ReadFull(d.r, d.lenBuf[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(d.lenBuf[:])
	if size == 0 {
		return nil
	}

	// Grow buffer if needed
	if int(size) > len(d.buf) {
		d.buf = make([]byte, size)
	}
	data := d.buf[:size]

	if _, err := io.ReadFull(d.r, data); err != nil {
		return err
	}

	// Reset entry
	entry.Source = ""
	entry.TimeNano = 0
	entry.Line = entry.Line[:0]
	entry.Partial = false
	entry.PartialLogMetadata = nil

	return unmarshalLogEntry(data, entry)
}

// unmarshalLogEntry decodes a protobuf-encoded LogEntry.
// Hand-rolled decoder to avoid depending on gogo/protobuf.
func unmarshalLogEntry(data []byte, entry *logEntry) error {
	for len(data) > 0 {
		// Read field tag (varint)
		tag, n := decodeVarint(data)
		if n == 0 {
			return fmt.Errorf("bad varint in tag")
		}
		data = data[n:]

		fieldNumber := tag >> 3
		wireType := tag & 0x7

		switch fieldNumber {
		case 1: // source (string, wire type 2 = length-delimited)
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for source", wireType)
			}
			s, rest, err := decodeBytes(data)
			if err != nil {
				return fmt.Errorf("decoding source: %w", err)
			}
			entry.Source = string(s)
			data = rest

		case 2: // time_nano (int64, wire type 0 = varint)
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for time_nano", wireType)
			}
			v, n := decodeVarint(data)
			if n == 0 {
				return fmt.Errorf("bad varint for time_nano")
			}
			entry.TimeNano = int64(v)
			data = data[n:]

		case 3: // line (bytes, wire type 2)
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for line", wireType)
			}
			s, rest, err := decodeBytes(data)
			if err != nil {
				return fmt.Errorf("decoding line: %w", err)
			}
			entry.Line = append(entry.Line[:0], s...)
			data = rest

		case 4: // partial (bool, wire type 0)
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for partial", wireType)
			}
			v, n := decodeVarint(data)
			if n == 0 {
				return fmt.Errorf("bad varint for partial")
			}
			entry.Partial = v != 0
			data = data[n:]

		case 5: // partial_log_metadata (message, wire type 2)
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for partial_log_metadata", wireType)
			}
			s, rest, err := decodeBytes(data)
			if err != nil {
				return fmt.Errorf("decoding partial_log_metadata: %w", err)
			}
			meta := &partialLogMetadata{}
			if err := unmarshalPartialMeta(s, meta); err != nil {
				return fmt.Errorf("decoding partial metadata: %w", err)
			}
			entry.PartialLogMetadata = meta
			data = rest

		default:
			// Skip unknown field
			rest, err := skipField(data, wireType)
			if err != nil {
				return fmt.Errorf("skipping field %d: %w", fieldNumber, err)
			}
			data = rest
		}
	}
	return nil
}

func unmarshalPartialMeta(data []byte, meta *partialLogMetadata) error {
	for len(data) > 0 {
		tag, n := decodeVarint(data)
		if n == 0 {
			return fmt.Errorf("bad varint in tag")
		}
		data = data[n:]

		fieldNumber := tag >> 3
		wireType := tag & 0x7

		switch fieldNumber {
		case 1: // last (bool)
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for last", wireType)
			}
			v, n := decodeVarint(data)
			if n == 0 {
				return fmt.Errorf("bad varint for last")
			}
			meta.Last = v != 0
			data = data[n:]

		case 2: // id (string)
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for id", wireType)
			}
			s, rest, err := decodeBytes(data)
			if err != nil {
				return fmt.Errorf("decoding id: %w", err)
			}
			meta.ID = string(s)
			data = rest

		case 3: // ordinal (int32)
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for ordinal", wireType)
			}
			v, n := decodeVarint(data)
			if n == 0 {
				return fmt.Errorf("bad varint for ordinal")
			}
			meta.Ordinal = int32(v)
			data = data[n:]

		default:
			rest, err := skipField(data, wireType)
			if err != nil {
				return fmt.Errorf("skipping field %d: %w", fieldNumber, err)
			}
			data = rest
		}
	}
	return nil
}

// --- protobuf primitives ---

func decodeVarint(data []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range data {
		if i >= 10 {
			return 0, 0
		}
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0
}

func decodeBytes(data []byte) ([]byte, []byte, error) {
	length, n := decodeVarint(data)
	if n == 0 {
		return nil, nil, fmt.Errorf("bad varint for length")
	}
	data = data[n:]
	if uint64(len(data)) < length {
		return nil, nil, fmt.Errorf("not enough data: need %d, have %d", length, len(data))
	}
	return data[:length], data[length:], nil
}

func skipField(data []byte, wireType uint64) ([]byte, error) {
	switch wireType {
	case 0: // varint
		_, n := decodeVarint(data)
		if n == 0 {
			return nil, fmt.Errorf("bad varint")
		}
		return data[n:], nil
	case 1: // 64-bit
		if len(data) < 8 {
			return nil, fmt.Errorf("not enough data for 64-bit field")
		}
		return data[8:], nil
	case 2: // length-delimited
		_, rest, err := decodeBytes(data)
		return rest, err
	case 5: // 32-bit
		if len(data) < 4 {
			return nil, fmt.Errorf("not enough data for 32-bit field")
		}
		return data[4:], nil
	default:
		return nil, fmt.Errorf("unknown wire type %d", wireType)
	}
}
