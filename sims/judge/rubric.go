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
