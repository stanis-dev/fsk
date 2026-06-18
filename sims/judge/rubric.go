// rubric.go is the LLM rubric layer that runs behind the deterministic gate.
// It grades the integration source against a per-scenario rubric (authored in
// scenario.json, seeded from the SOLUTION.md answer keys), with evidence-required
// binary verdicts and a citation check, and is conservative to a false PASS: any
// criterion that is not a cited MET makes the integration NON-COMPLIANT.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

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
// for a bare JSON object, but tolerate a ```json fence and surrounding prose by
// scanning for the first brace-balanced object (ignoring braces inside strings).
func parseModelJSON(text string) ([]verdict, error) {
	obj, err := firstJSONObject(text)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Criteria []verdict `json:"criteria"`
	}
	if err := json.Unmarshal([]byte(obj), &payload); err != nil {
		return nil, fmt.Errorf("parsing model JSON: %w", err)
	}
	return payload.Criteria, nil
}

// firstJSONObject returns the first top-level brace-balanced {...} in s, tracking
// string literals and escapes so a brace inside a quoted value does not end it.
func firstJSONObject(s string) (string, error) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", fmt.Errorf("no JSON object found")
	}
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
				return s[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced JSON object")
}

// citationCheck enforces that every MET is backed by evidence that actually
// appears in the (comment-stripped) source. A MET with an empty quote, or whose
// quote is not present, is downgraded to UNMET. This is the anti-hallucination and
// anti-gaming guard: a comment claiming correctness cannot satisfy a criterion
// because the quote is matched against stripped source.
func citationCheck(vs []verdict, strippedSource string) []verdict {
	for i := range vs {
		if vs[i].Verdict != "MET" {
			continue
		}
		q := strings.TrimSpace(vs[i].EvidenceQuote)
		if q == "" || !strings.Contains(strippedSource, q) {
			vs[i].Verdict = "UNMET"
			vs[i].Reasoning = strings.TrimSpace(vs[i].Reasoning + " [citation not found in source]")
		}
	}
	return vs
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
