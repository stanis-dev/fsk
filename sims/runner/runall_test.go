package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterScenarios_PrefixAndExact(t *testing.T) {
	all := []scenario{{id: "01-zero-to-receipt"}, {id: "06-fire-and-forget"}}
	got, err := filterScenarios(all, []string{"06"})
	if err != nil || len(got) != 1 || got[0].id != "06-fire-and-forget" {
		t.Fatalf("prefix match failed: %+v err=%v", got, err)
	}
	got, err = filterScenarios(all, []string{"01-zero-to-receipt"})
	if err != nil || len(got) != 1 || got[0].id != "01-zero-to-receipt" {
		t.Fatalf("exact match failed: %+v err=%v", got, err)
	}
	if _, err := filterScenarios(all, []string{"99"}); err == nil {
		t.Error("expected error for an id matching nothing")
	}
}

func TestRunAll_AllPassExitsZero(t *testing.T) {
	if testing.Short() {
		t.Skip("requires building the judge")
	}
	simsRoot, _ := filepath.Abs("..")
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	code := runAll(sc[:1], t.TempDir(), judgeBin, fakeAgent{}, runConfig{model: "m", effort: "e"}, &b)
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, b.String())
	}
}
