package driver

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogErrorRateLimiting(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

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
	os.Stdout = oldStdout

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

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

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
	os.Stdout = oldStdout

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
