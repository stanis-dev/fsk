package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripCommentsKeepLayout(t *testing.T) {
	in := "package p\nx := 1 // inline\n/* block */ y := 2\n"
	out := stripCommentsKeepLayout(in)
	if !strings.Contains(out, "x := 1") || !strings.Contains(out, "y := 2") {
		t.Fatalf("code not preserved verbatim: %q", out)
	}
	if strings.Contains(out, "inline") || strings.Contains(out, "block") {
		t.Fatalf("comments not removed: %q", out)
	}
}

func TestStripCommentsKeepLayoutHandlesCRLF(t *testing.T) {
	// go/scanner drops lone \r from the COMMENT literal, so start+len(lit) would
	// undercount the span and leak trailing comment bytes. CR-padded comments are
	// valid Go and could otherwise smuggle text into the citation source.
	in := "package p\n/*" + strings.Repeat("\r", 5) + "LEAKCLAIM */ realCode\n"
	out := stripCommentsKeepLayout(in)
	if strings.Contains(out, "LEAKCLAIM") {
		t.Fatalf("CR-padded comment leaked into citation source: %q", out)
	}
	if !strings.Contains(out, "realCode") {
		t.Fatalf("code after the comment was dropped: %q", out)
	}
}

func TestDenyRuleIgnoresComments(t *testing.T) {
	dir := t.TempDir()
	correct := "package x\n" +
		"// Do not call /refunds; void via /records CANCELLATION instead.\n" +
		"func void() { http.Post(base+\"/records\", nil) }\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(correct), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := readSource(dir)
	if err != nil {
		t.Fatalf("readSource error: %v", err)
	}
	if contains(src, "/refunds") {
		t.Error("readSource kept /refunds from a comment; comments must be stripped")
	}
}

func TestReadSourceExcludesTests(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nconst Host = \"test.api.fiskaly.com\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nconst Forbidden = \"/refunds\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := readSource(dir)
	if err != nil {
		t.Fatalf("readSource error: %v", err)
	}
	if want := "test.api.fiskaly.com"; !contains(src, want) {
		t.Errorf("readSource dropped non-test source (missing %q)", want)
	}
	if contains(src, "/refunds") {
		t.Error("readSource included a _test.go file; tests must be excluded")
	}
}

func TestVerdict_GateFailSkipsExpectations(t *testing.T) {
	rs := []checkResult{{ID: "x", Pass: false, Detail: "d"}}
	if checksPassed(rs) {
		t.Fatal("precondition")
	}
	// buildReport on a failed gate yields NON-COMPLIANT with nil expectations
	rep := buildReport("s", checksReport{Passed: false, Results: rs}, nil, "NON-COMPLIANT")
	if rep.Verdict != "NON-COMPLIANT" || rep.Expectations != nil {
		t.Errorf("gate-fail report wrong: %+v", rep)
	}
}

func TestBuildReport(t *testing.T) {
	cr := checksReport{Passed: true, Results: []checkResult{{ID: "x", Pass: true}}}
	r := buildReport("07-wrong-vat", cr,
		&rubricReport{Model: "claude-opus-4-8", Criteria: []verdict{{ID: "c1", Verdict: "UNMET"}}}, "NON-COMPLIANT")
	if r.Verdict != "NON-COMPLIANT" || r.Expectations == nil || !r.Checks.Passed || r.Scenario != "07-wrong-vat" {
		t.Fatalf("bad report: %+v", r)
	}
}

func TestRenderExpectationsContainsFields(t *testing.T) {
	s := renderExpectations(rubricReport{Model: "claude-opus-4-8", Criteria: []verdict{
		{ID: "c1", Verdict: "UNMET", Reasoning: "because", EvidenceQuote: "MenuVAT[x]"},
	}})
	for _, w := range []string{"UNMET", "c1", "because", "MenuVAT[x]", "claude-opus-4-8"} {
		if !strings.Contains(s, w) {
			t.Errorf("render missing %q", w)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
