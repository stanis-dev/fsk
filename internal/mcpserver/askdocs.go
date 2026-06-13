package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// askDocsClient proxies fiskaly's own "Ask AI" RAG assistant on
// workspace.fiskaly.com — a retrieval layer over their docs, OpenAPI spec,
// Zendesk KB, web pages and PDFs. It is the agent's read/grounding surface:
// advisory and cited, not authoritative. It complements (does not replace)
// the deterministic judge, and it is an EXTERNAL dependency, so every call
// degrades gracefully — a docs outage must never break the action tools.
type askDocsClient struct {
	baseURL string
	http    *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

func newAskDocsClient() *askDocsClient {
	return &askDocsClient{
		baseURL: "https://workspace.fiskaly.com",
		http:    &http.Client{Timeout: 75 * time.Second},
	}
}

func (a *askDocsClient) ensureToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Until(a.expires) > 60*time.Second {
		return a.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/session", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.baseURL)
	req.Header.Set("User-Agent", "zero-to-receipt-mcp (interview prototype; low volume)")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("session HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("unexpected session response")
	}
	a.token = out.Token
	a.expires = time.Unix(out.ExpiresAt, 0)
	return a.token, nil
}

// Citation mirrors the {title, url, product} entries the RAG returns.
type Citation struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Product string `json:"product,omitempty"`
}

type AskDocsResult struct {
	Answer    string     `json:"answer"`
	Grounded  bool       `json:"grounded"`
	Citations []Citation `json:"citations"`
	FollowUps []string   `json:"follow_ups,omitempty"`
}

// ask runs one RAG query. The product context (e.g. "SIGN_IT") is required
// for a grounded answer — without it the assistant only asks which product
// you mean.
func (a *askDocsClient) ask(ctx context.Context, question, product, persona string) (AskDocsResult, error) {
	token, err := a.ensureToken(ctx)
	if err != nil {
		return AskDocsResult{}, fmt.Errorf("authenticating to Ask-AI: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{"message": question, "persona": persona, "product": product})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return AskDocsResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.baseURL)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "zero-to-receipt-mcp (interview prototype; low volume)")

	resp, err := a.http.Do(req)
	if err != nil {
		return AskDocsResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusTooManyRequests {
			return AskDocsResult{}, fmt.Errorf("Ask-AI rate limit reached (5/min, 30/hr per session); try again shortly")
		}
		return AskDocsResult{}, fmt.Errorf("Ask-AI HTTP %d: %s", resp.StatusCode, msg)
	}

	// Parse the SSE stream: lines "data: {json}" with type text|done.
	var out AskDocsResult
	var sb strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var ev struct {
			Type      string     `json:"type"`
			Content   string     `json:"content"`
			Grounded  bool       `json:"grounded"`
			Citations []Citation `json:"citations"`
			FollowUps []string   `json:"followUps"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "text":
			sb.WriteString(ev.Content)
		case "done":
			out.Grounded = ev.Grounded
			out.Citations = ev.Citations
			out.FollowUps = ev.FollowUps
		}
	}
	if err := sc.Err(); err != nil {
		return AskDocsResult{}, err
	}
	out.Answer = sb.String()
	if out.Citations == nil {
		out.Citations = []Citation{}
	}
	return out, nil
}
