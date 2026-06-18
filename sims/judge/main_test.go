package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerdict(t *testing.T) {
	r := buildReport("07-wrong-vat", []ruleResult{{ID: "x", Pass: true}}, true,
		&rubricReport{Model: "claude-opus-4-8", Criteria: []verdict{{ID: "c1", Verdict: "UNMET"}}}, "NON-COMPLIANT")
	if r.Verdict != "NON-COMPLIANT" || r.Rubric == nil || r.Gate.Passed != true || r.Scenario != "07-wrong-vat" {
		t.Fatalf("bad report: %+v", r)
	}
}

func TestRenderRubricContainsFields(t *testing.T) {
	s := renderRubric(rubricReport{Model: "claude-opus-4-8", Criteria: []verdict{
		{ID: "c1", Verdict: "UNMET", Reasoning: "because", EvidenceQuote: "MenuVAT[x]", Cite: "SOLUTION"},
	}})
	for _, w := range []string{"UNMET", "c1", "because", "MenuVAT[x]", "SOLUTION", "claude-opus-4-8"} {
		if !strings.Contains(s, w) {
			t.Errorf("render missing %q", w)
		}
	}
}

func ruleByID(t *testing.T, id string) rule {
	t.Helper()
	for _, r := range catalog {
		if r.id == id {
			return r
		}
	}
	t.Fatalf("rule %q not in catalog", id)
	return rule{}
}

func TestDefaultRulesAreTheFiveBaseRules(t *testing.T) {
	byID := map[string]rule{}
	for _, r := range catalog {
		byID[r.id] = r
	}
	got, err := selectRules(byID, "")
	if err != nil {
		t.Fatalf("selectRules(\"\") error: %v", err)
	}
	want := defaultRules
	if len(got) != len(want) {
		t.Fatalf("default set size = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].id != want[i] {
			t.Errorf("default rule %d = %q, want %q", i, got[i].id, want[i])
		}
	}
}

func TestSelectRulesOrderAndUnknown(t *testing.T) {
	byID := map[string]rule{}
	for _, r := range catalog {
		byID[r.id] = r
	}
	got, err := selectRules(byID, "records-flow, polling ,fiskaly-host")
	if err != nil {
		t.Fatalf("selectRules error: %v", err)
	}
	want := []string{"records-flow", "polling", "fiskaly-host"}
	for i := range want {
		if got[i].id != want[i] {
			t.Errorf("selected[%d] = %q, want %q", i, got[i].id, want[i])
		}
	}
	if _, err := selectRules(byID, "no-such-rule"); err == nil {
		t.Error("expected error for unknown rule id, got nil")
	}
}

func TestPositiveRuleNeedsEveryWant(t *testing.T) {
	r := ruleByID(t, "vat-breakdown") // requires ALL of percentage/amount/exclusive/inclusive
	// Three of the four present is not enough.
	if r.pass(`{"exclusive":"1.00","inclusive":"1.22","percentage":"22"}`) {
		t.Error("vat-breakdown passed without the 'amount' field")
	}
	if !r.pass(`{"percentage":"22","amount":"0.66","exclusive":"3.00","inclusive":"3.66"}`) {
		t.Error("vat-breakdown failed with all four fields present")
	}
}

func TestApiVersionCurrentNeedsHeaderAndDate(t *testing.T) {
	r := ruleByID(t, "api-version-current")
	if r.pass(`const apiVersion = "2026-02-03"`) {
		t.Error("api-version-current passed with the date but no X-Api-Version header")
	}
	if r.pass(`req.Header.Set("X-Api-Version", "2025-08-12")`) {
		t.Error("api-version-current passed with an old date")
	}
	if !r.pass(`req.Header.Set("X-Api-Version", "2026-02-03")`) {
		t.Error("api-version-current failed with both the header and the current date")
	}
}

func TestDenyRuleFailsWhenForbiddenTokenAppears(t *testing.T) {
	r := ruleByID(t, "no-invented-refunds") // deny /refunds
	if !r.pass(`http.Post(base + "/records", ...)`) {
		t.Error("no-invented-refunds should pass when /refunds is absent")
	}
	if r.pass(`http.Post(base + "/refunds", ...)`) {
		t.Error("no-invented-refunds should fail when /refunds appears")
	}
}

// A deny rule must fire on real request construction but NOT on an explanatory
// comment — readSource strips comments before rules run.
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
	if !ruleByID(t, "no-invented-refunds").pass(src) {
		t.Error("no-invented-refunds wrongly failed a correct impl that only mentions /refunds in a comment")
	}
	// And it still fires when /refunds is a real path string.
	bad := "package x\nfunc void() { http.Post(base+\"/refunds\", nil) }\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	src, _ = readSource(dir)
	if ruleByID(t, "no-invented-refunds").pass(src) {
		t.Error("no-invented-refunds should fail on a real /refunds request")
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
