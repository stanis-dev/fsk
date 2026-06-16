# search_fiskaly_docs Implementation Plan (Plan A: tools + ranker + seed corpus)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `search_fiskaly_docs` and `fetch_fiskaly_doc` to the (currently empty) Go MCP server, backed by a hand-curated seed corpus of the probed SIGN IT contract, so a clean-room consumer agent can ground its integration and pass the deterministic judge.

**Architecture:** A `corpus` package loads a `go:embed`-ed `index.json` (sections of curated docs) and serves keyword search (field-weighted BM25-lite) + id lookup. `main` registers two read-only MCP tools over it via `mcp.AddTool`, which derives input/output JSON Schema from Go structs and returns both `structuredContent` and a JSON `TextContent` block automatically. The corpus travels inside the binary, so it survives the hermetic Docker eval where no docs are on the filesystem.

**Tech Stack:** Go 1.23, `github.com/modelcontextprotocol/go-sdk v1.2.0` (already pinned), stdlib only otherwise. No new dependencies in Plan A.

---

## Key implementation decisions (review these before executing)

1. **Seed corpus is hand-curated from `research/api-probes/NOTES.md`.** Plan A ships ~5 sections covering the exact facts the judge checks (host, `/tokens`, `X-Idempotency-Key`, `X-Api-Version`, two-call `/records`). The full spec-operation + KB corpus is Plan B (the generator). This is the smallest-effort path to a measured eval pass.
2. **Tokenize at load, not in `index.json`.** The spec mentioned precomputing tokens in the index; for a corpus this small, tokenizing once at startup is negligible and keeps `index.json` (hand- and generator-authored) simple. This is the one deliberate deviation from the spec.
3. **Tokenizer splits on non-alphanumeric** (so `X-Idempotency-Key` → `x idempotency key`, `/records` → `records`). Matching is unaffected and arguably better; the spec's "preserve path tokens" is not needed.
4. **No YAML / OpenAPI parsing in Plan A.** Deferred to Plan B.

## File structure

- Create `mcp/corpus/corpus.go` — `Section` type, `//go:embed index.json`, `Load()`, `New()`, `Lookup()`.
- Create `mcp/corpus/search.go` — tokenizer, BM25-lite index, `Search()`.
- Create `mcp/corpus/index.json` — the seed corpus (committed artifact).
- Create `mcp/corpus/corpus_test.go` — `Lookup` / `Load` tests.
- Create `mcp/corpus/search_test.go` — ranking tests.
- Create `mcp/tools.go` — input/output structs, `handleSearch`/`handleFetch`, `registerTools`.
- Create `mcp/tools_test.go` — handler tests against a stub corpus.
- Modify `mcp/main.go` — load corpus, register tools.
- Create `evals/assert-grounded.sh` — transcript assertion (searched-before-coding).

---

## Task 1: Author the eval scenario (AGENTS.md: before implementation)

**Files:**
- Create: `evals/assert-grounded.sh`

- [ ] **Step 1: Write the grounding assertion script**

```bash
#!/usr/bin/env bash
# assert-grounded.sh — the consumer agent must ground itself in the docs before
# writing integration code: the transcript must contain a search_fiskaly_docs
# tool call BEFORE the first file mutation (Write/Edit/MultiEdit).
#
# Usage: evals/assert-grounded.sh <transcript.jsonl>
# Exit:  0 grounded · 1 not grounded · 2 usage/error
set -euo pipefail
transcript="${1:?usage: assert-grounded.sh <transcript.jsonl>}"
[ -f "$transcript" ] || { echo "transcript not found: $transcript" >&2; exit 2; }

# stream-json writes one event per line, in order, so line numbers are a faithful
# ordering. We compare the first search call against the first code mutation.
search_line=$(grep -n '"name":"search_fiskaly_docs"' "$transcript" | head -1 | cut -d: -f1 || true)
mutate_line=$(grep -nE '"name":"(Write|Edit|MultiEdit)"' "$transcript" | head -1 | cut -d: -f1 || true)

if [ -z "$search_line" ]; then
  echo "NOT GROUNDED: agent never called search_fiskaly_docs"
  exit 1
fi
if [ -z "$mutate_line" ]; then
  echo "INCONCLUSIVE: agent searched but never wrote integration code"
  exit 1
fi
if [ "$search_line" -lt "$mutate_line" ]; then
  echo "GROUNDED: searched (line $search_line) before first code change (line $mutate_line)"
  exit 0
fi
echo "NOT GROUNDED: first code change (line $mutate_line) precedes first search (line $search_line)"
exit 1
```

