package main

import (
	"strings"
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

func TestParseModelJSON(t *testing.T) {
	cases := []string{
		`{"criteria":[{"id":"c1","verdict":"MET","evidence_quote":"x","reasoning":"r"}]}`,
		"here:\n```json\n{\"criteria\":[{\"id\":\"c1\",\"verdict\":\"UNMET\",\"evidence_quote\":\"\",\"reasoning\":\"r\"}]}\n```\n",
		"blah {\"criteria\":[{\"id\":\"c1\",\"verdict\":\"CANNOT_ASSESS\",\"evidence_quote\":\"\",\"reasoning\":\"r\"}]} trailing",
	}
	for i, c := range cases {
		got, err := parseModelJSON(c)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if len(got) != 1 || got[0].ID != "c1" {
			t.Fatalf("case %d bad: %+v", i, got)
		}
	}
	if _, err := parseModelJSON("no json here"); err == nil {
		t.Fatal("want error on no-json")
	}
}

func TestParseModelJSONHandlesBraceInString(t *testing.T) {
	// A brace inside a string literal must not end the object early.
	in := `{"criteria":[{"id":"c1","verdict":"UNMET","evidence_quote":"map[string]int{}","reasoning":"r"}]}`
	got, err := parseModelJSON(in)
	if err != nil || len(got) != 1 || got[0].EvidenceQuote != "map[string]int{}" {
		t.Fatalf("brace-in-string mishandled: %+v err=%v", got, err)
	}
}

func TestCitationCheckDowngradesAbsentEvidence(t *testing.T) {
	out := citationCheck([]verdict{{ID: "a", Verdict: "MET", EvidenceQuote: "o.VATRate"}}, "x := o.VATRate * 100")
	if out[0].Verdict != "MET" {
		t.Fatal("present evidence should stay MET")
	}
	out = citationCheck([]verdict{{ID: "b", Verdict: "MET", EvidenceQuote: "MenuVAT[item]"}}, "x := o.VATRate")
	if out[0].Verdict != "UNMET" {
		t.Fatal("absent evidence must downgrade to UNMET")
	}
	out = citationCheck([]verdict{{ID: "c", Verdict: "MET", EvidenceQuote: ""}}, "anything")
	if out[0].Verdict != "UNMET" {
		t.Fatal("empty evidence on a MET must downgrade to UNMET")
	}
}

func TestConformant(t *testing.T) {
	if !conformant([]verdict{{Verdict: "MET"}, {Verdict: "MET"}}) {
		t.Fatal("all MET => conformant")
	}
	if conformant([]verdict{{Verdict: "MET"}, {Verdict: "UNMET"}}) {
		t.Fatal("any UNMET => not conformant")
	}
	if conformant([]verdict{{Verdict: "CANNOT_ASSESS"}}) {
		t.Fatal("CANNOT_ASSESS => not conformant")
	}
	if conformant(nil) {
		t.Fatal("no criteria => not conformant")
	}
}

func TestBuildRubricPrompt(t *testing.T) {
	p := buildRubricPrompt("package main // src", []criterion{
		{ID: "c1", Criterion: "check X", Where: "foo.go", Cite: "NOTES"},
	})
	for _, want := range []string{"c1", "check X", "foo.go", "NOTES", "package main // src", "MET", "UNMET", "CANNOT_ASSESS", "evidence_quote", "JSON"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
