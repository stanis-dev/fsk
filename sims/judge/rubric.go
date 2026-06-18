// rubric.go is the LLM rubric layer that runs behind the deterministic gate.
// It grades the integration source against a per-scenario rubric (authored in
// scenario.json, seeded from the SOLUTION.md answer keys), with evidence-required
// binary verdicts and a citation check, and is conservative to a false PASS: any
// criterion that is not a cited MET makes the integration NON-COMPLIANT.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// judgeModelID is the model the rubric layer judges with. It is a stronger,
// different tier than the coder under test (claude-sonnet-4-6) to reduce
// self-preference; both are still Claude (the harness auth is OAuth-only), so true
// model-family diversity is not available — see the design spec's limitations.
const judgeModelID = "claude-opus-4-8"

const judgeEffort = "high"

// claudeArgs are the CLI flags for a one-shot structured judgement. The prompt is
// passed on stdin, not as an argument, to avoid arg-length limits.
func claudeArgs(model, effort string) []string {
	return []string{"-p", "--model", model, "--effort", effort, "--output-format", "json"}
}

// claudeModel shells the claude CLI once and returns the assistant's final text.
// It is the real modelFn. There is no silent fallback: a missing binary, a non-zero
// exit, or an unparseable envelope is a hard error (the caller exits non-zero).
func claudeModel(prompt string) (string, error) {
	cmd := exec.Command("claude", claudeArgs(judgeModelID, judgeEffort)...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var env struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		return "", fmt.Errorf("parsing claude --output-format json: %w", err)
	}
	if strings.TrimSpace(env.Result) == "" {
		return "", fmt.Errorf("claude returned an empty result")
	}
	return env.Result, nil
}

// criterion is one atomic, binary rubric check, authored in scenario.json. It is
// what the deterministic regex layer provably cannot see (e.g. which VAT rate fed
// the breakdown), grounded by a short cite into the answer key.
type criterion struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Where     string `json:"where"`
	Cite      string `json:"cite"`
}

// parseScenarioRubric extracts judge.rubric from scenario.json bytes. A scenario
// with no rubric yields a nil slice and nil error (gate-only).
func parseScenarioRubric(data []byte) ([]criterion, error) {
	var s struct {
		Judge struct {
			Rubric []criterion `json:"rubric"`
		} `json:"judge"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing rubric: %w", err)
	}
	return s.Judge.Rubric, nil
}

// rubricFromScenario reads a scenario.json and returns its judge.rubric.
func rubricFromScenario(path string) ([]criterion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scenario: %w", err)
	}
	return parseScenarioRubric(data)
}

// verdict is the model's judgement of one criterion, after the citation check.
type verdict struct {
	ID            string `json:"id"`
	Verdict       string `json:"verdict"` // MET | UNMET | CANNOT_ASSESS
	EvidenceQuote string `json:"evidence_quote"`
	Reasoning     string `json:"reasoning"`
	Cite          string `json:"cite,omitempty"`
}

// parseModelJSON extracts the verdict array from a model reply. The model is asked
// for a bare JSON object, but tolerate a ```json fence and surrounding prose:
// consider every brace-balanced object in the text and return the first one that
// parses AND carries a non-empty criteria array. This skips prose that contains
// braces (e.g. a mention of map[string]int{}) instead of mistaking it for the
// answer.
func parseModelJSON(text string) ([]verdict, error) {
	var lastErr error = fmt.Errorf("no JSON object found")
	for _, obj := range jsonCandidates(text) {
		var payload struct {
			Criteria []verdict `json:"criteria"`
		}
		if err := json.Unmarshal([]byte(obj), &payload); err != nil {
			lastErr = err
			continue
		}
		if len(payload.Criteria) > 0 {
			return payload.Criteria, nil
		}
	}
	return nil, fmt.Errorf("parsing model JSON: %w", lastErr)
}

// jsonCandidates returns every brace-balanced {...} substring, left to right, so
// the outermost real object is tried before inner or prose objects.
func jsonCandidates(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			if obj := balancedFrom(s, i); obj != "" {
				out = append(out, obj)
			}
		}
	}
	return out
}

