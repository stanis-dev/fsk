package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"backend/internal/scenarios"
)

// Runner is the interface the orchestrator adapter must satisfy.
// KillContainer is dockerctx-bound by the adapter; ContainerName wraps the
// orchestrator package func.
type Runner interface {
	RunScenario(ctx context.Context, s scenarios.Scenario, detached bool) (string, error)
	Resolve(id string) (scenarios.Scenario, bool)
	ContainerName(runDir string) string
	KillContainer(container string) error
}

// ActiveRun is a snapshot of one live run, exported for SSE and tests.
type ActiveRun struct {
	ID         string
	ScenarioID string
	Container  string
	Phase      string
}

// liveRun tracks a single in-flight or reattached run.
type liveRun struct {
	id         string
	scenarioID string
	container  string             // set once RunScenario returns a runDir
	runDir     string             // base name of the run dir, set once known
	phase      string             // "queued" | "running" | "done" | "error"
	ctx        context.Context    // cancelled by cancel
	cancel     context.CancelFunc // call to abort the run
}

type job struct {
	id         string
	scenarioID string
}

// Service is a bounded worker pool with a live-run registry.
type Service struct {
	r        Runner
	runsBase string
	workers  int
	queue    chan job
	mu       sync.Mutex
	live     map[string]*liveRun
	counter  atomic.Int64
}

// NewService creates a Service. Call Start() to launch workers and reattach.
func NewService(r Runner, runsBase string, workers int) *Service {
	return &Service{
		r:        r,
		runsBase: runsBase,
		workers:  workers,
		queue:    make(chan job, workers*4),
		live:     make(map[string]*liveRun),
	}
}

// Start launches workers goroutines and registers any in-flight dirs from a
// previous process.
func (s *Service) Start() {
	s.reattach()
	for i := 0; i < s.workers; i++ {
		go s.work()
	}
}

// Enqueue validates the scenario, registers a queued liveRun, and pushes the
// job. Returns an opaque run id, or an error if the scenario is unknown.
func (s *Service) Enqueue(scenarioID, model, effort string) (string, error) {
	if _, ok := s.r.Resolve(scenarioID); !ok {
		return "", fmt.Errorf("unknown scenario %q", scenarioID)
	}

	n := s.counter.Add(1)
	id := fmt.Sprintf("job.%d", n)

	ctx, cancel := context.WithCancel(context.Background())

	lr := &liveRun{
		id:         id,
		scenarioID: scenarioID,
		phase:      "queued",
		ctx:        ctx,
		cancel:     cancel,
	}

	s.mu.Lock()
	s.live[id] = lr
	s.mu.Unlock()

	s.queue <- job{id: id, scenarioID: scenarioID}
	return id, nil
}

// Cancel stops a live run and writes a cancelled marker. Returns false if the
// run is not found. Idempotent: a second call returns false.
func (s *Service) Cancel(runID string) bool {
	s.mu.Lock()
	lr, ok := s.live[runID]
	if !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.live, runID)
	s.mu.Unlock()

	lr.cancel()

	// Derive container name if we have a run dir but not yet the container field.
	container := lr.container
	if container == "" && lr.runDir != "" {
		container = s.r.ContainerName(filepath.Join(s.runsBase, lr.runDir))
	}
	if container != "" {
		_ = s.r.KillContainer(container)
	}

	if lr.runDir != "" {
		marker := filepath.Join(s.runsBase, lr.runDir, "cancelled")
		_ = os.WriteFile(marker, []byte{}, 0o644)
	}

	return true
}

// Active returns a point-in-time snapshot of all live runs.
func (s *Service) Active() []ActiveRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ActiveRun, 0, len(s.live))
	for _, lr := range s.live {
		out = append(out, ActiveRun{
			ID:         lr.id,
			ScenarioID: lr.scenarioID,
			Container:  lr.container,
			Phase:      lr.phase,
		})
	}
	return out
}

// work is the per-worker goroutine; ranges over the queue until it is closed.
func (s *Service) work() {
	for j := range s.queue {
		s.runJob(j)
	}
}

func (s *Service) runJob(j job) {
	// If the run was cancelled before the worker picked it up, skip it.
	s.mu.Lock()
	lr, ok := s.live[j.id]
	if !ok {
		s.mu.Unlock()
		return
	}
	// Check if already cancelled (ctx done before worker started).
	if err := lr.ctx.Err(); err != nil {
		s.mu.Unlock()
		return
	}
	lr.phase = "running"
	s.mu.Unlock()

	sc, ok := s.r.Resolve(j.scenarioID)
	if !ok {
		s.setPhaseAndDeregister(j.id, "error")
		return
	}

	runDir, runErr := s.r.RunScenario(lr.ctx, sc, false)

	// Reconcile: once RunScenario returns we know the real run dir. Update the
	// registry so a concurrent Cancel can write the marker and name the container.
	if runDir != "" {
		base := filepath.Base(runDir)
		container := s.r.ContainerName(runDir)
		s.mu.Lock()
		if lr2, still := s.live[j.id]; still {
			lr2.runDir = base
			lr2.container = container
		}
		s.mu.Unlock()
	}

	if runErr != nil {
		s.setPhaseAndDeregister(j.id, "error")
		return
	}
	s.setPhaseAndDeregister(j.id, "done")
}

func (s *Service) setPhaseAndDeregister(id, phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lr, ok := s.live[id]; ok {
		lr.phase = phase
		delete(s.live, id)
	}
}

// reattach scans runsBase for run.* dirs with neither judge.txt nor cancelled
// and registers each as a liveRun with a no-op cancel so Cancel can still
// docker-kill the orphan container and mark it.
func (s *Service) reattach() {
	entries, err := os.ReadDir(s.runsBase)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "run.") {
			continue
		}
		name := e.Name()
		full := filepath.Join(s.runsBase, name)
		if fileExists(filepath.Join(full, "judge.txt")) || fileExists(filepath.Join(full, "cancelled")) {
			continue
		}
		container := s.r.ContainerName(full)
		id := "reattach." + name
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // reattach entries have no live goroutine; ctx is pre-cancelled
		s.mu.Lock()
		s.live[id] = &liveRun{
			id:        id,
			runDir:    name,
			container: container,
			phase:     "running",
			ctx:       ctx,
			cancel:    func() {},
		}
		s.mu.Unlock()
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
