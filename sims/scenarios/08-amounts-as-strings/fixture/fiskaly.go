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

// fiskalyClient is an UNFINISHED SIGN IT receipt client a teammate started. It
// authenticates and posts the records flow, but it is not yet wired into
// CompleteOrder. See issueReceipt for where it was left off.
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
	_, _ = rand.Read(b[:])
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
	data, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
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

// issueReceipt issues a sale receipt through the records flow.
//
// TODO(teammate): left off here. Sending the order total as the euro amount,
// e.g. 4.33. Haven't done the per-line VAT split yet.
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
	intentionID, _ := intention["id"].(string)
	total := float64(o.Gross()) / 100.0 // euros as a float
	_, err = c.post(ctx, "/records", map[string]any{
		"type":   "TRANSACTION",
		"record": map[string]any{"id": intentionID},
		"operation": map[string]any{
			"type": "RECEIPT",
			"document": map[string]any{
				"number": o.ID,
				"total":  total,
			},
		},
	})
	return err
}
