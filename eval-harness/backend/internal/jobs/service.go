package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

type Runner interface {
	RunScenario(ctx context.Context, s scenarios.Scenario, model, effort string, detached bool, onStart func(runDir string)) (string, error)
	Resolve(id string) (scenarios.Scenario, bool)
	ContainerName(runDir string) string
	KillContainer(container string) error
}

// ActiveRun is a snapshot of one live run.
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
	container  string
	runDir     string
	phase      string
	ctx        context.Context
	cancel     context.CancelFunc
}

type job struct {
	id         string
	scenarioID string
	model      string
	effort     string
}

// Event is emitted at every phase transition and consumed by SSE subscribers.
type Event struct {
	RunID      string `json:"runId"`
	ScenarioID string `json:"scenarioId"`
	Phase      string `json:"phase"`
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
	subs     map[int]chan Event
	nextSub  int
}

// NewService creates a Service. Call Start() to launch workers and reattach.
func NewService(r Runner, runsBase string, workers int) *Service {
	return &Service{
		r:        r,
		runsBase: runsBase,
		workers:  workers,
		queue:    make(chan job, workers*4),
		live:     make(map[string]*liveRun),
		subs:     make(map[int]chan Event),
	}
}

// Subscribe returns a buffered phase-event channel and an idempotent unsubscribe.
func (s *Service) Subscribe() (<-chan Event, func()) {
	s.mu.Lock()
	id := s.nextSub
	s.nextSub++
	ch := make(chan Event, 16)
	s.subs[id] = ch
	s.mu.Unlock()

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.subs, id)
			s.mu.Unlock()
			close(ch)
		})
	}
	return ch, unsub
}

// Must be called with s.mu held.
func (s *Service) publish(ev Event) {
	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
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
	s.publish(Event{RunID: id, ScenarioID: scenarioID, Phase: "queued"})
	s.mu.Unlock()

	s.queue <- job{id: id, scenarioID: scenarioID, model: model, effort: effort}
	return id, nil
}

// Cancel stops a live run and writes a cancelled marker.
func (s *Service) Cancel(runID string) (bool, error) {
	s.mu.Lock()
	lr, ok := s.live[runID]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	scenarioID := lr.scenarioID
	delete(s.live, runID)
	runDir := lr.runDir
	container := lr.container
	s.publish(Event{RunID: runID, ScenarioID: scenarioID, Phase: "cancelled"})
	s.mu.Unlock()

	lr.cancel()

	var errs []error
	if container == "" && runDir != "" {
		container = s.r.ContainerName(runDir)
	}
	if container != "" {
		if err := s.r.KillContainer(container); err != nil {
			errs = append(errs, err)
		}
	}

	if runDir != "" {
		marker := filepath.Join(runDir, artifacts.CancelledFile)
		if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
			errs = append(errs, err)
		}
	}

	return true, errors.Join(errs...)
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
	s.mu.Lock()
	lr, ok := s.live[j.id]
	if !ok {
		s.mu.Unlock()
		return
	}
	if err := lr.ctx.Err(); err != nil {
		s.mu.Unlock()
		return
	}
	lr.phase = "running"
	s.publish(Event{RunID: j.id, ScenarioID: j.scenarioID, Phase: "running"})
	s.mu.Unlock()

	sc, ok := s.r.Resolve(j.scenarioID)
	if !ok {
		s.setPhaseAndDeregister(j.id, "error")
		return
	}

	onStart := func(runDir string) {
		container := s.r.ContainerName(runDir)
		s.mu.Lock()
		if lr2, still := s.live[j.id]; still {
			lr2.runDir = runDir
			lr2.container = container
		}
		s.mu.Unlock()
	}

	_, runErr := s.r.RunScenario(lr.ctx, sc, j.model, j.effort, false, onStart)

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
		scenarioID := lr.scenarioID
		lr.phase = phase
		delete(s.live, id)
		s.publish(Event{RunID: id, ScenarioID: scenarioID, Phase: phase})
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
		if fileExists(filepath.Join(full, artifacts.JudgeLogFile)) || fileExists(filepath.Join(full, artifacts.CancelledFile)) {
			continue
		}
		container := s.r.ContainerName(full)
		id := "reattach." + name
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // reattach entries have no live goroutine; ctx is pre-cancelled
		s.mu.Lock()
		s.live[id] = &liveRun{
			id:        id,
			runDir:    full,
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
