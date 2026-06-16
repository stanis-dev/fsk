package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
)

type searchInput struct {
	Query string `json:"query" jsonschema:"keyword or natural-language query against the fiskaly SIGN IT documentation"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results to return; defaults to 8"`
}

type searchResult struct {
	ID      string `json:"id" jsonschema:"document id; pass to fetch_fiskaly_doc to read the full section"`
	Title   string `json:"title" jsonschema:"human-readable title"`
	URL     string `json:"url" jsonschema:"canonical citation URL"`
	Snippet string `json:"snippet" jsonschema:"best-matching passage from the document"`
}

type searchOutput struct {
	Results []searchResult `json:"results" jsonschema:"ranked search hits, best first"`
}

type fetchInput struct {
	ID string `json:"id" jsonschema:"document id returned by search_fiskaly_docs"`
}

type fetchMetadata struct {
	Source  string `json:"source" jsonschema:"corpus source: spec | probe | brief | kb"`
	Path    string `json:"path" jsonschema:"API path/operation this document covers, if any"`
	Version string `json:"version" jsonschema:"SIGN IT API version this document describes"`
}

type fetchOutput struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Text     string        `json:"text" jsonschema:"the full document text"`
	URL      string        `json:"url"`
	Metadata fetchMetadata `json:"metadata"`
}

func handleSearch(c *corpus.Corpus, in searchInput) (searchOutput, error) {
	if strings.TrimSpace(in.Query) == "" {
		return searchOutput{}, fmt.Errorf("query must be non-empty")
	}
	hits := c.Search(in.Query, in.Limit)
	out := searchOutput{Results: make([]searchResult, 0, len(hits))}
	for _, h := range hits {
		out.Results = append(out.Results, searchResult{
			ID: h.Section.ID, Title: h.Section.Title, URL: h.Section.URL, Snippet: h.Snippet,
		})
	}
	return out, nil
}

func handleFetch(c *corpus.Corpus, in fetchInput) (fetchOutput, error) {
	sec, ok := c.Lookup(in.ID)
	if !ok {
		return fetchOutput{}, fmt.Errorf("no document with id %q", in.ID)
	}
	return fetchOutput{
		ID: sec.ID, Title: sec.Title, Text: sec.Text, URL: sec.URL,
		Metadata: fetchMetadata{Source: sec.Source, Path: sec.Path, Version: sec.Version},
	}, nil
}

// registerTools wires the two read-only docs tools onto the server. AddTool
// derives input/output JSON Schema from the structs, sets StructuredContent from
// the returned output, and adds the JSON as a TextContent block automatically.
func registerTools(s *mcp.Server, c *corpus.Corpus) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_fiskaly_docs",
		Description: "Search the curated fiskaly SIGN IT documentation by keyword. Returns ranked {id, title, url, snippet}; call fetch_fiskaly_doc with an id to read the full section.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, searchOutput, error) {
		out, err := handleSearch(c, in)
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "fetch_fiskaly_doc",
		Description: "Fetch the full text of a fiskaly SIGN IT documentation section by id (ids come from search_fiskaly_docs).",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in fetchInput) (*mcp.CallToolResult, fetchOutput, error) {
		out, err := handleFetch(c, in)
		return nil, out, err
	})
}

func ptr[T any](v T) *T { return &v }
