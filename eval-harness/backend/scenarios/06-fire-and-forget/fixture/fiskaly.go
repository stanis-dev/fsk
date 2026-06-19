package pos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// fiskalyClient is an unfinished SIGN IT receipt client. It authenticates and
// runs the two-call records flow, but it is not yet wired into CompleteOrder.
type fiskalyClient struct {
	baseURL string
	apiKey  string
	secret  string
	jwt     string
	system  string // commissioned FISCAL_DEVICE system id
	hc      *http.Client
}

const fiskalyAPIVersion = "2026-02-03"

func newFiskalyClient(apiKey, secret, systemID string) *fiskalyClient {
	return &fiskalyClient{
		baseURL: "https://test.api.fiskaly.com",
		apiKey:  apiKey,
		secret:  secret,
		system:  systemID,
		hc:      &http.Client{},
	}
}

func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("idempotency key: %w", err))
	}
	return hex.EncodeToString(b[:]) // lowercase hex
}

// post sends a JSON POST with the headers fiskaly requires on every write.
func (c *fiskalyClient) post(ctx context.Context, path string, body any) (map[string]any, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Version", fiskalyAPIVersion)
	req.Header.Set("X-Idempotency-Key", newIdempotencyKey())
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return out, fmt.Errorf("fiskaly %s: status %d", path, resp.StatusCode)
	}
	return out, nil
}

// authenticate exchanges the API credentials for a 24h JWT.
func (c *fiskalyClient) authenticate(ctx context.Context) error {
	out, err := c.post(ctx, "/tokens", map[string]any{"key": c.apiKey, "secret": c.secret})
	if err != nil {
		return err
	}
	if content, ok := out["content"].(map[string]any); ok {
		if jwt, ok := content["authentication"].(string); ok {
			c.jwt = jwt
		}
	}
	return nil
}

// issueReceipt issues a sale receipt as the two-call records flow.
func (c *fiskalyClient) issueReceipt(ctx context.Context, o *Order) error {
	if err := c.authenticate(ctx); err != nil {
		return err
	}
	intention, err := c.post(ctx, "/records", map[string]any{
		"type":      "INTENTION",
		"system":    map[string]any{"id": c.system},
		"operation": map[string]any{"type": "TRANSACTION"},
	})
	if err != nil {
		return err
	}
	intentionID, ok := intention["id"].(string)
	if !ok || intentionID == "" {
		return fmt.Errorf("fiskaly intention response missing id")
	}
	_, err = c.post(ctx, "/records", map[string]any{
		"type":   "TRANSACTION",
		"record": map[string]any{"id": intentionID},
		"operation": map[string]any{
			"type":     "RECEIPT",
			"document": map[string]any{"number": o.ID},
		},
	})
	if err != nil {
		return err
	}
	return nil
}
