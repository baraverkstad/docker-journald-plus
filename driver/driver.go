package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
	"github.com/docker/go-plugins-helpers/sdk"
)

// Driver implements the Docker log driver plugin protocol.
type Driver struct {
	mu        sync.Mutex
	consumers map[string]*logConsumer // keyed by FIFO path
	sendFn    JournalSendFunc        // injectable for testing
}

// logConsumer tracks state for a single container's log stream.
type logConsumer struct {
	fifoPath string
	cfg      *Config
	writer   *journalWriter
	cancel   context.CancelFunc
	done     chan struct{}
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

// RegisterHandlers wires up the HTTP endpoints on the plugin handler.
func (d *Driver) RegisterHandlers(h sdk.Handler) {
	h.HandleFunc("/LogDriver.StartLogging", d.handleStartLogging)
	h.HandleFunc("/LogDriver.StopLogging", d.handleStopLogging)
	h.HandleFunc("/LogDriver.Capabilities", d.handleCapabilities)
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

// CapabilitiesResponse tells Docker what the driver supports.
type CapabilitiesResponse struct {
	Cap capability `json:"Cap"`
	Err string     `json:"Err"`
}

type capability struct {
	ReadLogs bool `json:"ReadLogs"`
}

type errResponse struct {
	Err string `json:"Err"`
}

// --- Handlers ---

func (d *Driver) handleStartLogging(w http.ResponseWriter, r *http.Request) {
	var req StartLoggingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, fmt.Errorf("decoding request: %w", err))
		return
	}

	// Parse container info to get Config map
	var info containerInfo
	if err := json.Unmarshal(req.Info, &info); err != nil {
		respondErr(w, fmt.Errorf("parsing container info: %w", err))
		return
	}

	cfg, err := ParseConfig(info.Config)
	if err != nil {
		respondErr(w, fmt.Errorf("invalid log options: %w", err))
		return
	}

	writer, err := newJournalWriter(cfg, req.Info, d.sendFn)
	if err != nil {
		respondErr(w, fmt.Errorf("creating journal writer: %w", err))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	f, err := fifo.OpenFifo(ctx, req.File, syscall.O_RDONLY, 0)
	if err != nil {
		cancel()
		respondErr(w, fmt.Errorf("opening fifo %s: %w", req.File, err))
		return
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

	respondOK(w)
}

func (d *Driver) handleStopLogging(w http.ResponseWriter, r *http.Request) {
	var req StopLoggingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, fmt.Errorf("decoding request: %w", err))
		return
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

	respondOK(w)
}

func (d *Driver) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	resp := CapabilitiesResponse{
		Cap: capability{ReadLogs: false},
	}
	json.NewEncoder(w).Encode(resp)
}

// consumeLog reads log entries from the FIFO, reassembles partials,
// merges multiline, detects priority, and writes to journald.
func (d *Driver) consumeLog(ctx context.Context, f io.ReadCloser, lc *logConsumer) {
	defer close(lc.done)
	defer f.Close()

	partial := newPartialAssembler()

	merger := newMultilineMerger(lc.cfg, func(msg mergedMessage) {
		// Detect priority and write to journal
		pri, line := DetectPriority(lc.cfg, msg.Line, msg.Source)
		if err := lc.writer.Write(msg, pri, line); err != nil {
			fmt.Printf("journald-plus: error writing to journal: %v\n", err)
		}
	})

	dec := newLogEntryDecoder(f)
	for {
		var entry logEntry
		if err := dec.decode(&entry); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				break
			}
			fmt.Printf("journald-plus: error decoding log entry: %v\n", err)
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

func respondOK(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(errResponse{Err: ""})
}

func respondErr(w http.ResponseWriter, err error) {
	json.NewEncoder(w).Encode(errResponse{Err: err.Error()})
}
