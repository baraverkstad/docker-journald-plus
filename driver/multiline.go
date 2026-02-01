package driver

import (
	"bytes"
	"sync"
	"time"
)

// mergedMessage is a complete message after multiline merging.
type mergedMessage struct {
	Line      []byte
	Source    string
	TimeNano int64
}

// multilineMerger buffers and merges consecutive continuation lines.
type multilineMerger struct {
	cfg    *Config
	output func(mergedMessage)

	mu        sync.Mutex
	buf       bytes.Buffer
	lineCount int
	source    string
	timeNano  int64
	timer     *time.Timer
	hasData   bool
}

func newMultilineMerger(cfg *Config, output func(mergedMessage)) *multilineMerger {
	return &multilineMerger{
		cfg:    cfg,
		output: output,
	}
}

// AddLine processes a single reassembled log line.
func (m *multilineMerger) AddLine(line []byte, source string, timeNano int64) {
	// If multiline is disabled, pass through directly
	if m.cfg.MultilineRegex == nil {
		m.output(mergedMessage{
			Line:     append([]byte(nil), line...),
			Source:   source,
			TimeNano: timeNano,
		})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	isContinuation := m.cfg.MultilineRegex.Match(line)

	if !isContinuation {
		// This is a new message -- flush any buffered content first
		m.flushLocked()
		// Start new buffer
		m.buf.Write(line)
		m.lineCount = 1
		m.source = source
		m.timeNano = timeNano
		m.hasData = true
		m.resetTimerLocked()
		return
	}

	// Continuation line
	if !m.hasData {
		// No previous message to attach to -- treat as new message
		m.buf.Write(line)
		m.lineCount = 1
		m.source = source
		m.timeNano = timeNano
		m.hasData = true
		m.resetTimerLocked()
		return
	}

	// Check limits before appending
	if m.lineCount >= m.cfg.MultilineMaxLines ||
		m.buf.Len()+len(m.cfg.MultilineSep)+len(line) > m.cfg.MultilineMaxBytes {
		m.flushLocked()
		// Start fresh with this line
		m.buf.Write(line)
		m.lineCount = 1
		m.source = source
		m.timeNano = timeNano
		m.hasData = true
		m.resetTimerLocked()
		return
	}

	// Append continuation
	m.buf.WriteString(m.cfg.MultilineSep)
	m.buf.Write(line)
	m.lineCount++
	m.resetTimerLocked()
}

// Flush forces any buffered content to be emitted.
func (m *multilineMerger) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushLocked()
}

func (m *multilineMerger) flushLocked() {
	if !m.hasData {
		return
	}
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}

	msg := mergedMessage{
		Line:     make([]byte, m.buf.Len()),
		Source:   m.source,
		TimeNano: m.timeNano,
	}
	copy(msg.Line, m.buf.Bytes())

	m.buf.Reset()
	m.lineCount = 0
	m.hasData = false

	m.output(msg)
}

func (m *multilineMerger) resetTimerLocked() {
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(m.cfg.MultilineTimeout, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.flushLocked()
	})
}
