package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	evSearch = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fiskaly__search_fiskaly_docs"}]}}`
	evWrite  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`
)

func TestCheckGrounded_SearchBeforeWrite(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evSearch, evWrite))
	if !ok {
		t.Fatalf("expected grounded, got %q", verdict)
	}
}

func TestCheckGrounded_WriteBeforeSearch(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite, evSearch))
	if ok {
		t.Fatalf("expected NOT grounded, got %q", verdict)
	}
}

func TestCheckGrounded_NeverSearched(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite))
	if ok || verdict == "" {
		t.Fatalf("expected NOT grounded with a reason, got ok=%v %q", ok, verdict)
	}
}

func TestCheckGrounded_SearchedNeverWrote(t *testing.T) {
	ok, _ := checkGrounded(writeTranscript(t, evSearch))
	if ok {
		t.Fatal("expected NOT grounded (inconclusive) when nothing was written")
	}
}
