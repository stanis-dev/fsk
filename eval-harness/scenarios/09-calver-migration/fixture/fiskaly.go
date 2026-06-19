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

// fiskalyClient is a half-finished SIGN IT client written against an older
// version of the API. It is not yet wired into CompleteOrder.
type fiskalyClient struct {
	baseURL string
	apiKey  string
	secret  string
	jwt     string
	hc      *http.Client
}

// fiskalyAPIVersion pins the API version this client was written against.
const fiskalyAPIVersion = "2025-08-12"

func newFiskalyClient(apiKey, secret string) *fiskalyClient {
	return &fiskalyClient{
		baseURL: "https://test.api.fiskaly.com",
		apiKey:  apiKey,
		secret:  secret,
		hc:      &http.Client{},
	}
}

func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("idempotency key: %w", err))
	}
	return hex.EncodeToString(b[:])
}

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

// provision creates the merchant. Written against the old resource model:
// a legal "entity" and its fiscal-device "asset".
func (c *fiskalyClient) provision(ctx context.Context) error {
	if _, err := c.post(ctx, "/entities", map[string]any{
		"type": "COMPANY",
		"name": map[string]any{"legal": "Acme Srl", "trade": "Acme"},
	}); err != nil {
		return err
	}
	if _, err := c.post(ctx, "/assets", map[string]any{
		"type":     "FISCAL_DEVICE",
		"software": map[string]any{"name": "pos", "version": "1.0"},
	}); err != nil {
		return err
	}
	return nil
}

// issueReceipt issues the sale receipt through the records flow.
func (c *fiskalyClient) issueReceipt(ctx context.Context, o *Order) error {
	if err := c.authenticate(ctx); err != nil {
		return err
	}
	intention, err := c.post(ctx, "/records", map[string]any{
		"type":      "INTENTION",
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
	return err
}
