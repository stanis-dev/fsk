package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

// fakeRunner implements Runner for tests; no Docker needed.
type fakeRunner struct {
	mu        sync.Mutex
	killCalls []string

	// block controls whether RunScenario blocks until released.
	// Send on blockRelease to unblock a specific call; RunScenario selects on
	// ctx.Done() and blockRelease.
	block        bool
	blockRelease chan struct{}

	// failRun causes RunScenario to return an error.
	failRun  bool
	failKill bool
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		blockRelease: make(chan struct{}),
	}
}

func (f *fakeRunner) Resolve(id string) (scenarios.Scenario, bool) {
	if id == "known" || id == "known2" {
		return scenarios.Scenario{ID: id}, true
	}
	return scenarios.Scenario{}, false
}

func (f *fakeRunner) RunScenario(ctx context.Context, s scenarios.Scenario, model, effort string, detached bool, onStart func(runDir string)) (string, error) {
	// Create a temp run dir to simulate the real runner creating it before the
	// long coder step. Call onStart immediately so the registry records the dir
	// while the run is still in flight (mirrors the real runner's behaviour).
	runDir, err := os.MkdirTemp("", "run.")
	if err != nil {
		return "", err
	}

	if onStart != nil {
		onStart(runDir)
	}

	if f.block {
		select {
		case <-ctx.Done():
			// On cancel the real runner returns "" so the caller cannot rely on
			// the return value — the registry was already updated via onStart.
			return "", ctx.Err()
		case <-f.blockRelease:
		}
	}

	if f.failRun {
		return runDir, os.ErrInvalid
	}

	if err := os.WriteFile(filepath.Join(runDir, artifacts.JudgeLogFile), []byte("pass"), 0o644); err != nil {
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
	if f.failKill {
		return errors.New("kill failed")
	}
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

// waitRunDir blocks until the liveRun for runID has a non-empty runDir.
func waitRunDir(t *testing.T, svc *Service, runID string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		lr, ok := svc.live[runID]
		if ok && lr.runDir != "" {
			dir := lr.runDir
			svc.mu.Unlock()
			return dir
		}
		svc.mu.Unlock()
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for runDir on %s", runID)
	return ""
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

	// onStart fires before the block, so the registry has the runDir already.
	// Cancel finds the container name, kills it, and writes the marker.
	ok, err := svc.Cancel(id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !ok {
		t.Fatal("Cancel returned false for live run")
	}

	// Active() should now be empty.
	waitActiveEmpty(t, svc)
}

func TestCancelUnknown(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	ok, err := svc.Cancel("does-not-exist")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ok {
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

	// Wait until the worker is blocked inside RunScenario AND onStart has fired,
	// so the registry has recorded the run dir.
	runDir := waitRunDir(t, svc, id)

	// Cancel: ctx is cancelled (unblocks RunScenario), KillContainer called,
	// and the cancelled marker written to the now-known run dir.
	ok, err := svc.Cancel(id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !ok {
		t.Fatal("Cancel returned false")
	}

	if f.killCallCount() == 0 {
		t.Error("KillContainer was not called")
	}

	marker := filepath.Join(runDir, artifacts.CancelledFile)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("cancelled marker not written: %v", err)
	}

	waitActiveEmpty(t, svc)
}

func TestCancelIdempotent(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	id, _ := svc.Enqueue("known", "m", "e")
	// First cancel may return true or false depending on timing; second must
	// return false.
	_, _ = svc.Cancel(id)
	ok, err := svc.Cancel(id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ok {
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
	ok, err := svc.Cancel(found)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !ok {
		t.Fatal("Cancel of reattached run returned false")
	}

	// Marker must exist.
	marker := filepath.Join(orphan, artifacts.CancelledFile)
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

// nextEvent drains one Event from ch with a 5 s timeout.
func nextEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("subscriber channel closed unexpectedly")
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Event")
		return Event{}
	}
}

func TestSubscribeQueuedRunningDone(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	ch, unsub := svc.Subscribe()
	defer unsub()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	phases := []string{"queued", "running", "done"}
	for _, want := range phases {
		ev := nextEvent(t, ch)
		if ev.RunID != id {
			t.Errorf("phase %q: got RunID %q, want %q", want, ev.RunID, id)
		}
		if ev.ScenarioID != "known" {
			t.Errorf("phase %q: got ScenarioID %q, want %q", want, ev.ScenarioID, "known")
		}
		if ev.Phase != want {
			t.Errorf("got phase %q, want %q", ev.Phase, want)
		}
	}
}

func TestSubscribeCancelledEvent(t *testing.T) {
	f := newFakeRunner()
	f.block = true
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	ch, unsub := svc.Subscribe()
	defer unsub()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Drain queued + running before cancelling.
	ev := nextEvent(t, ch)
	if ev.Phase != "queued" {
		t.Fatalf("expected queued, got %q", ev.Phase)
	}
	waitActivePhase(t, svc, "running")
	ev = nextEvent(t, ch)
	if ev.Phase != "running" {
		t.Fatalf("expected running, got %q", ev.Phase)
	}

	ok, err := svc.Cancel(id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !ok {
		t.Fatal("Cancel returned false")
	}

	ev = nextEvent(t, ch)
	if ev.RunID != id {
		t.Errorf("cancelled event: got RunID %q, want %q", ev.RunID, id)
	}
	if ev.Phase != "cancelled" {
		t.Errorf("got phase %q, want cancelled", ev.Phase)
	}
}

func TestCancelReportsKillFailure(t *testing.T) {
	f := newFakeRunner()
	f.block = true
	f.failKill = true
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	id, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	waitRunDir(t, svc, id)

	ok, err := svc.Cancel(id)
	if !ok {
		t.Fatal("Cancel returned false")
	}
	if err == nil {
		t.Fatal("expected kill error")
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	f := newFakeRunner()
	svc := NewService(f, t.TempDir(), 1)
	svc.Start()

	ch, unsub := svc.Subscribe()

	// Unsubscribe before any enqueue.
	unsub()
	// Idempotent: second call must not panic.
	unsub()

	// Channel must be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel, got a value")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after unsubscribe")
	}

	// Enqueue after unsubscribe; no event should arrive on the closed channel.
	_, err := svc.Enqueue("known", "m", "e")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	waitActiveEmpty(t, svc)
	// If we get here without a panic or send-on-closed, unsubscribe is safe.
}

func TestReattachSkipsCompleted(t *testing.T) {
	runsBase := t.TempDir()

	// A dir with judge.txt should NOT be reattached.
	done := filepath.Join(runsBase, "run.done")
	if err := os.MkdirAll(done, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(done, artifacts.JudgeLogFile), []byte("pass"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A dir with cancelled should NOT be reattached.
	cancelled := filepath.Join(runsBase, "run.cancelled")
	if err := os.MkdirAll(cancelled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cancelled, artifacts.CancelledFile), []byte{}, 0o644); err != nil {
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
