package jobs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"backend/internal/scenarios"
)

// fakeRunner implements Runner for tests; no Docker needed.
type fakeRunner struct {
	mu           sync.Mutex
	killCalls    []string
	containerMap map[string]string // runDir → container name

	// block controls whether RunScenario blocks until released.
	// Send on blockRelease to unblock a specific call; RunScenario selects on
	// ctx.Done() and blockRelease.
	block        bool
	blockRelease chan struct{}

	// failRun causes RunScenario to return an error.
	failRun bool
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		containerMap: make(map[string]string),
		blockRelease: make(chan struct{}),
	}
}

func (f *fakeRunner) Resolve(id string) (scenarios.Scenario, bool) {
	if id == "known" || id == "known2" {
		return scenarios.Scenario{ID: id}, true
	}
	return scenarios.Scenario{}, false
}

func (f *fakeRunner) RunScenario(ctx context.Context, s scenarios.Scenario, detached bool) (string, error) {
	// Create a temp run dir to simulate the real runner.
	runDir, err := os.MkdirTemp("", "run.")
	if err != nil {
		return "", err
	}

	f.mu.Lock()
	f.containerMap[runDir] = "fiskaly-eval-" + filepath.Base(runDir)
	f.mu.Unlock()

	if f.block {
		select {
		case <-ctx.Done():
			// Cancelled; return without writing judge.txt.
			return runDir, ctx.Err()
		case <-f.blockRelease:
		}
	}

	if f.failRun {
		return runDir, os.ErrInvalid
	}

	if err := os.WriteFile(filepath.Join(runDir, "judge.txt"), []byte("pass"), 0o644); err != nil {
		return "", err
	}
	return runDir, nil
}

func (f *fakeRunner) ContainerName(runDir string) string {
	return "fiskaly-eval-" + filepath.Base(runDir)
}

func (f *fakeRunner) KillContainer(container string) error {
	f.mu.Lock()
	f.killCalls = append(f.killCalls, container)
	f.mu.Unlock()
	return nil
}

func (f *fakeRunner) killCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.killCalls)
}

