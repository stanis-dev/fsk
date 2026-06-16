package main

import (
	"testing"

	"fiskaly-mcp/corpus"
)

func stub() *corpus.Corpus {
	return corpus.New([]corpus.Section{{
		ID: "probe:records-flow", Title: "the records flow", URL: "fsk://probe/notes#records-flow",
		Source: "probe", Path: "POST /records", Version: "2026-02-03",
		Text: "a receipt is two POST /records calls, INTENTION then TRANSACTION RECEIPT",
	}})
}

func TestHandleSearchReturnsHit(t *testing.T) {
	out, err := handleSearch(stub(), searchInput{Query: "records"})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].ID != "probe:records-flow" {
		t.Fatalf("unexpected results: %+v", out.Results)
	}
	if out.Results[0].URL == "" || out.Results[0].Snippet == "" {
		t.Fatal("expected url and snippet to be populated")
	}
}

func TestHandleSearchEmptyQueryErrors(t *testing.T) {
	if _, err := handleSearch(stub(), searchInput{Query: "  "}); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestHandleFetchKnownAndUnknown(t *testing.T) {
	out, err := handleFetch(stub(), fetchInput{ID: "probe:records-flow"})
	if err != nil {
		t.Fatalf("handleFetch error: %v", err)
	}
	if out.Text == "" || out.Metadata.Source != "probe" || out.Metadata.Path != "POST /records" {
		t.Fatalf("unexpected fetch output: %+v", out)
	}
	if _, err := handleFetch(stub(), fetchInput{ID: "nope"}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}
