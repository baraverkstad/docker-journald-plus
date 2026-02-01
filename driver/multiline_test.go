package driver

import (
	"sync"
	"testing"
	"time"
)

type collectedMessages struct {
	mu   sync.Mutex
	msgs []mergedMessage
}

func (c *collectedMessages) add(msg mergedMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
}

func (c *collectedMessages) get() []mergedMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]mergedMessage, len(c.msgs))
	copy(out, c.msgs)
	return out
}

func TestMultilineDisabled(t *testing.T) {
	cfg := mustConfig(t, map[string]string{"multiline-regex": ""})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("line 1"), "stdout", 1000)
	m.AddLine([]byte("  line 2"), "stdout", 2000)
	m.AddLine([]byte("line 3"), "stdout", 3000)

	msgs := collected.get()
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	if string(msgs[1].Line) != "  line 2" {
		t.Errorf("msg[1] = %q, want %q", string(msgs[1].Line), "  line 2")
	}
}

func TestMultilineMergesWhitespace(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"multiline-timeout": "1s", // long timeout so we control flushing
	})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("first line"), "stdout", 1000)
	m.AddLine([]byte("  continuation 1"), "stdout", 2000)
	m.AddLine([]byte("\tcontinuation 2"), "stdout", 3000)
	// New non-continuation line triggers flush of previous
	m.AddLine([]byte("second line"), "stdout", 4000)
	m.Flush() // flush the second

	msgs := collected.get()
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	want := "first line\n  continuation 1\n\tcontinuation 2"
	if string(msgs[0].Line) != want {
		t.Errorf("msg[0] = %q, want %q", string(msgs[0].Line), want)
	}
	if msgs[0].TimeNano != 1000 {
		t.Errorf("msg[0].TimeNano = %d, want 1000", msgs[0].TimeNano)
	}
	if string(msgs[1].Line) != "second line" {
		t.Errorf("msg[1] = %q, want %q", string(msgs[1].Line), "second line")
	}
}

func TestMultilineTimeout(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"multiline-timeout": "20ms",
	})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("line"), "stdout", 1000)
	m.AddLine([]byte("  cont"), "stdout", 2000)

	// Wait for timeout to fire
	time.Sleep(50 * time.Millisecond)

	msgs := collected.get()
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (timeout should have flushed)", len(msgs))
	}
	if string(msgs[0].Line) != "line\n  cont" {
		t.Errorf("msg = %q, want %q", string(msgs[0].Line), "line\n  cont")
	}
}

func TestMultilineMaxLines(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"multiline-max-lines": "3",
		"multiline-timeout":   "1s",
	})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("line 1"), "stdout", 1000)
	m.AddLine([]byte(" cont 1"), "stdout", 2000)
	m.AddLine([]byte(" cont 2"), "stdout", 3000)
	// Next continuation exceeds max-lines=3, so previous is flushed
	m.AddLine([]byte(" cont 3"), "stdout", 4000)
	m.Flush()

	msgs := collected.get()
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if string(msgs[0].Line) != "line 1\n cont 1\n cont 2" {
		t.Errorf("msg[0] = %q", string(msgs[0].Line))
	}
}

func TestMultilineMaxBytes(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"multiline-max-bytes": "20",
		"multiline-timeout":   "1s",
	})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("12345678901234"), "stdout", 1000) // 14 bytes
	// continuation would be 14 + 1 (sep) + 7 = 22 > 20
	m.AddLine([]byte(" cont2"), "stdout", 2000)
	m.Flush()

	msgs := collected.get()
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
}

func TestMultilineCustomSeparator(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"multiline-separator": " | ",
		"multiline-timeout":   "1s",
	})
	var collected collectedMessages
	m := newMultilineMerger(cfg, collected.add)

	m.AddLine([]byte("first"), "stdout", 1000)
	m.AddLine([]byte("  second"), "stdout", 2000)
	m.Flush()

	msgs := collected.get()
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if string(msgs[0].Line) != "first |   second" {
		t.Errorf("msg = %q, want %q", string(msgs[0].Line), "first |   second")
	}
}
