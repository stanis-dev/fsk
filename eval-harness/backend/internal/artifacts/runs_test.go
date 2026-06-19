package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixtureDir returns the path to the dashboard fixtures directory relative to
// this source file so the tests are location-independent.
func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file: eval-harness/backend/internal/artifacts/runs_test.go
	// fixtures: eval-harness/dashboard/__fixtures__
	return filepath.Join(filepath.Dir(file), "../../../../eval-harness/dashboard/__fixtures__")
}

func TestSummarizeRunDoneFixture(t *testing.T) {
	sample := filepath.Join(fixtureDir(t), "run.sample")
	s := SummarizeRun(sample)
	if s.Status != "done" {
		t.Errorf("status = %q, want done", s.Status)
	}
	if s.Judge != "PASS" {
		t.Errorf("judge = %q, want PASS", s.Judge)
	}
	if s.Build != "PASS" {
		t.Errorf("build = %q, want PASS", s.Build)
	}
	if s.Tests != "PASS" {
		t.Errorf("tests = %q, want PASS", s.Tests)
	}
	if s.Harness != "docker" {
		t.Errorf("harness = %q, want docker", s.Harness)
	}
	if s.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", s.Model)
	}
	if s.Coder != "claude-code" {
		t.Errorf("coder = %q, want claude-code", s.Coder)
	}
	if s.Turns != "12" {
		t.Errorf("turns = %q, want 12", s.Turns)
	}
	if s.Cost != "$1.50" {
		t.Errorf("cost = %q, want $1.50", s.Cost)
	}
}

func TestLoadRunTranscriptDiffJudge(t *testing.T) {
	fixtures := fixtureDir(t)
	rd, ok := LoadRun(fixtures, "run.sample")
	if !ok || rd == nil {
		t.Fatal("LoadRun returned false")
	}
	hasTool := false
	for _, e := range rd.Transcript {
		if e.Kind == "tool" {
			hasTool = true
			break
		}
	}
	if !hasTool {
		t.Error("transcript has no tool events")
	}
	hasAdd := false
	for _, l := range rd.Diff {
		if l.Cls == "add" {
			hasAdd = true
			break
		}
	}
	if !hasAdd {
		t.Error("diff has no add lines")
	}
	if !strings.Contains(rd.JudgeLog, "conformant") {
		t.Errorf("judgeLog does not contain 'conformant': %q", rd.JudgeLog)
	}
}

func TestParseJudgeReportNullAndGarbage(t *testing.T) {
	if ParseJudgeReport("") != nil {
		t.Error("empty string should return nil")
	}
	if ParseJudgeReport("not json") != nil {
		t.Error("garbage should return nil")
	}
}

func TestParseJudgeReportExpectationsShape(t *testing.T) {
	// truthy expectations without criteria array → nil
	if ParseJudgeReport(`{"verdict":"conformant","expectations":{"model":"m"}}`) != nil {
		t.Error("expectations without criteria should return nil")
	}
	// criteria explicitly null → nil
	if ParseJudgeReport(`{"verdict":"conformant","expectations":{"model":"m","criteria":null}}`) != nil {
		t.Error("criteria:null should return nil")
	}
	// expectations null → ok
	r := ParseJudgeReport(`{"verdict":"conformant","expectations":null,"checks":{"passed":true,"results":[]},"scenario":"x","note":""}`)
	if r == nil {
		t.Error("expectations:null should not return nil")
	}
}

func TestParseJudgeReportFull(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"scenario": "07-wrong-vat",
		"verdict":  "conformant",
		"checks":   map[string]any{"passed": true, "results": []map[string]any{{"id": "r1", "pass": true, "detail": "ok"}}},
		"expectations": map[string]any{
			"model": "claude-opus-4-8",
			"criteria": []map[string]any{
				{"id": "vat-derived-from-line", "verdict": "MET", "evidence_quote": "pct := line.VATRate", "reasoning": "ok"},
			},
		},
		"note": "",
	})
	r := ParseJudgeReport(string(raw))
	if r == nil {
		t.Fatal("expected non-nil report")
	}
	if r.Verdict != "conformant" {
		t.Errorf("verdict = %q", r.Verdict)
	}
	if !r.Checks.Passed {
		t.Error("checks.passed = false")
	}
	if r.Expectations == nil {
		t.Fatal("expectations is nil")
	}
	if len(r.Expectations.Criteria) == 0 || r.Expectations.Criteria[0].ID != "vat-derived-from-line" {
		t.Errorf("criteria[0].ID = %q", r.Expectations.Criteria[0].ID)
	}
}

func TestLoadRunJudgeReport(t *testing.T) {
	fixtures := fixtureDir(t)
	rd, ok := LoadRun(fixtures, "run.sample")
	if !ok || rd == nil {
		t.Fatal("LoadRun returned false")
	}
	if rd.JudgeReport == nil {
		t.Fatal("judgeReport is nil")
	}
	if rd.JudgeReport.Verdict != "conformant" {
		t.Errorf("verdict = %q", rd.JudgeReport.Verdict)
	}
	if rd.JudgeReport.Expectations == nil {
		t.Fatal("expectations is nil")
	}
	if len(rd.JudgeReport.Expectations.Criteria) == 0 {
		t.Fatal("criteria is empty")
	}
	if rd.JudgeReport.Expectations.Criteria[0].ID != "vat-derived-from-line" {
		t.Errorf("criteria[0].ID = %q", rd.JudgeReport.Expectations.Criteria[0].ID)
	}
	if rd.JudgeReport.Expectations.Criteria[0].Verdict != "MET" {
		t.Errorf("criteria[0].Verdict = %q", rd.JudgeReport.Expectations.Criteria[0].Verdict)
	}
}

func TestListRunsFindsFixture(t *testing.T) {
	fixtures := fixtureDir(t)
	runs := ListRuns(fixtures)
	found := false
	for _, s := range runs {
		if s.ID == "run.sample" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("run.sample not found in %v", runs)
	}
}

func TestSummarizeRunCancelledPrecedence(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, MetaFile), []byte(`{"scenario":"01-demo","harness":"docker"}`), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, JudgeLogFile), []byte("VERDICT: conformant\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, CancelledFile), []byte("2026-06-18T00:00:00Z\n"), 0o644))
	s := SummarizeRun(dir)
	if s.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", s.Status)
	}
}

func TestLoadRunEmptyDirNoNullSlices(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run.empty-test")
	must(t, os.Mkdir(runDir, 0o755))
	must(t, os.WriteFile(filepath.Join(runDir, MetaFile), []byte(`{"scenario":"01-demo","harness":"docker"}`), 0o644))

	rd, ok := LoadRun(dir, "run.empty-test")
	if !ok || rd == nil {
		t.Fatal("LoadRun returned false for valid empty run dir")
	}

	out, err := json.Marshal(rd)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(out)

	for _, want := range []string{`"transcript":[]`, `"diff":[]`, `"byTool":[]`, `"queries":[]`, `"docsFetched":[]`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled JSON missing %q; got: %s", want, s)
		}
	}
	for _, bad := range []string{`"transcript":null`, `"diff":null`, `"byTool":null`, `"queries":null`, `"docsFetched":null`} {
		if strings.Contains(s, bad) {
			t.Errorf("marshaled JSON contains %q (should be []); got: %s", bad, s)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
