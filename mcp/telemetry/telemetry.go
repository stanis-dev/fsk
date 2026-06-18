// Package telemetry records one structured event per MCP tools/call to a file
// sink, so the eval harness can see how an agent used the docs tools. It never
// writes to stdout, which is the MCP stdio protocol channel.
package telemetry

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Event is one tools/call observation. Field names are the on-disk JSONL schema.
type Event struct {
	TS          string          `json:"ts"`
	SessionID   string          `json:"session_id,omitempty"`
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args,omitempty"`
	ResultCount int             `json:"result_count"`
	IsError     bool            `json:"is_error"`
	Error       string          `json:"error,omitempty"`
	LatencyMS   int64           `json:"latency_ms"`
}

// Recorder persists telemetry events. Implementations must be safe for
// concurrent use.
type Recorder interface {
	Record(Event)
}

// FileRecorder appends one JSON object per line to a file. Writes are
// best-effort: a failure is logged to stderr and never propagates.
type FileRecorder struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

func NewFileRecorder(path string) (*FileRecorder, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileRecorder{f: f, enc: json.NewEncoder(f)}, nil
}

func (r *FileRecorder) Record(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.enc.Encode(e); err != nil {
		log.Printf("telemetry: write failed: %v", err)
	}
}

func (r *FileRecorder) Close() error { return r.f.Close() }

type nopRecorder struct{}

func (nopRecorder) Record(Event) {}

// Nop returns a Recorder that discards everything.
func Nop() Recorder { return nopRecorder{} }
