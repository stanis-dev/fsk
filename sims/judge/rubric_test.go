package main

import (
	"testing"
)

func TestParseScenarioRubric(t *testing.T) {
	data := []byte(`{"judge":{"rules":["fiskaly-host"],"rubric":[
		{"id":"c1","criterion":"does X","where":"checkout.go","cite":"SOLUTION.md"}]}}`)
	got, err := parseScenarioRubric(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "c1" || got[0].Criterion != "does X" {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseScenarioRubricEmpty(t *testing.T) {
	got, err := parseScenarioRubric([]byte(`{"judge":{"rules":["x"]}}`))
	if err != nil || got != nil {
		t.Fatalf("want nil,nil got %+v %v", got, err)
	}
}