- [ ] **Step 2: Make it executable and verify it rejects an empty transcript**

Run:
```bash
chmod +x evals/assert-grounded.sh
printf '' > /tmp/empty.jsonl
evals/assert-grounded.sh /tmp/empty.jsonl; echo "exit=$?"
```
Expected: `NOT GROUNDED: agent never called search_fiskaly_docs` then `exit=1`.

- [ ] **Step 3: Verify it passes a well-ordered transcript**

Run:
```bash
printf '%s\n' \
  '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"search_fiskaly_docs"}]}}' \
  '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}' \
  > /tmp/ok.jsonl
evals/assert-grounded.sh /tmp/ok.jsonl; echo "exit=$?"
```
Expected: `GROUNDED: searched (line 1) before first code change (line 2)` then `exit=0`.

- [ ] **Step 4: Document the acceptance criteria**

Append to the top-of-file comment block is already present. The full eval gate (run in Task 7) is:
`evals/run-eval.sh` produces **build PASS, tests PASS, judge conformant**, AND `evals/assert-grounded.sh` on its transcript exits 0.

- [ ] **Step 5: Commit**

```bash
git add evals/assert-grounded.sh
git commit -m "Eval: assert agent grounds in docs before writing integration"
git push origin main
```

---

## Task 2: Corpus data model, embedded loader, and seed corpus

**Files:**
- Create: `mcp/corpus/index.json`
- Create: `mcp/corpus/corpus.go`
- Test: `mcp/corpus/corpus_test.go`

- [ ] **Step 1: Write the seed corpus `mcp/corpus/index.json`**

