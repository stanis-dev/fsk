package main

import (
	"os"
	"path/filepath"
	"testing"
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

	got, err := discoverScenarios(root)
	if err != nil {
		t.Fatalf("discoverScenarios: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d scenarios, want 2: %+v", len(got), got)
	}
	if got[0].id != "01-alpha" || got[1].id != "02-beta" {
		t.Errorf("scenarios not sorted/identified: %s, %s", got[0].id, got[1].id)
	}
	if filepath.Base(got[0].fixtureDir) != "fixture" {
		t.Errorf("fixtureDir = %q, want .../fixture", got[0].fixtureDir)
	}
	if filepath.Base(got[0].scenarioJSON) != "scenario.json" {
		t.Errorf("scenarioJSON = %q, want .../scenario.json", got[0].scenarioJSON)
	}
}

func TestFindSimsRoot(t *testing.T) {
	root := t.TempDir()
	sims := filepath.Join(root, "eval-harness")
	mustMkdir(t, filepath.Join(sims, "scenarios"))
	mustMkdir(t, filepath.Join(sims, "judge"))

	// Above sims: a dir that contains sims/.
	if got, err := findSimsRoot(root); err != nil || got != sims {
		t.Errorf("from parent: got %q, %v; want %q", got, err, sims)
	}
	// Inside sims: the dir itself has scenarios/ and judge/.
	if got, err := findSimsRoot(sims); err != nil || got != sims {
		t.Errorf("from sims itself: got %q, %v; want %q", got, err, sims)
	}
	// A grandchild walks up to find it.
	grandchild := filepath.Join(sims, "judge")
	if got, err := findSimsRoot(grandchild); err != nil || got != sims {
		t.Errorf("from grandchild: got %q, %v; want %q", got, err, sims)
	}
	// A tree with no sims/ terminates with an error instead of looping.
	if _, err := findSimsRoot(t.TempDir()); err == nil {
		t.Error("expected error when no sims/ is present")
	}
}

func TestIsSimsDir_RequiresBoth(t *testing.T) {
	only := t.TempDir()
	mustMkdir(t, filepath.Join(only, "scenarios"))
	if isSimsDir(only) {
		t.Error("a dir with only scenarios/ should not be a sims dir")
	}
	mustMkdir(t, filepath.Join(only, "judge"))
	if !isSimsDir(only) {
		t.Error("a dir with both scenarios/ and judge/ should be a sims dir")
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
	if _, err := discoverScenarios(t.TempDir()); err == nil {
		t.Fatal("expected error when no scenarios are present")
	}
}

func TestRunGoCmd_PassAndFail(t *testing.T) {
	good := t.TempDir()
	writeFile(t, filepath.Join(good, "go.mod"), "module good\n\ngo 1.23\n")
	writeFile(t, filepath.Join(good, "good.go"), "package good\n\nfunc Add(a, b int) int { return a + b }\n")
	if r := runGoCmd(good, "build", "./..."); !r.OK {
		t.Errorf("build of valid module failed: %s", r.Output)
	}

	bad := t.TempDir()
	writeFile(t, filepath.Join(bad, "go.mod"), "module bad\n\ngo 1.23\n")
	writeFile(t, filepath.Join(bad, "bad.go"), "package bad\n\nfunc Broken() int { return }\n")
	if r := runGoCmd(bad, "build", "./..."); r.OK {
		t.Errorf("build of broken module unexpectedly succeeded: %s", r.Output)
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
