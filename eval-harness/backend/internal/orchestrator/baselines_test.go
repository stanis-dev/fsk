package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/scenarios"
)

func TestDiscoverScenarios(t *testing.T) {
	root := t.TempDir()
	// Two valid scenarios.
	mkScenario(t, root, "01-alpha", true, true)
	mkScenario(t, root, "02-beta", true, true)
	// A numeric dir missing its fixture: not a runnable scenario.
	mkScenario(t, root, "03-no-fixture", false, true)
	// A numeric dir missing scenario.json: not a runnable scenario.
	mkScenario(t, root, "04-no-json", true, false)
	// A non-numeric dir (e.g. docs): ignored.
	mkScenario(t, root, "AUTHORING", true, true)

	got, err := scenarios.Discover(root)
	if err != nil {
		t.Fatalf("scenarios.Discover: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d scenarios, want 2: %+v", len(got), got)
	}
	if got[0].ID != "01-alpha" || got[1].ID != "02-beta" {
		t.Errorf("scenarios not sorted/identified: %s, %s", got[0].ID, got[1].ID)
	}
	if filepath.Base(got[0].FixtureDir) != "fixture" {
		t.Errorf("FixtureDir = %q, want .../fixture", got[0].FixtureDir)
	}
	if filepath.Base(got[0].ScenarioJSON) != "scenario.json" {
		t.Errorf("ScenarioJSON = %q, want .../scenario.json", got[0].ScenarioJSON)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	mustMkdir(t, filepath.Join(src, "nested"))
	writeFile(t, filepath.Join(src, "nested", "deep.go"), "package nested\n")
	exe := filepath.Join(src, "run.sh")
	writeFile(t, exe, "#!/bin/sh\necho hi\n")
	if err := os.Chmod(exe, 0o755); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "nested", "deep.go"))
	if err != nil || string(got) != "package nested\n" {
		t.Errorf("nested file not copied byte-identically: %q, %v", got, err)
	}
	fi, err := os.Stat(filepath.Join(dst, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("file mode = %o, want 755", fi.Mode().Perm())
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverScenarios_NoneIsError(t *testing.T) {
	if _, err := scenarios.Discover(t.TempDir()); err == nil {
		t.Fatal("expected error when no scenarios are present")
	}
}

func TestRunGoCmd_PassAndFail(t *testing.T) {
	good := t.TempDir()
	writeFile(t, filepath.Join(good, "go.mod"), "module good\n\ngo 1.26.4\n")
	writeFile(t, filepath.Join(good, "good.go"), "package good\n\nfunc Add(a, b int) int { return a + b }\n")
	if r := runGoCmd(good, "build", "./..."); strings.TrimSpace(r.Output) != "" {
		t.Errorf("build of valid module failed: %s", r.Output)
	}

	bad := t.TempDir()
	writeFile(t, filepath.Join(bad, "go.mod"), "module bad\n\ngo 1.26.4\n")
	writeFile(t, filepath.Join(bad, "bad.go"), "package bad\n\nfunc Broken() int { return }\n")
	if r := runGoCmd(bad, "build", "./..."); strings.TrimSpace(r.Output) == "" {
		t.Errorf("build of broken module unexpectedly succeeded")
	}
}

func mkScenario(t *testing.T, root, id string, withFixture, withJSON bool) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withFixture {
		if err := os.MkdirAll(filepath.Join(dir, "fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if withJSON {
		writeFile(t, filepath.Join(dir, "scenario.json"), `{"id":"`+id+`"}`)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
