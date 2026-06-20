package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"backend/internal/scenarios"
)

const (
	evSearch = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fiskaly__search_fiskaly_docs"}]}}`
	evWrite  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`
)

type fakeAgent struct{}

func (f fakeAgent) build(_ context.Context) error { return nil }

func (f fakeAgent) run(ctx context.Context, rd runDir, task string, cfg runConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tr := evSearch + "\n" + evWrite + "\n"
	if err := os.WriteFile(filepath.Join(rd.path, "transcript.jsonl"), []byte(tr), 0o644); err != nil {
		return err
	}
	pos := filepath.Join(rd.work, "pos.go")
	b, err := os.ReadFile(pos)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(pos, append(b, []byte("\n// touched by fake agent\n")...), 0o644)
}

func TestContainerName(t *testing.T) {
	if got := containerName("/x/y/run.AbC.123"); got != "fiskaly-eval-run.AbC.123" {
		t.Errorf("containerName = %q", got)
	}
}

func TestRunScenario_ArtifactsWritten(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the go toolchain and the judge")
	}
	ehRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := scenarios.Discover(filepath.Join(ehRoot, "backend", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	one := sc[0] // 01-zero-to-receipt

	res, err := runScenario(context.Background(), one, t.TempDir(), fakeAgent{}, runConfig{model: "m", effort: "e"}, nil)
	if err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	for _, name := range []string{"meta.json", "build.txt", "test.txt", "judge.txt", "judge.json", "changes.diff", "transcript.jsonl"} {
		if _, err := os.Stat(filepath.Join(res.runDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}

// blockingAgent blocks run until ctx is cancelled, simulating a long container run.
type blockingAgent struct{}

func (b blockingAgent) build(_ context.Context) error { return nil }

func (b blockingAgent) run(ctx context.Context, rd runDir, task string, cfg runConfig) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestRunScenario_CancelStopsRun(t *testing.T) {
	ehRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := scenarios.Discover(filepath.Join(ehRoot, "backend", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	one := sc[0]

	ctx, cancel := context.WithCancel(context.Background())
	runsBase := t.TempDir()

	done := make(chan struct {
		res scenarioResult
		err error
	}, 1)
	go func() {
		res, err := runScenario(ctx, one, runsBase, blockingAgent{}, runConfig{model: "m", effort: "e"}, nil)
		done <- struct {
			res scenarioResult
			err error
		}{res, err}
	}()

	cancel()

	result := <-done
	if result.err == nil {
		t.Fatal("expected error after cancel, got nil")
	}
	if !errors.Is(result.err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", result.err)
	}

	// Capture the run dir from the single child of runsBase (created before cancel)
	// and assert judge.txt was not written.
	entries, err := os.ReadDir(runsBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 0 {
		runDir := filepath.Join(runsBase, entries[0].Name())
		if _, err := os.Stat(filepath.Join(runDir, "judge.txt")); err == nil {
			t.Error("judge.txt must not be written when run is cancelled")
		}
	}
}
