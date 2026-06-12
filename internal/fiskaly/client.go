package fiskaly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	TestBaseURL = "https://test.api.fiskaly.com"
	LiveBaseURL = "https://live.api.fiskaly.com"

	APIVersion = "2026-02-03"
)

// APIError is the unified error envelope: every non-2xx response carries
// {content: {status, code, error, message}} plus an X-Trace-Identifier header.
type APIError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Name    string `json:"error"`
	Message string `json:"message"`
	TraceID string `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("fiskaly %s (%d): %s [trace %s]", e.Code, e.Status, e.Message, e.TraceID)
}

type envelope[T any] struct {
	Content  T              `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Client is a minimal typed client for the fiskaly Unified API (SIGN IT).
// It owns token lifecycle (mint on demand, refresh before expiry) and
// stamps the protocol headers every write needs: X-Api-Version and a fresh
// X-Idempotency-Key per attempt.
type Client struct {
	BaseURL string
	HTTP    *http.Client

	key    string
	secret string

	mu       sync.Mutex
	bearer   string
	expires  time.Time
	identity Token

	// OnCall, when set, observes every API call after it completes; the
	// judge consumes this trail. It must not mutate its arguments.
	OnCall func(CallRecord)
}

// CallRecord is the audit trail entry for one API call.
type CallRecord struct {
	At             time.Time       `json:"at"`
	Host           string          `json:"host"`
	Method         string          `json:"method"`
	Path           string          `json:"path"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Status         int             `json:"status"`
	TraceID        string          `json:"trace_id,omitempty"`
	RequestBody    json.RawMessage `json:"request_body,omitempty"`
	ResponseBody   json.RawMessage `json:"response_body,omitempty"`
}

func NewClient(baseURL, key, secret string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		key:     key,
		secret:  secret,
	}
}

// Identity returns the organization and subject the current token is bound
// to, minting a token first if needed.
func (c *Client) Identity(ctx context.Context) (Token, error) {
	if err := c.ensureToken(ctx); err != nil {
		return Token{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.identity, nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bearer != "" && time.Until(c.expires) > 5*time.Minute {
		return nil
	}
	body := map[string]any{
		"content": map[string]any{"type": "API_KEY", "key": c.key, "secret": c.secret},
	}
	var out envelope[Token]
	if err := c.do(ctx, http.MethodPost, "/tokens", "", body, &out, false); err != nil {
		return fmt.Errorf("minting token: %w", err)
	}
	c.bearer = out.Content.Authentication.Bearer
	c.expires = out.Content.Authentication.ExpiresAt
	c.identity = out.Content
	c.identity.Authentication.Bearer = "" // never retained outside the client
	return nil
}

// do executes one API call. Callers hold no locks except ensureToken, which
// passes authed=false to avoid recursing into itself.
func (c *Client) do(ctx context.Context, method, path, scope string, in, out any, authed bool) error {
	if authed {
		// ensureToken locks c.mu; call it before re-locking for bearer read.
		if err := c.ensureToken(ctx); err != nil {
			return err
		}
	}

	var reqBody []byte
	var bodyReader io.Reader
	if in != nil {
		var err error
		reqBody, err = json.Marshal(in)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Version", APIVersion)
	idem := ""
	if method == http.MethodPost || method == http.MethodPatch {
		idem = uuid.NewString()
		req.Header.Set("X-Idempotency-Key", idem)
	}
	if authed {
		c.mu.Lock()
		req.Header.Set("Authorization", "Bearer "+c.bearer)
		c.mu.Unlock()
	}
	if scope != "" {
		req.Header.Set("X-Scope-Identifier", scope)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if c.OnCall != nil {
		c.OnCall(CallRecord{
			At:             time.Now(),
			Host:           c.BaseURL,
			Method:         method,
			Path:           path,
			IdempotencyKey: idem,
			Status:         resp.StatusCode,
			TraceID:        resp.Header.Get("X-Trace-Identifier"),
			RequestBody:    redactSecrets(reqBody),
			ResponseBody:   redactSecrets(respBody),
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errEnv envelope[APIError]
		if json.Unmarshal(respBody, &errEnv) == nil && errEnv.Content.Code != "" {
			errEnv.Content.TraceID = resp.Header.Get("X-Trace-Identifier")
			return &errEnv.Content
		}
		return fmt.Errorf("fiskaly HTTP %d: %s", resp.StatusCode, respBody)
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// --- typed operations ---------------------------------------------------------

func (c *Client) CreateOrganization(ctx context.Context, in OrganizationCreate) (Organization, error) {
	var out envelope[Organization]
	err := c.do(ctx, http.MethodPost, "/organizations", "", envelope[OrganizationCreate]{Content: in}, &out, true)
	return out.Content, err
}

// CreateSubject creates an API key subject. A non-empty scopeOrgID binds the
// subject to that organization (the only way to operate inside a UNIT org —
// the scope header does not redirect resource creation calls themselves).
func (c *Client) CreateSubject(ctx context.Context, in SubjectCreate, scopeOrgID string) (Subject, error) {
	var out envelope[Subject]
	err := c.do(ctx, http.MethodPost, "/subjects", scopeOrgID, envelope[SubjectCreate]{Content: in}, &out, true)
	return out.Content, err
}

func (c *Client) CreateTaxpayer(ctx context.Context, in TaxpayerCreate) (Taxpayer, error) {
	var out envelope[Taxpayer]
	err := c.do(ctx, http.MethodPost, "/taxpayers", "", envelope[TaxpayerCreate]{Content: in}, &out, true)
	return out.Content, err
}

func (c *Client) CreateLocation(ctx context.Context, in LocationCreate) (Location, error) {
	var out envelope[Location]
	err := c.do(ctx, http.MethodPost, "/locations", "", envelope[LocationCreate]{Content: in}, &out, true)
	return out.Content, err
}

func (c *Client) CreateSystem(ctx context.Context, in SystemCreate) (System, error) {
	var out envelope[System]
	err := c.do(ctx, http.MethodPost, "/systems", "", envelope[SystemCreate]{Content: in}, &out, true)
	return out.Content, err
}

// SetState drives the ACQUIRED -> COMMISSIONED -> DECOMMISSIONED lifecycle.
// resource is the collection path segment: "taxpayers", "locations" or "systems".
func (c *Client) SetState(ctx context.Context, resource, id, state string) error {
	path := fmt.Sprintf("/%s/%s", resource, id)
	return c.do(ctx, http.MethodPatch, path, "", envelope[StateUpdate]{Content: StateUpdate{State: state}}, nil, true)
}

func (c *Client) CreateRecord(ctx context.Context, in RecordCreate) (Record, error) {
	var out envelope[Record]
	err := c.do(ctx, http.MethodPost, "/records", "", envelope[RecordCreate]{Content: in}, &out, true)
	return out.Content, err
}

func (c *Client) GetRecord(ctx context.Context, id string) (Record, error) {
	var out envelope[Record]
	err := c.do(ctx, http.MethodGet, "/records/"+id, "", nil, &out, true)
	return out.Content, err
}

// WaitRecord polls until the record reaches a final state+mode. The TEST
// environment usually completes synchronously; LIVE transmits to AdE and
// needs the poll loop.
func (c *Client) WaitRecord(ctx context.Context, id string, timeout time.Duration) (Record, error) {
	deadline := time.Now().Add(timeout)
	for {
		rec, err := c.GetRecord(ctx, id)
		if err != nil {
			return rec, err
		}
		if rec.Terminal() {
			return rec, nil
		}
		if time.Now().After(deadline) {
			return rec, fmt.Errorf("record %s not terminal after %s (state=%s mode=%s)", id, timeout, rec.State, rec.Mode)
		}
		select {
		case <-ctx.Done():
			return rec, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
