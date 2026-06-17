package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
	r := ruleByID(t, "vat-breakdown") // requires BOTH exclusive and inclusive
	if r.pass(`vat := map[string]string{"exclusive": "1.00"}`) {
		t.Error("vat-breakdown passed with only 'exclusive' present")
	}
	if !r.pass(`{"exclusive":"1.00","inclusive":"1.22","percentage":"22"}`) {
		t.Error("vat-breakdown failed with both exclusive and inclusive present")
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
