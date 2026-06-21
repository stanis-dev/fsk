// Command eval-harness serves the eval API: it builds the Docker image once,
// then runs scenarios on request and streams their progress to the dashboard.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/jobs"
	"backend/internal/orchestrator"
	"backend/internal/scenarios"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(2)
	}

	runner, err := orchestrator.NewRunner(orchestrator.Config{
		ScenariosDir:   cfg.ScenariosDir,
		RepoRoot:       cfg.RepoRoot,
		DockerfilePath: cfg.DockerfilePath,
		RunsBase:       cfg.RunsDir,
		Image:          cfg.Image,
		Model:          cfg.Model,
		Effort:         cfg.Effort,
		Token:          cfg.Token,
		DockerContext:  cfg.DockerContext,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(2)
	}

	svc := jobs.NewService(runnerAdapter{runner}, cfg.RunsDir, cfg.Workers)
	svc.Start()

	h := api.Handler(api.Config{
		RunsDir:      cfg.RunsDir,
		ScenariosDir: cfg.ScenariosDir,
		CORSOrigin:   cfg.CORSOrigin,
		Service:      svc,
	})
	fmt.Fprintf(os.Stderr, "eval-harness: serving on http://%s (cors: %s)\n", cfg.Addr, cfg.CORSOrigin)
	if err := http.ListenAndServe(cfg.Addr, h); err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(1)
	}
}

// runnerAdapter wraps *orchestrator.Runner to satisfy jobs.Runner.
// The adapter binds the Docker context so KillContainer is a single-arg call.
type runnerAdapter struct {
	r *orchestrator.Runner
}

func (a runnerAdapter) RunScenario(ctx context.Context, s scenarios.Scenario, onStart func(runDir string)) (string, error) {
	return a.r.RunScenario(ctx, s, orchestrator.RunOptions{OnStart: onStart})
}

func (a runnerAdapter) Resolve(id string) (scenarios.Scenario, bool) {
	return a.r.Resolve(id)
}

func (a runnerAdapter) ContainerName(runDir string) string {
	return orchestrator.ContainerName(runDir)
}

func (a runnerAdapter) KillContainer(container string) error {
	return orchestrator.KillContainer(container, a.r.DockerContext())
}
