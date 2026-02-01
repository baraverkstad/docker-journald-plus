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
}

// logConsumer tracks state for a single container's log stream.
type logConsumer struct {
	fifoPath string
	info     StartLoggingRequest
	cancel   context.CancelFunc
	done     chan struct{}
}

// New creates a new Driver.
func New() *Driver {
	return &Driver{
		consumers: make(map[string]*logConsumer),
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
		info:     req,
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

// consumeLog reads log entries from the FIFO and processes them.
func (d *Driver) consumeLog(ctx context.Context, f io.ReadCloser, lc *logConsumer) {
	defer close(lc.done)
	defer f.Close()

	dec := newLogEntryDecoder(f)
	for {
		var entry logEntry
		if err := dec.decode(&entry); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			fmt.Printf("journald-plus: error decoding log entry: %v\n", err)
			return
		}

		// TODO: partial reassembly, multiline merge, priority detection, journald write
		_ = entry
	}
}

// --- HTTP helpers ---

func respondOK(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(errResponse{Err: ""})
}

func respondErr(w http.ResponseWriter, err error) {
	json.NewEncoder(w).Encode(errResponse{Err: err.Error()})
}
