package main

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
)

func connectTestServer(t *testing.T, configure func(*mcp.Server)) (*mcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()
	c, err := corpus.Load()
	if err != nil {
		t.Fatalf("corpus.Load: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "test"}, nil)
	registerTools(server, c)
	if configure != nil {
		configure(server)
	}
	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session, ctx
}

func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestServerListsBothTools(t *testing.T) {
	session, ctx := connectTestServer(t, nil)
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	if !got["search_fiskaly_docs"] || !got["fetch_fiskaly_doc"] {
		t.Fatalf("missing tools, got: %v", got)
	}
}

func TestServerSearchAndFetch(t *testing.T) {
	session, ctx := connectTestServer(t, nil)

	sr, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_fiskaly_docs", Arguments: map[string]any{"query": "idempotency key"},
	})
	if err != nil {
		t.Fatalf("search CallTool: %v", err)
	}
	if sr.IsError {
		t.Fatalf("search returned IsError: %s", resultText(sr))
	}
	if !strings.Contains(resultText(sr), "probe:auth-and-headers") {
		t.Fatalf("expected auth-and-headers in results, got: %s", resultText(sr))
	}

	fr, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "fetch_fiskaly_doc", Arguments: map[string]any{"id": "probe:records-flow"},
	})
	if err != nil {
		t.Fatalf("fetch CallTool: %v", err)
	}
	if fr.IsError || !strings.Contains(resultText(fr), "INTENTION") {
		t.Fatalf("unexpected fetch result: isErr=%v body=%s", fr.IsError, resultText(fr))
	}
}

func TestServerFetchUnknownIsError(t *testing.T) {
	session, ctx := connectTestServer(t, nil)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "fetch_fiskaly_doc", Arguments: map[string]any{"id": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for unknown id")
	}
}
