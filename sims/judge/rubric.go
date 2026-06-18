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
