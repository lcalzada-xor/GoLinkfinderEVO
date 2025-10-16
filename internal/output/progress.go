package output

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ToolStatus int

const (
	ToolStatusRunning ToolStatus = iota
	ToolStatusSuccess
	ToolStatusFailed
	ToolStatusCancelled
)

type progressLog struct {
	mu    sync.Mutex
	out   io.Writer
	label string

	entries []*toolEntry
	index   map[string]*toolEntry

	total     int
	completed int
	failed    int

	frames []rune
	ticker *time.Ticker
	stopCh chan struct{}
	doneCh chan struct{}

	startedAt time.Time
	prevLines int
	nextID    int
}

type toolEntry struct {
	id       string
	label    string
	status   ToolStatus
	message  string
	framePos int
	finished bool
}

// ProgressLog manages interactive terminal output for concurrent tool execution.
// It renders an individual spinner per running tool while keeping the global
// progress bar visible at the bottom.
type ProgressLog struct {
	log *progressLog
}

// ToolHandle represents an in-flight tool displayed in the progress log.
type ToolHandle struct {
	log *progressLog
	id  string
}

// NewProgressLog initialises a ProgressLog that writes to the provided writer.
// The label is used when rendering the global progress bar.
func NewProgressLog(writer io.Writer, label string) *ProgressLog {
	if writer == nil {
		writer = io.Discard
	}

	label = strings.TrimSpace(label)
	if label == "" {
		label = "tasks"
	} else {
		label = strings.ToLower(filepath.Base(label))
	}

	pl := &progressLog{
		out:       writer,
		label:     label,
		frames:    []rune{'|', '/', '-', '\\'},
		index:     make(map[string]*toolEntry),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		startedAt: time.Now(),
	}
	pl.ticker = time.NewTicker(120 * time.Millisecond)

	go pl.loop()

	return &ProgressLog{log: pl}
}

// Start registers a new running tool with the provided description.
func (p *ProgressLog) Start(description string) *ToolHandle {
	if p == nil || p.log == nil {
		return &ToolHandle{}
	}
	return p.log.start(description)
}

// Stop finalises the progress rendering and restores the terminal state.
func (p *ProgressLog) Stop() {
	if p == nil || p.log == nil {
		return
	}
	p.log.stop()
}

// Success marks the tool as completed successfully.
func (h *ToolHandle) Success(message string) {
	if h == nil || h.log == nil {
		return
	}
	h.log.finish(h.id, ToolStatusSuccess, message)
}

// Fail marks the tool as completed with an error message.
func (h *ToolHandle) Fail(message string) {
	if h == nil || h.log == nil {
		return
	}
	h.log.finish(h.id, ToolStatusFailed, message)
}

// Cancel marks the tool as cancelled.
func (h *ToolHandle) Cancel(message string) {
	if h == nil || h.log == nil {
		return
	}
	h.log.finish(h.id, ToolStatusCancelled, message)
}

func (h *ToolHandle) isZero() bool {
	return h == nil || h.log == nil || h.id == ""
}

func (l *progressLog) start(description string) *ToolHandle {
	desc := strings.TrimSpace(description)
	if desc == "" {
		desc = "processing"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.total++
	entry := &toolEntry{
		id:     fmt.Sprintf("T%d", l.nextID),
		label:  desc,
		status: ToolStatusRunning,
	}
	l.nextID++
	l.entries = append(l.entries, entry)
	l.index[entry.id] = entry

	l.renderLocked()

	return &ToolHandle{log: l, id: entry.id}
}

func (l *progressLog) finish(id string, status ToolStatus, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.index[id]
	if !ok || entry.finished {
		return
	}

	entry.status = status
	entry.message = strings.TrimSpace(message)
	entry.finished = true
	l.completed++
	if status == ToolStatusFailed {
		l.failed++
	}

	l.renderLocked()
}

func (l *progressLog) stop() {
	l.mu.Lock()
	if l.stopCh == nil {
		l.mu.Unlock()
		return
	}
	close(l.stopCh)
	l.stopCh = nil
	l.mu.Unlock()

	<-l.doneCh
}

func (l *progressLog) loop() {
	for {
		select {
		case <-l.ticker.C:
			l.mu.Lock()
			for _, entry := range l.entries {
				if entry.status == ToolStatusRunning {
					entry.framePos = (entry.framePos + 1) % len(l.frames)
				}
			}
			l.renderLocked()
			l.mu.Unlock()
		case <-l.stopCh:
			l.ticker.Stop()
			l.mu.Lock()
			l.renderLocked()
			if l.prevLines > 0 {
				fmt.Fprint(l.out, "\n")
				l.prevLines = 0
			}
			l.mu.Unlock()
			close(l.doneCh)
			return
		}
	}
}

func (l *progressLog) renderLocked() {
	if len(l.entries) == 0 {
		return
	}

	var buf bytes.Buffer

	if l.prevLines > 0 {
		fmt.Fprintf(&buf, "\033[%dF", l.prevLines)
	}
	buf.WriteString("\r\033[J")

	for i, entry := range l.entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "\r\033[K%s", l.renderEntry(entry))
	}

	buf.WriteByte('\n')
	fmt.Fprintf(&buf, "\r\033[K%s", l.renderProgress())

	if _, err := l.out.Write(buf.Bytes()); err != nil {
		return
	}

	l.prevLines = len(l.entries) + 1
}

func (l *progressLog) renderEntry(entry *toolEntry) string {
	var prefix string
	switch entry.status {
	case ToolStatusRunning:
		prefix = fmt.Sprintf("[%c]", l.frames[entry.framePos])
	case ToolStatusSuccess:
		prefix = "[✔]"
	case ToolStatusFailed:
		prefix = "[✖]"
	case ToolStatusCancelled:
		prefix = "[⏹]"
	default:
		prefix = "[ ]"
	}

	if entry.message != "" {
		return fmt.Sprintf("%s %s - %s", prefix, entry.label, entry.message)
	}
	return fmt.Sprintf("%s %s", prefix, entry.label)
}

func (l *progressLog) renderProgress() string {
	barWidth := 30
	ratio := 0.0
	if l.total > 0 {
		ratio = float64(l.completed) / float64(l.total)
		if ratio > 1 {
			ratio = 1
		}
	}
	filled := int(math.Round(ratio * float64(barWidth)))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	status := "ok"
	switch {
	case l.completed < l.total:
		status = "running"
	case l.failed > 0:
		status = "fail"
	}

	eta := l.estimateETA()

	return fmt.Sprintf("[%s] %d/%d %s (%s ETA %s)", bar, l.completed, l.total, l.label, status, eta)
}

func (l *progressLog) estimateETA() string {
	remaining := l.total - l.completed
	if remaining <= 0 {
		return "0s"
	}
	if l.completed == 0 {
		return "?"
	}

	elapsed := time.Since(l.startedAt)
	avg := elapsed / time.Duration(l.completed)
	eta := avg * time.Duration(remaining)
	if eta < 0 {
		eta = 0
	}

	if eta > time.Hour {
		return fmt.Sprintf("%dh%dm", int(eta.Hours()), int(eta.Minutes())%60)
	}
	if eta > time.Minute {
		return fmt.Sprintf("%dm%ds", int(eta.Minutes()), int(eta.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(eta.Seconds()))
}
