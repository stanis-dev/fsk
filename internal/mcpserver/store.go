// Package mcpserver exposes the SIGN IT integration as MCP tools any AI
// agent can drive: provision a merchant sandbox, issue and cancel fiscal
// receipts, and have a judge audit everything the agent did.
package mcpserver

import (
	"fmt"
	"sync"
	"sync/atomic"

	"z2r/internal/audit"
	"z2r/internal/fiskaly"
)

// Sandbox is one provisioned merchant stack plus the client scoped to it
// and the audit trail of everything done inside it. The scoped credentials
// stay server-side: agents hold an opaque sandbox_id, never API secrets.
type Sandbox struct {
	ID     string
	Name   string
	Stack  fiskaly.Stack
	Client *fiskaly.Client
	Trail  *audit.Trail

	docSeq atomic.Int64
}

func (s *Sandbox) NextDocumentNumber() string {
	return fmt.Sprintf("%d", s.docSeq.Add(1))
}

type Store struct {
	mu        sync.Mutex
	sandboxes map[string]*Sandbox
	seq       atomic.Int64
}

func NewStore() *Store {
	return &Store{sandboxes: map[string]*Sandbox{}}
}

func (st *Store) Add(name string, stack fiskaly.Stack, client *fiskaly.Client, trail *audit.Trail) *Sandbox {
	sb := &Sandbox{
		ID:     fmt.Sprintf("sbx-%03d", st.seq.Add(1)),
		Name:   name,
		Stack:  stack,
		Client: client,
		Trail:  trail,
	}
	st.mu.Lock()
	st.sandboxes[sb.ID] = sb
	st.mu.Unlock()
	return sb
}

func (st *Store) Get(id string) (*Sandbox, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	sb, ok := st.sandboxes[id]
	if !ok {
		return nil, fmt.Errorf("unknown sandbox %q — call provision_sandbox first, or list your sandboxes with audit_session", id)
	}
	return sb, nil
}

func (st *Store) All() []*Sandbox {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]*Sandbox, 0, len(st.sandboxes))
	for _, sb := range st.sandboxes {
		out = append(out, sb)
	}
	return out
}
