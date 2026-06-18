package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	evSearch = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fiskaly__search_fiskaly_docs"}]}}`
	evWrite  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`
)

func TestCheckGrounded_SearchBeforeWrite(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evSearch, evWrite))
	if !ok {
		t.Fatalf("expected grounded, got %q", verdict)
	}
}

func TestCheckGrounded_WriteBeforeSearch(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite, evSearch))
	if ok {
		t.Fatalf("expected NOT grounded, got %q", verdict)
	}
}

func TestCheckGrounded_NeverSearched(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evWrite))
	if ok || verdict == "" {
		t.Fatalf("expected NOT grounded with a reason, got ok=%v %q", ok, verdict)
	}
}

func TestCheckGrounded_SearchedNeverWrote(t *testing.T) {
	ok, verdict := checkGrounded(writeTranscript(t, evSearch))
	if ok || verdict == "" {
		t.Fatalf("expected NOT grounded with a reason, got ok=%v %q", ok, verdict)
	}
}

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
