package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Driver implements the Docker log driver plugin protocol.
type Driver struct {
	mu        sync.Mutex
	consumers map[string]*logConsumer // keyed by FIFO path
	sendFn    JournalSendFunc         // injectable for testing
}

// logConsumer tracks state for a single container's log stream.
type logConsumer struct {
	fifoPath string
	cfg      *Config
	writer   *journalWriter
	cancel   context.CancelFunc
	done     chan struct{}

	errMu          sync.Mutex
	lastErrLog     time.Time
	suppressedErrs int
}

// New creates a new Driver using the real journald send function.
func New() *Driver {
	return NewWithSendFunc(defaultJournalSend)
}

// NewWithSendFunc creates a Driver with a custom journal send function (for testing).
func NewWithSendFunc(sendFn JournalSendFunc) *Driver {
	return &Driver{
		consumers: make(map[string]*logConsumer),
		sendFn:    sendFn,
	}
}

// Routes returns the URL path → handler map for the plugin HTTP server.
func (d *Driver) Routes() map[string]func([]byte) []byte {
	return map[string]func([]byte) []byte{
		"/LogDriver.StartLogging": d.handleStartLogging,
		"/LogDriver.StopLogging":  d.handleStopLogging,
		"/LogDriver.Capabilities": d.handleCapabilities,
	}
}

// --- Request/Response types ---

// StartLoggingRequest is sent by Docker when a container starts.
type StartLoggingRequest struct {
	File string          `json:"File"`
	Info json.RawMessage `json:"Info"`
}

// StopLoggingRequest is sent by Docker when a container stops.
type StopLoggingRequest struct {
	File string `json:"File"`
}

type errResponse struct {
	Err string `json:"Err"`
}

// --- Handlers ---

func (d *Driver) handleStartLogging(body []byte) []byte {
	var req StartLoggingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return respondErr(fmt.Errorf("decoding request: %w", err))
	}

	// Parse container info to get Config map
	var info containerInfo
	if err := json.Unmarshal(req.Info, &info); err != nil {
		return respondErr(fmt.Errorf("parsing container info: %w", err))
	}

	cfg, err := ParseConfig(info.Config)
	if err != nil {
		return respondErr(fmt.Errorf("invalid log options: %w", err))
	}

	writer, err := newJournalWriter(cfg, info, d.sendFn)
	if err != nil {
		return respondErr(fmt.Errorf("creating journal writer: %w", err))
	}

	// Stop a previous consumer for the same FIFO instead of orphaning it
	d.mu.Lock()
	old := d.consumers[req.File]
	d.mu.Unlock()
	if old != nil {
		old.cancel()
		<-old.done
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	f, err := os.OpenFile(req.File, os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
	if err != nil {
		cancel()
		return respondErr(fmt.Errorf("opening fifo %s: %w", req.File, err))
	}

	lc := &logConsumer{
		fifoPath: req.File,
		cfg:      cfg,
		writer:   writer,
		cancel:   cancel,
		done:     done,
	}

	d.mu.Lock()
	d.consumers[req.File] = lc
	d.mu.Unlock()

	go d.consumeLog(ctx, f, lc)

	return respondOK()
}

func (d *Driver) handleStopLogging(body []byte) []byte {
	var req StopLoggingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return respondErr(fmt.Errorf("decoding request: %w", err))
	}

	d.mu.Lock()
	lc, ok := d.consumers[req.File]
	if ok {
		delete(d.consumers, req.File)
	}
	d.mu.Unlock()

	if ok {
		lc.cancel()
		<-lc.done // wait for consumer goroutine to finish draining
	}

	return respondOK()
}

func (d *Driver) handleCapabilities(_ []byte) []byte {
	return []byte(`{"Cap":{"ReadLogs":false},"Err":""}`)
}

// logError rate-limits error logging to prevent log floods.
// Logs at most 1 error per minute; suppressed errors are counted.
func (lc *logConsumer) logError(format string, args ...any) {
	lc.errMu.Lock()
	defer lc.errMu.Unlock()

	now := time.Now()
	elapsed := now.Sub(lc.lastErrLog)

	if elapsed >= time.Minute {
		if lc.suppressedErrs > 0 {
			fmt.Fprintf(os.Stderr, "journald-plus: suppressed %d errors in last %v\n",
				lc.suppressedErrs, elapsed.Round(time.Second))
			lc.suppressedErrs = 0
		}
		fmt.Fprintf(os.Stderr, "journald-plus: "+format+"\n", args...)
		lc.lastErrLog = now
	} else {
		lc.suppressedErrs++
	}
}

