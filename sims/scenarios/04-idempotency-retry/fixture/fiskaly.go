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

// fiskalyClient is an UNFINISHED SIGN IT client a teammate started. Auth works
// through a retry helper; the records flow is not implemented yet. It is not
// wired into CompleteOrder.
type fiskalyClient struct {
	baseURL string
	apiKey  string
	secret  string
	jwt     string
	idemKey string // generated once; reused for every request (see newFiskalyClient)
	hc      *http.Client
}

const fiskalyAPIVersion = "2026-02-03"

func newIdempotencyKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:]) // lowercase hex
}

func newFiskalyClient(apiKey, secret string) *fiskalyClient {
	return &fiskalyClient{
		baseURL: "https://test.api.fiskaly.com",
		apiKey:  apiKey,
		secret:  secret,
		// TODO(teammate): generate the idempotency key once and reuse it so
		// retries don't double-write.
		idemKey: newIdempotencyKey(),
		hc:      &http.Client{},
	}
}

// postWithRetry sends a JSON POST, retrying transient failures. It reuses
// c.idemKey on every attempt and every call.
func (c *fiskalyClient) postWithRetry(ctx context.Context, path string, body any) (map[string]any, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Version", fiskalyAPIVersion)
		req.Header.Set("X-Idempotency-Key", c.idemKey)
		if c.jwt != "" {
			req.Header.Set("Authorization", "Bearer "+c.jwt)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var out map[string]any
		_ = json.Unmarshal(data, &out)
		if resp.StatusCode/100 == 2 {
			return out, nil
		}
		lastErr = fmt.Errorf("fiskaly %s: status %d", path, resp.StatusCode)
	}
	return nil, lastErr
}

// authenticate exchanges the API credentials for a 24h JWT.
func (c *fiskalyClient) authenticate(ctx context.Context) error {
	out, err := c.postWithRetry(ctx, "/tokens", map[string]any{"key": c.apiKey, "secret": c.secret})
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

// issueReceipt issues the sale receipt.
//
// TODO(teammate): not started. Wire up the records flow here.
func (c *fiskalyClient) issueReceipt(ctx context.Context, o *Order) error {
	return fmt.Errorf("issueReceipt: not implemented")
}