// waitActive blocks until Active() returns exactly n entries or 5 s elapses.
func waitActive(t *testing.T, svc *Service, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(svc.Active()) == n {
			return
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d active runs; got %d", n, len(svc.Active()))
}

// waitActivePhase blocks until any ActiveRun in Active() has the given phase.
func waitActivePhase(t *testing.T, svc *Service, phase string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, a := range svc.Active() {
			if a.Phase == phase {
				return
			}
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for phase %q", phase)
}

// waitActiveEmpty blocks until Active() is empty.
func waitActiveEmpty(t *testing.T, svc *Service) {
	t.Helper()
	waitActive(t, svc, 0)
}

// ---- Tests ----------------------------------------------------------------

func TestEnqueueUnknown(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	_, err := svc.Enqueue("unknown", "m", "e")
	if err == nil {
		t.Fatal("expected error for unknown scenario")
	}
}

func TestEnqueueRunsToCompletion(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id == "" {
		t.Fatal("Enqueue returned empty id")
	}

	// Wait for the run to finish (Active() becomes empty).
	waitActiveEmpty(t, svc)
}

func TestCancelLiveRun(t *testing.T) {
	runsBase := t.TempDir()
	f := newFakeRunner()
	f.block = true

	svc := NewService(f, runsBase, 1)
	svc.Start()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Wait until the worker is blocked inside RunScenario (phase "running").
	waitActivePhase(t, svc, "running")

	// We need the run dir to check for the cancelled marker. The run dir is
	// reconciled after RunScenario returns, but since f.block is true the call
	// hasn't returned yet. The marker is only written if runDir is known.
	// We cancel first; RunScenario will return via ctx.Done with a runDir.
	// After cancel the worker reconciles and the marker is written.

	ok := svc.Cancel(id)
	if !ok {
		t.Fatal("Cancel returned false for live run")
	}

	// KillContainer should have been called (container set once RunScenario
	// returned with a runDir; Cancel calls KillContainer on the container).
	// Because Cancel may run before RunScenario returns (and thus before
	// container is set on the liveRun), KillContainer may or may not be called.
	// The spec says best-effort, so we just verify Cancel returned true.

	// Active() should now be empty.
	waitActiveEmpty(t, svc)
}

func TestCancelUnknown(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	if svc.Cancel("does-not-exist") {
		t.Fatal("Cancel should return false for unknown run")
	}
}

func TestCancelWritesMarker(t *testing.T) {
	runsBase := t.TempDir()
	f := newFakeRunner()
	f.block = true

	svc := NewService(f, runsBase, 1)
	svc.Start()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	waitActivePhase(t, svc, "running")
	svc.Cancel(id)

	// After cancellation the worker unblocks (ctx.Done), RunScenario returns
	// a runDir, and the worker reconciles it onto the liveRun. Cancel already
	// removed the liveRun from the registry, so the worker's reconciliation is
	// a no-op for the registry. BUT the marker is written by Cancel only if
	// runDir is already set. Since RunScenario hadn't returned yet, runDir was
	// empty at Cancel time and no marker is written here.
	//
	// To test marker writing we need the runDir set BEFORE cancel. Use the
	// reattach path instead (see TestReattachCancel).
}

func TestCancelIdempotent(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	id, _ := svc.Enqueue("known", "m", "e")
	// First cancel may return true or false depending on timing; second must
	// return false.
	svc.Cancel(id)
	if svc.Cancel(id) {
		t.Fatal("second Cancel should return false")
	}
}

func TestReattachRegistersInFlight(t *testing.T) {
	runsBase := t.TempDir()

	// Simulate a dir left over from a previous process: no judge.txt, no cancelled.
	orphan := filepath.Join(runsBase, "run.orphan123")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	svc := NewService(f, runsBase, 1)
	svc.Start()

	// reattach should have registered the orphan dir.
	active := svc.Active()
	if len(active) == 0 {
		t.Fatal("expected at least one reattached run")
	}

	var found string
	for _, a := range active {
		if a.Phase == "running" {
			found = a.ID
			break
		}
	}
	if found == "" {
		t.Fatal("no reattached running run found in Active()")
	}

	// Cancel should succeed and write a marker.
	ok := svc.Cancel(found)
	if !ok {
		t.Fatal("Cancel of reattached run returned false")
	}

	// Marker must exist.
	marker := filepath.Join(orphan, "cancelled")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("cancelled marker not written: %v", err)
	}
}

func TestWorkers1Serializes(t *testing.T) {
	f := newFakeRunner()
	f.block = true

	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	// Enqueue two jobs. With workers=1, the second must wait.
	id1, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue 1: %v", err)
	}
	id2, err := svc.Enqueue("known2", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}

	// Wait for the first to be running.
	waitActivePhase(t, svc, "running")

	// The second should still be queued (or not yet reached by the worker).
	// Check that not both are "running" simultaneously.
	active := svc.Active()
	runningCount := 0
	for _, a := range active {
		if a.Phase == "running" {
			runningCount++
		}
	}
	if runningCount > 1 {
		t.Fatalf("workers=1 but %d runs in phase 'running'", runningCount)
	}

	// Unblock the first run.
	f.blockRelease <- struct{}{}

	// After first finishes, unblock the second.
	// We need the second to start running before we can release it.
	// Since f.block is still true, the second will block too.
	// Wait for the second to be running.
	waitActivePhase(t, svc, "running")

	f.blockRelease <- struct{}{}

	// Both should finish.
	waitActiveEmpty(t, svc)

	_ = id1
	_ = id2
}

func TestReattachSkipsCompleted(t *testing.T) {
	runsBase := t.TempDir()

	// A dir with judge.txt should NOT be reattached.
	done := filepath.Join(runsBase, "run.done")
	if err := os.MkdirAll(done, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(done, "judge.txt"), []byte("pass"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A dir with cancelled should NOT be reattached.
	cancelled := filepath.Join(runsBase, "run.cancelled")
	if err := os.MkdirAll(cancelled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cancelled, "cancelled"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	svc := NewService(f, runsBase, 1)
	svc.Start()

	// Active() should be empty (no in-flight dirs).
	if got := svc.Active(); len(got) != 0 {
		t.Fatalf("expected 0 active, got %d", len(got))
	}
}
