package driver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLogErrorRateLimiting(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	lc := &logConsumer{}

	// First error should be logged immediately
	lc.logError("error 1")
	time.Sleep(10 * time.Millisecond) // give time for write

	// Rapid-fire errors within 1 minute should be suppressed
	for i := 2; i <= 10; i++ {
		lc.logError("error %d", i)
	}
	time.Sleep(10 * time.Millisecond)

	// Check we only logged the first error
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stderr = oldStderr

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 1 {
		t.Errorf("expected 1 log line during cooldown, got %d:\n%s", len(lines), output)
	}
	if !strings.Contains(output, "error 1") {
		t.Errorf("expected first error to be logged, got: %s", output)
	}

	// Check suppressed count
	if lc.suppressedErrs != 9 {
		t.Errorf("expected 9 suppressed errors, got %d", lc.suppressedErrs)
	}
}

func TestLogErrorAfterCooldown(t *testing.T) {
	lc := &logConsumer{}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Log first error
	lc.logError("error 1")

	// Simulate time passing by directly manipulating lastErrLog
	lc.lastErrLog = time.Now().Add(-61 * time.Second)

	// Suppress some errors (simulate)
	lc.suppressedErrs = 5

	// Log another error after cooldown
	lc.logError("error 2")
	time.Sleep(10 * time.Millisecond)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stderr = oldStderr

	output := buf.String()

	// Should see both "suppressed N errors" and the new error
	if !strings.Contains(output, "suppressed 5 errors") {
		t.Errorf("expected suppressed count message, got: %s", output)
	}
	if !strings.Contains(output, "error 2") {
		t.Errorf("expected second error to be logged, got: %s", output)
	}
	if lc.suppressedErrs != 0 {
		t.Errorf("expected suppressed counter reset, got %d", lc.suppressedErrs)
	}
}

func TestStopLoggingDrainsFifo(t *testing.T) {
	var mu sync.Mutex
	var count int
	send := func(message string, priority Priority, vars map[string]string) error {
		time.Sleep(500 * time.Microsecond) // simulate slow journald
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}
	d := NewWithSendFunc(send)

	dir := t.TempDir()
	fifo := filepath.Join(dir, "log.fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}

	// Like dockerd, hold the FIFO write end open before StartLogging
	// (O_WRONLY open blocks until the driver opens the read end).
	const n = 200
	written := make(chan error, 1)
	go func() {
		w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			written <- err
			return
		}
		for i := range n {
			entry := buildLogEntry("stdout", int64(i+1), fmt.Sprintf("line %d", i), false)
			var frame [4]byte
			binary.BigEndian.PutUint32(frame[:], uint32(len(entry)))
			if _, err := w.Write(append(frame[:], entry...)); err != nil {
				w.Close()
				written <- err
				return
			}
		}
		written <- w.Close()
	}()

	req := fmt.Sprintf(`{"File":%q,"Info":{"Config":{"multiline-regex":""},"ContainerID":"abcdef123456","ContainerName":"/test"}}`, fifo)
	if resp := d.handleStartLogging([]byte(req)); string(resp) != `{"Err":""}` {
		t.Fatalf("start: %s", resp)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}

	// Docker sends StopLogging after the container exits; buffered entries
	// must still be drained before responding.
	d.handleStopLogging(fmt.Appendf(nil, `{"File":%q}`, fifo))

	mu.Lock()
	got := count
	mu.Unlock()
	if got != n {
		t.Errorf("got %d messages, want %d (%d entries lost at StopLogging)", got, n, n-got)
	}
}

// A second StartLogging for the same FIFO must cancel the previous
// consumer instead of orphaning its goroutine in the registry.
func TestStartLoggingReplacesExistingConsumer(t *testing.T) {
	defer func(d time.Duration) { stopDrainTimeout = d }(stopDrainTimeout)
	stopDrainTimeout = 100 * time.Millisecond

	d := NewWithSendFunc(func(string, Priority, map[string]string) error { return nil })

	dir := t.TempDir()
	fifo := filepath.Join(dir, "log.fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	opened := make(chan *os.File, 1)
	go func() {
		w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err == nil {
			opened <- w
		}
	}()

	req := fmt.Sprintf(`{"File":%q,"Info":{"Config":{},"ContainerID":"abcdef123456","ContainerName":"/test"}}`, fifo)
	if resp := d.handleStartLogging([]byte(req)); string(resp) != `{"Err":""}` {
		t.Fatalf("start 1: %s", resp)
	}
	w := <-opened
	defer w.Close()

	d.mu.Lock()
	first := d.consumers[fifo]
	d.mu.Unlock()

	if resp := d.handleStartLogging([]byte(req)); string(resp) != `{"Err":""}` {
		t.Fatalf("start 2: %s", resp)
	}

	select {
	case <-first.done:
	case <-time.After(10 * time.Second):
		t.Fatal("first consumer not stopped by second StartLogging")
	}

	d.mu.Lock()
	second := d.consumers[fifo]
	n := len(d.consumers)
	d.mu.Unlock()
	if second == first || second == nil || n != 1 {
		t.Errorf("registry not replaced: %d entries, replaced=%v", n, second != first)
	}

	d.handleStopLogging(fmt.Appendf(nil, `{"File":%q}`, fifo))
}

// A consumer that exits at EOF must remove itself from the registry
// without waiting for a StopLogging that may never arrive.
func TestConsumerRemovesItselfOnEOF(t *testing.T) {
	d := NewWithSendFunc(func(string, Priority, map[string]string) error { return nil })

	dir := t.TempDir()
	fifo := filepath.Join(dir, "log.fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	go func() {
		if w, err := os.OpenFile(fifo, os.O_WRONLY, 0); err == nil {
			w.Close() // immediate EOF for the consumer
		}
	}()

	req := fmt.Sprintf(`{"File":%q,"Info":{"Config":{},"ContainerID":"abcdef123456","ContainerName":"/test"}}`, fifo)
	if resp := d.handleStartLogging([]byte(req)); string(resp) != `{"Err":""}` {
		t.Fatalf("start: %s", resp)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		d.mu.Lock()
		n := len(d.consumers)
		d.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d consumers still registered after EOF", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