// stopDrainTimeout bounds FIFO draining after StopLogging. Dockerd closes
// the FIFO write end when the container exits, so EOF arrives naturally;
// the deadline only guards against a hung writer or stalled journald.
// Variable to allow shortening in tests.
var stopDrainTimeout = 5 * time.Second

// pollingReader retries EAGAIN reads on platforms where the runtime cannot
// poll FIFOs (e.g. macOS). On Linux, FIFO reads block in the poller and
// EAGAIN never surfaces, making this a pass-through.
type pollingReader struct {
	f          *os.File
	ctx        context.Context
	drainUntil time.Time
	seenData   bool
}

func (r *pollingReader) Read(p []byte) (int, error) {
	for {
		n, err := r.f.Read(p)
		if n > 0 {
			r.seenData = true
			r.drainUntil = time.Time{}
			return n, err
		}
		if err == io.EOF && !r.seenData && r.ctx.Err() == nil {
			if r.drainUntil.IsZero() {
				r.drainUntil = time.Now().Add(stopDrainTimeout)
			} else if time.Now().After(r.drainUntil) {
				return n, err
			}
			time.Sleep(time.Millisecond)
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) {
			return n, err
		}
		if r.ctx.Err() != nil {
			if r.drainUntil.IsZero() {
				r.drainUntil = time.Now().Add(stopDrainTimeout)
			} else if time.Now().After(r.drainUntil) {
				return 0, r.ctx.Err()
			}
		}
		time.Sleep(time.Millisecond)
	}
}

// consumeLog reads log entries from the FIFO, reassembles partials,
// merges multiline, detects priority, and writes to journald.
func (d *Driver) consumeLog(ctx context.Context, f *os.File, lc *logConsumer) {
	defer close(lc.done)
	// Deregister on exit (EOF/error) so the registry cannot grow when
	// StopLogging never arrives
	defer func() {
		d.mu.Lock()
		if d.consumers[lc.fifoPath] == lc {
			delete(d.consumers, lc.fifoPath)
		}
		d.mu.Unlock()
	}()
	defer f.Close()

	go func() {
		<-ctx.Done()
		// On unpollable FIFOs this fails; pollingReader bounds the drain instead.
		_ = f.SetReadDeadline(time.Now().Add(stopDrainTimeout))
	}()

	partial := newPartialAssembler()

	merger := newMultilineMerger(lc.cfg, func(msg mergedMessage) {
		line := msg.Line
		var fields map[string]string
		var priority Priority
		priorityDetected := false

		// Try JSON parsing first if enabled
		if parsed, ok := ParseJSONLog(lc.cfg, line); ok {
			// JSON parsing succeeded
			fields = parsed.ExtraFields

			// Use JSON message as log body, appending inline JSON if present
			msg := parsed.Message
			if parsed.InlineJSON != "" {
				msg = msg + " " + parsed.InlineJSON
			}
			if msg != "" {
				line = []byte(msg)
			}

			// Detect priority from JSON level field
			if parsed.Level != "" {
				if pri, ok := priorityNames[strings.ToLower(parsed.Level)]; ok {
					priority = pri
					priorityDetected = true
				}
			}
		}

		// Strip timestamp (before priority detection so ^ERROR matches after stripping)
		if lc.cfg.StripTimestamp {
			line = StripTimestamp(line, lc.cfg.StripTimestampRegex)
		}

		// Detect priority via regex/default if not already detected from JSON
		if !priorityDetected {
			priority, line = DetectPriority(lc.cfg, line, msg.Source)
		}

		// Strip priority level string from message if enabled
		if lc.cfg.StripPriority {
			line = StripPriority(line, lc.cfg.StripPriorityRegex)
		}

		// Normalize whitespace
		if lc.cfg.NormalizeWhitespace {
			line = NormalizeWhitespace(line)
		}

		// Write to journal with JSON fields
		if err := lc.writer.Write(msg, priority, line, fields); err != nil {
			lc.logError("error writing to journal: %v", err)
		}
	})

	dec := newLogEntryDecoder(&pollingReader{f: f, ctx: ctx})
	for {
		var entry logEntry
		if err := dec.decode(&entry); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				break
			}
			lc.logError("error decoding log entry: %v", err)
			break
		}

		// 1. Reassemble partial messages
		line, source, timeNano, complete := partial.Add(&entry)
		if !complete {
			continue
		}

		// 2. Feed into multiline merger
		merger.AddLine(line, source, timeNano)
	}

	// Flush remaining buffered content
	merger.Flush()
}

// --- HTTP helpers ---

func respondOK() []byte {
	return []byte(`{"Err":""}`)
}

func respondErr(err error) []byte {
	b, _ := json.Marshal(errResponse{Err: err.Error()})
	return b
}