```json
[
  {
    "id": "probe:auth-and-headers",
    "title": "SIGN IT — authentication, host, and required headers",
    "url": "fsk://probe/notes#auth-and-headers",
    "source": "probe",
    "path": "POST /tokens",
    "version": "2026-02-03",
    "text": "Host: all calls go to https://test.api.fiskaly.com (TEST) or https://live.api.fiskaly.com (LIVE). Required on every call: header X-Api-Version: 2026-02-03. Required on every POST (including POST /tokens): header X-Idempotency-Key, a lowercase-hex UUID v4 (uppercase from uuidgen is rejected). Request/header validation runs BEFORE auth, so a missing idempotency key returns 400 even with an invalid bearer. Authenticate by POST /tokens with the API key/secret (fields key and secret); the JWT is returned under content.authentication (NOT content.access_token) and is valid 24h. Send it as Authorization: Bearer <jwt> on subsequent calls."
  },
  {
    "id": "probe:scoped-subject",
    "title": "SIGN IT — scoping a subject to a unit organization",
    "url": "fsk://probe/notes#scoped-subject",
    "source": "probe",
    "path": "POST /subjects",
    "version": "2026-02-03",
    "text": "The HUB API key authenticates as a GROUP organization. Create a UNIT with POST /organizations {type: UNIT, name}. You cannot create a taxpayer on a GROUP org (returns 405 E_METHOD_NOT_ALLOWED). Instead, POST /subjects {type: API_KEY, name} WITH header X-Scope-Identifier: <unit-id> to mint credentials scoped to that UNIT; the key and secret are returned ONCE under content.credentials and are non-recoverable. Then POST /tokens with those scoped credentials to get a UNIT-scoped JWT; subsequent calls need no scope header."
  },
  {
    "id": "probe:provisioning",
    "title": "SIGN IT — provisioning taxpayer, location, and system",
    "url": "fsk://probe/notes#provisioning",
    "source": "probe",
    "path": "POST /taxpayers",
    "version": "2026-02-03",
    "text": "With a UNIT-scoped token: POST /taxpayers (type COMPANY) where name requires BOTH legal and trade, plus the Italian fiscalization block {type: IT, tax_id_number(11), vat_id_number(11), credentials: {type: FISCONLINE, pin, password, tax_id_number(16)}} (dummy values accepted in TEST). POST /locations (type BRANCH) with taxpayer.id, name (<=32 chars), address. POST /systems (type FISCAL_DEVICE) requires location, producer {type: MPN, number, details.name}, software {name, version}. Each is created state=ACQUIRED mode=INACTIVE. Commission each via PATCH .../{id} {content: {state: COMMISSIONED}} in order taxpayer -> location -> system; mode flips to OPERATIVE automatically. X-Idempotency-Key is required on PATCH too."
  },
  {
    "id": "probe:records-flow",
    "title": "SIGN IT — issuing a receipt: the two-call records flow",
    "url": "fsk://probe/notes#records-flow",
    "source": "probe",
    "path": "POST /records",
    "version": "2026-02-03",
    "text": "A receipt is TWO POST /records calls, not a single /receipts POST. 1) INTENTION: POST /records {type: INTENTION, system.id, operation: {type: TRANSACTION}} -> state=ACCEPTED mode=PROCESSING. 2) TRANSACTION: POST /records {type: TRANSACTION, record.id: <intention-id>, operation: {type: RECEIPT, document: {number, total_vat: {amount, exclusive, inclusive}}, entries: [SALE/ITEM with full VAT breakdown], payments: [CASH >=1]}}. In TEST this returns state=COMPLETED mode=FINISHED synchronously, with compliance.data a DCW progressive number (zeroed in TEST) and compliance.url the AdE print endpoint. LIVE is async -> poll the record until FINISHED."
  },
  {
    "id": "probe:money-model",
    "title": "SIGN IT — VAT, amounts, and value rules",
    "url": "fsk://probe/notes#money-model",
    "source": "probe",
    "path": "",
    "version": "2026-02-03",
    "text": "Amounts are decimal STRINGS matching ^(-)?\\d{1,12}(\\.\\d{1,8})?$, not numbers. A VatRateCategory requires ALL of percentage, amount, exclusive, and inclusive — the API derives none of them, so the integration must compute every field. All resource IDs are UUIDv7 (time-ordered). Subject credentials are shown once and are non-recoverable. There is no DELETE; resources are retired via DECOMMISSIONED/DISABLED states."
  }
]
```

- [ ] **Step 2: Write the failing test for the loader and lookup**

```go
package corpus

import "testing"

func TestLoadEmbeddedCorpusIsNonEmpty(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := c.Lookup("probe:records-flow"); !ok {
		t.Fatal("expected seed section probe:records-flow to be present")
	}
}

func TestLookupUnknownID(t *testing.T) {
	c := New([]Section{{ID: "a", Title: "A", Text: "x"}})
	if _, ok := c.Lookup("missing"); ok {
		t.Fatal("expected Lookup of unknown id to return ok=false")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails to compile**

Run: `cd mcp && go test ./corpus/`
Expected: FAIL — `undefined: Load`, `undefined: New`, `undefined: Section`.

- [ ] **Step 4: Implement `mcp/corpus/corpus.go`**

```go
// Package corpus serves the curated fiskaly SIGN IT documentation corpus that is
// embedded into the MCP binary, so a clean-room consumer agent can search and
// fetch docs with no filesystem or network access.
package corpus

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed index.json
var indexJSON []byte

