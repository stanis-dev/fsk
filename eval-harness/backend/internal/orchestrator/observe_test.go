package orchestrator

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitDiffStaged(t *testing.T) {
	work := t.TempDir()
	gitInit(t, work)
	writeFile(t, filepath.Join(work, "a.go"), "package a\n")
	gitCommitAll(t, work, "baseline")
	writeFile(t, filepath.Join(work, "a.go"), "package a\n\nvar X = 1\n")

	diff, err := gitDiffStaged(work)
	if err != nil {
		t.Fatalf("gitDiffStaged: %v", err)
	}
	if !strings.Contains(diff, "var X = 1") {
		t.Errorf("diff missing the change:\n%s", diff)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	mustGit(t, dir, "init", "-q")
}

func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	mustGit(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "add", "-A")
	mustGit(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", msg)
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
