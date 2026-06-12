// Package audit collects what an agent actually did — every API call and
// every MCP tool invocation — and judges it against compliance rules.
package audit

import (
	"encoding/json"
	"sync"
	"time"

	"z2r/internal/fiskaly"
)

type ToolCall struct {
	At   time.Time       `json:"at"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Trail is the append-only session record the judge audits. One trail per
// sandbox keeps each merchant's activity independently auditable.
type Trail struct {
	mu    sync.Mutex
	calls []fiskaly.CallRecord
	tools []ToolCall
}

func (t *Trail) AddCall(c fiskaly.CallRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls = append(t.calls, c)
}

func (t *Trail) AddTool(name string, args any) {
	raw, _ := json.Marshal(args)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tools = append(t.tools, ToolCall{At: time.Now(), Tool: name, Args: raw})
}

func (t *Trail) Snapshot() ([]fiskaly.CallRecord, []ToolCall) {
	t.mu.Lock()
	defer t.mu.Unlock()
	calls := make([]fiskaly.CallRecord, len(t.calls))
	copy(calls, t.calls)
	tools := make([]ToolCall, len(t.tools))
	copy(tools, t.tools)
	return calls, tools
}