// Section is one self-contained unit of documentation (one id = one fetch).
type Section struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Text    string `json:"text"`
}

// Corpus is an in-memory, searchable view of the embedded sections.
type Corpus struct {
	sections []Section
	byID     map[string]Section
	index    *bm25
}

// Load parses the embedded index and builds a searchable Corpus.
func Load() (*Corpus, error) {
	var secs []Section
	if err := json.Unmarshal(indexJSON, &secs); err != nil {
		return nil, fmt.Errorf("corpus: parsing embedded index: %w", err)
	}
	if len(secs) == 0 {
		return nil, fmt.Errorf("corpus: embedded index is empty")
	}
	return New(secs), nil
}

// New builds a Corpus from sections in memory (used by Load and by tests).
func New(secs []Section) *Corpus {
	byID := make(map[string]Section, len(secs))
	for _, s := range secs {
		byID[s.ID] = s
	}
	return &Corpus{sections: secs, byID: byID, index: newBM25(secs)}
}

// Lookup returns the section with the given id.
func (c *Corpus) Lookup(id string) (Section, bool) {
	s, ok := c.byID[id]
	return s, ok
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd mcp && go test ./corpus/`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
git add mcp/corpus/index.json mcp/corpus/corpus.go mcp/corpus/corpus_test.go
git commit -m "MCP corpus: seed index + embedded loader and lookup"
git push origin main
```

---

## Task 3: Field-weighted BM25-lite search

**Files:**
- Create: `mcp/corpus/search.go`
- Test: `mcp/corpus/search_test.go`

- [ ] **Step 1: Write the failing ranking test**

```go
package corpus

import "testing"

func testCorpus() *Corpus {
	return New([]Section{
		{ID: "records", Title: "issuing a receipt: the records flow", Path: "POST /records",
			Text: "a receipt is two POST /records calls, INTENTION then TRANSACTION RECEIPT"},
		{ID: "auth", Title: "authentication and headers", Path: "POST /tokens",
			Text: "POST /tokens returns a JWT; X-Idempotency-Key required on every post"},
		{ID: "money", Title: "VAT and amounts", Path: "",
			Text: "amounts are decimal strings; VatRateCategory requires all fields"},
	})
}

func TestSearchRanksRecordsFirst(t *testing.T) {
	c := testCorpus()
	hits := c.Search("records flow", 8)
	if len(hits) == 0 {
		t.Fatal("expected hits for 'records flow'")
	}
	if hits[0].Section.ID != "records" {
		t.Fatalf("expected 'records' first, got %q", hits[0].Section.ID)
	}
	if hits[0].Snippet == "" {
		t.Fatal("expected a non-empty snippet")
	}
}

func TestSearchEmptyQueryReturnsNil(t *testing.T) {
	if c := testCorpus(); c.Search("   ", 8) != nil {
		t.Fatal("expected nil for whitespace query")
	}
}

func TestSearchNoMatchReturnsEmpty(t *testing.T) {
	if hits := testCorpus().Search("kubernetes helm chart", 8); len(hits) != 0 {
		t.Fatalf("expected no hits, got %d", len(hits))
	}
}

func TestSearchHonorsLimit(t *testing.T) {
	c := testCorpus()
	if hits := c.Search("post", 1); len(hits) > 1 {
		t.Fatalf("expected at most 1 hit, got %d", len(hits))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd mcp && go test ./corpus/ -run TestSearch`
Expected: FAIL — `c.Search undefined`.

- [ ] **Step 3: Implement `mcp/corpus/search.go`**

```go
package corpus

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Hit is one ranked search result.
type Hit struct {
	Section Section
	Score   float64
	Snippet string
}

const (
	boostTitle = 3
	boostPath  = 2
	boostText  = 1
	k1         = 1.5
	b          = 0.75
	snippetLen = 200
)

type docStats struct {
	tf     map[string]int
	length int
}

type bm25 struct {
	docs  []docStats
	df    map[string]int
	n     int
	avgdl float64
}

func newBM25(secs []Section) *bm25 {
	idx := &bm25{df: map[string]int{}, n: len(secs)}
	var total int
	for _, s := range secs {
		tf := map[string]int{}
		addTokens(tf, s.Title, boostTitle)
		addTokens(tf, s.Path, boostPath)
		addTokens(tf, s.Text, boostText)
		length := 0
		for _, c := range tf {
			length += c
		}
		for term := range tf {
			idx.df[term]++
		}
		idx.docs = append(idx.docs, docStats{tf: tf, length: length})
		total += length
	}
	if idx.n > 0 {
		idx.avgdl = float64(total) / float64(idx.n)
	}
	return idx
}

// Search returns up to limit sections ranked by field-weighted BM25-lite.
// A whitespace-only query returns nil; no matches returns an empty slice.
func (c *Corpus) Search(query string, limit int) []Hit {
	if limit <= 0 {
		limit = 8
	}
	qterms := tokenize(query)
	if len(qterms) == 0 {
		return nil
	}
	hits := []Hit{}
	for i, ds := range c.index.docs {
		score := 0.0
		for _, t := range qterms {
			tf := ds.tf[t]
			if tf == 0 {
				continue
			}
			df := float64(c.index.df[t])
			idf := math.Log(1 + (float64(c.index.n)-df+0.5)/(df+0.5))
			denom := float64(tf) + k1*(1-b+b*float64(ds.length)/c.index.avgdl)
			score += idf * (float64(tf) * (k1 + 1)) / denom
		}
		if score > 0 {
			sec := c.sections[i]
			hits = append(hits, Hit{Section: sec, Score: score, Snippet: snippet(sec.Text, qterms)})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func addTokens(tf map[string]int, s string, boost int) {
	for _, t := range tokenize(s) {
		tf[t] += boost
	}
}

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// snippet returns a window of text around the first matching query term.
// It assumes ASCII-ish docs; offsets come from a lowercased copy of equal length.
func snippet(text string, qterms []string) string {
	lower := strings.ToLower(text)
	pos := -1
	for _, t := range qterms {
		if i := strings.Index(lower, t); i >= 0 && (pos < 0 || i < pos) {
			pos = i
		}
	}
	start := 0
	if pos > 0 {
		start = pos
	}
	end := start + snippetLen
	if end > len(text) {
		end = len(text)
	}
	out := strings.TrimSpace(text[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(text) {
		out += "…"
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd mcp && go test ./corpus/`
Expected: PASS (all corpus tests).

- [ ] **Step 5: Commit**

```bash
git add mcp/corpus/search.go mcp/corpus/search_test.go
git commit -m "MCP corpus: field-weighted BM25-lite search with snippets"
git push origin main
```

---

## Task 4: MCP tool contracts and handlers

**Files:**
- Create: `mcp/tools.go`
- Test: `mcp/tools_test.go`

- [ ] **Step 1: Write the failing handler tests**

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd mcp && go test .`
Expected: FAIL — `undefined: handleSearch`, `undefined: searchInput`, etc.

- [ ] **Step 3: Implement `mcp/tools.go`**

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd mcp && go test .`
Expected: PASS (all handler tests).

- [ ] **Step 5: Commit**

```bash
git add mcp/tools.go mcp/tools_test.go
git commit -m "MCP: search_fiskaly_docs and fetch_fiskaly_doc tool handlers"
git push origin main
```

---

## Task 5: Wire the tools into the server

**Files:**
- Modify: `mcp/main.go`

- [ ] **Step 1: Replace `mcp/main.go` with the wired server**

```go
// Command fiskaly-mcp is the fiskaly MCP server. It serves the curated SIGN IT
// documentation corpus through two read-only tools, search_fiskaly_docs and
// fetch_fiskaly_doc, so a consumer agent can ground its integration in the docs.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
)

func main() {
	c, err := corpus.Load()
	if err != nil {
		log.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "v0.1.0"}, nil)
	registerTools(server, c)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build the whole module**

Run: `cd mcp && go build ./... && go vet ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add mcp/main.go
git commit -m "MCP: serve the docs tools from the server entrypoint"
git push origin main
```

---

## Task 6: In-memory integration test (real server, real corpus)

**Files:**
- Create: `mcp/server_test.go`

Exercises the wired server end to end over the SDK's in-memory transport: tool listing, a real search, a real fetch, and the unknown-id error path. More reliable than hand-framed stdio JSON-RPC.

- [ ] **Step 1: Write the integration test**

```go
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
)

