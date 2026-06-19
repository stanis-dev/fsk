// Package telemetry records MCP tool-call events without writing to stdout.
package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	Record(Event) error
}

// FileRecorder appends one JSON object per line to a file.
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

func (r *FileRecorder) Record(e Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.enc.Encode(e); err != nil {
		log.Printf("telemetry: write failed: %v", err)
		return err
	}
	return nil
}

func (r *FileRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

// Middleware records one Event per tools/call. Other methods pass through
// untouched. The handlers themselves are never modified.
func Middleware(rec Recorder) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			ev := Event{
				TS:        time.Now().UTC().Format(time.RFC3339),
				LatencyMS: time.Since(start).Milliseconds(),
			}
			ev.SessionID = sessionID(req)
			if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
				ev.Tool = p.Name
				if len(p.Arguments) > 0 {
					ev.Args = append(json.RawMessage(nil), p.Arguments...)
				}
			}
			switch {
			case err != nil:
				ev.IsError = true
				ev.Error = err.Error()
			case res != nil:
				if ctr, ok := res.(*mcp.CallToolResult); ok {
					ev.IsError = ctr.IsError
					if ctr.IsError {
						ev.Error = contentText(ctr.Content)
					}
					ev.ResultCount = docsResultCount(ctr)
				}
			}
			if err := rec.Record(ev); err != nil {
				return nil, err
			}
			return res, err
		}
	}
}

func sessionID(req mcp.Request) string {
	switch sess := req.GetSession().(type) {
	case nil:
		return ""
	case *mcp.ServerSession:
		if sess == nil {
			return ""
		}
		return sess.ID()
	case *mcp.ClientSession:
		if sess == nil {
			return ""
		}
		return sess.ID()
	default:
		return sess.ID()
	}
}

// docsResultCount derives a count from a tool result without importing the server's
// typed output: a list-returning tool exposes a top-level "results" array; a
// single-document tool returns one object.
func docsResultCount(ctr *mcp.CallToolResult) int {
	if ctr.IsError || ctr.StructuredContent == nil {
		return 0
	}
	b, err := json.Marshal(ctr.StructuredContent)
	if err != nil {
		return 0
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(b, &obj) != nil {
		return 0
	}
	if raw, ok := obj["results"]; ok {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			return len(arr)
		}
	}
	if len(obj) > 0 {
		return 1
	}
	return 0
}

func contentText(cs []mcp.Content) string {
	var b strings.Builder
	for _, c := range cs {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