// balancedFrom returns the brace-balanced object starting at s[start] ('{'),
// tracking string literals and escapes so a brace inside a quoted value does not
// end it. Returns "" if it never balances before EOF.
func balancedFrom(s string, start int) string {
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// modelFn invokes a judge model with a prompt and returns its raw text reply.
// Injecting it keeps the pipeline unit-testable with a stub; claudeModel is the
// real implementation.
type modelFn func(prompt string) (string, error)

// rubricReport is the structured outcome of the rubric layer.
type rubricReport struct {
	Model    string    `json:"model"`
	Criteria []verdict `json:"criteria"`
}

// runRubric ties prompt -> model -> parse -> cite-fill -> citation check together.
// Source carries comments (for the model); stripped is the comment-stripped source
// the citation check validates against. Any criterion the model did not return is
// added as CANNOT_ASSESS so a skipped check can never silently pass.
func runRubric(source, stripped string, crits []criterion, model modelFn, modelName string) (rubricReport, error) {
	prompt := buildRubricPrompt(source, crits)
	// Retry only malformed output (a known nondeterministic failure mode of
	// structured LLM replies). A model invocation error is not retried — it is a
	// hard failure surfaced to the caller (no silent fallback).
	const maxAttempts = 3
	var vs []verdict
	var parseErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		raw, err := model(prompt)
		if err != nil {
			return rubricReport{}, fmt.Errorf("judge model: %w", err)
		}
		vs, parseErr = parseModelJSON(raw)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return rubricReport{}, fmt.Errorf("judge model output unparseable after %d attempts: %w", maxAttempts, parseErr)
	}
	byID := map[string]*verdict{}
	for i := range vs {
		byID[vs[i].ID] = &vs[i]
	}
	out := make([]verdict, 0, len(crits))
	for _, c := range crits {
		if v, ok := byID[c.ID]; ok {
			v.Cite = c.Cite
			out = append(out, *v)
			continue
		}
		out = append(out, verdict{
			ID:        c.ID,
			Verdict:   "CANNOT_ASSESS",
			Reasoning: "model returned no verdict for this criterion",
			Cite:      c.Cite,
		})
	}
	out = citationCheck(out, stripped)
	return rubricReport{Model: modelName, Criteria: out}, nil
}

// citationCheck enforces that every MET is backed by evidence that actually
// appears in the (comment-stripped) source. A MET with an empty quote, or whose
// quote is not present, is downgraded to UNMET. This is the anti-hallucination and
// anti-gaming guard: a comment claiming correctness cannot satisfy a criterion
// because the quote is matched against stripped source.
func citationCheck(vs []verdict, citationSource string) []verdict {
	normSrc := normalizeWS(citationSource)
	for i := range vs {
		if vs[i].Verdict != "MET" {
			continue
		}
		q := strings.TrimSpace(vs[i].EvidenceQuote)
		// Match whitespace-insensitively: the model copies from raw source, which
		// differs from the citation source only in indentation/line breaks.
		if q == "" || !strings.Contains(normSrc, normalizeWS(q)) {
			vs[i].Verdict = "UNMET"
			vs[i].Reasoning = strings.TrimSpace(vs[i].Reasoning + " [citation not found in source]")
		}
	}
	return vs
}

// normalizeWS collapses every run of whitespace (spaces, tabs, newlines) to a
// single space and trims, so a quote and the source match despite reflowing.
func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// conformant is conservative to a false PASS: the integration is conformant only
// if there is at least one verdict and every verdict is MET. Any UNMET or
// CANNOT_ASSESS (abstention) blocks the pass.
func conformant(vs []verdict) bool {
	if len(vs) == 0 {
		return false
	}
	for _, v := range vs {
		if v.Verdict != "MET" {
			return false
		}
	}
	return true
}

// buildRubricPrompt frames a conservative conformance review of one fiskaly
// integration. The source carries comments (the model reasons over them) but the
// caller's citation check later validates every MET quote against the
// comment-stripped source, so a comment that merely claims correctness cannot pass.
func buildRubricPrompt(source string, crits []criterion) string {
	var b strings.Builder
	b.WriteString(`You are a strict conformance reviewer for an Italian fiscalization integration
(fiskaly SIGN IT). The domain is tax-sensitive: a wrong PASS ships systematic
non-compliance, so when in doubt you FAIL. Judge ONLY the criteria below, each
independently, against the integration source. For each criterion return exactly
one verdict:
  - MET: the source clearly satisfies the criterion. You MUST copy a verbatim code
    span from the source into evidence_quote (real code, not a comment).
  - UNMET: the source violates the criterion or lacks the required behavior.
  - CANNOT_ASSESS: you cannot tell from the source. (This counts as not a pass.)
Default to UNMET or CANNOT_ASSESS rather than guessing MET.

Reply with ONLY one JSON object and no prose, no markdown fences:
{"criteria":[{"id":"<id>","verdict":"MET|UNMET|CANNOT_ASSESS","evidence_quote":"<verbatim code span or empty>","reasoning":"<one sentence>"}]}

CRITERIA:
`)
	for _, c := range crits {
		fmt.Fprintf(&b, "- id: %s\n  check: %s\n  where: %s\n  reference: %s\n", c.ID, c.Criterion, c.Where, c.Cite)
	}
	b.WriteString("\nINTEGRATION SOURCE:\n```go\n")
	b.WriteString(source)
	b.WriteString("\n```\n")
	return b.String()
}