func connectTestServer(t *testing.T) (*mcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()
	c, err := corpus.Load()
	if err != nil {
		t.Fatalf("corpus.Load: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "test"}, nil)
	registerTools(server, c)
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
	session, ctx := connectTestServer(t)
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
	session, ctx := connectTestServer(t)

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
	session, ctx := connectTestServer(t)
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
```

- [ ] **Step 2: Run the integration test**

Run: `cd mcp && go test .`
Expected: PASS (handler tests from Task 4 plus the three server tests).

- [ ] **Step 3: Commit**

```bash
git add mcp/server_test.go
git commit -m "MCP: in-memory integration test for the docs tools"
git push origin main
```

---

## Task 7: Run the eval gate (AGENTS.md: feature is done only when evals pass)

**Files:** none (runs the harness).

> This task spends a real headless Claude run via `evals/run-eval.sh`. It is the acceptance gate, not a unit test.

- [ ] **Step 1: Run the eval with the new MCP**

Run: `RUN_MODEL=claude-sonnet-4-6 RUN_EFFORT=high evals/run-eval.sh`
Expected: prints a run dir and `build: PASS  tests: PASS  judge: PASS` at the end. Note the printed `run dir:` path as `$RUN`.

- [ ] **Step 2: Assert the agent grounded itself before coding**

Run: `evals/assert-grounded.sh "$RUN/transcript.jsonl"; echo "exit=$?"`
Expected: `GROUNDED: searched (...) before first code change (...)` and `exit=0`.

- [ ] **Step 3: Confirm no regression in the fixture tests**

Run: `(cd "$RUN/pos" && go test ./...)`
Expected: `ok` — the agent kept existing tests green and added new ones.

- [ ] **Step 4: Record the result**

If all three pass, the gate is met: the tool moved a clean-room agent from "can't integrate" (control: empty-MCP baseline runs in the dashboard) to "passes the conformance judge". If the judge fails, inspect `"$RUN/judge.txt"` and `"$RUN/changes.diff"` to see which contract rule the agent missed, and whether the seed corpus covers it; add or sharpen the missing section in `mcp/corpus/index.json` and re-run from Step 1.

---

## Follow-on: Plan B (corpus generator)

Out of scope for this plan; write as its own spec→plan when Plan A's gate is green. Sketch:

- `mcp/corpus/gen/` — a `go:generate` tool reading `research/specs/fiskaly_oas_2026-02-03.yaml` (the OpenAPI template) + `research/specs/fiskaly_IT_2026-02-03.yaml` (the IT overlay), plus curated markdown under `mcp/corpus/sources/{brief,kb}/`.
- Resolve `((<key))` placeholders against the overlay (missing key = the 168-blank-descriptions defect → log it; this is opportunity #3 in miniature) and recursively resolve `$ref: '#/components/...'`.
- Emit one `Section` per operation (`id: "spec:POST /records"`, `url: ".../sign-it/2026-02-03#operation/<operationId>"`), one per KB article, one per brief section — overwriting `mcp/corpus/index.json`.
- New dependency: a YAML parser (`gopkg.in/yaml.v3`). Keep KB/brief bodies as committed local markdown so the build stays hermetic (no network at build time).
```
