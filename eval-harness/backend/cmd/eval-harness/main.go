// Command eval-harness serves the eval API: it builds the Docker image and
// judge once, then runs scenarios on request and streams their progress to the
// dashboard. It takes no arguments; configuration is the constants below.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"backend/internal/api"
	"backend/internal/jobs"
	"backend/internal/orchestrator"
	"backend/internal/scenarios"
)

const (
	addr       = "127.0.0.1:8090"
	corsOrigin = "http://localhost:8080"
	model      = orchestrator.DefaultModel
	effort     = orchestrator.DefaultEffort
	workers    = 1
	image      = "fiskaly-eval"

	scenariosSubdir   = "backend/scenarios"
	dockerfileSubpath = "backend/sandbox/Dockerfile"
)

func main() {
	ehRoot, err := resolveRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(2)
	}
	runsDir, err := resolveRunsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(2)
	}

	scenariosDir := filepath.Join(ehRoot, scenariosSubdir)
	runner, err := orchestrator.NewRunner(orchestrator.Config{
		ScenariosDir:   scenariosDir,
		RepoRoot:       filepath.Dir(ehRoot),
		DockerfilePath: filepath.Join(ehRoot, dockerfileSubpath),
		RunsBase:       runsDir,
		Image:          image,
		Model:          model,
		Effort:         effort,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(2)
	}

	svc := jobs.NewService(runnerAdapter{runner}, runsDir, workers)
	svc.Start()

	h := api.Handler(api.Config{
		RunsDir:      runsDir,
		ScenariosDir: scenariosDir,
		CORSOrigin:   corsOrigin,
		Service:      svc,
	})
	fmt.Fprintf(os.Stderr, "eval-harness: serving on http://%s (cors: %s)\n", addr, corsOrigin)
	if err := http.ListenAndServe(addr, h); err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		os.Exit(1)
	}
}

// resolveRoot returns the eval-harness root: the nearest ancestor of cwd
// containing both backend/ and dashboard/.
func resolveRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if isDir(filepath.Join(dir, "backend")) && isDir(filepath.Join(dir, "dashboard")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate eval-harness root (a dir with backend/ and dashboard/) from %s", wd)
		}
		dir = parent
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func resolveRunsDir() (string, error) {
	if runsDir := os.Getenv("FISKALY_RUNS_DIR"); runsDir != "" {
		return runsDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "fiskaly-eval"), nil
}

// runnerAdapter wraps *orchestrator.Runner to satisfy jobs.Runner.
// The adapter binds the Docker context so KillContainer is a single-arg call.
type runnerAdapter struct {
	r *orchestrator.Runner
}

func (a runnerAdapter) RunScenario(ctx context.Context, s scenarios.Scenario, model, effort string, onStart func(runDir string)) (string, error) {
	return a.r.RunScenario(ctx, s, orchestrator.RunOptions{
		Model:   model,
		Effort:  effort,
		OnStart: onStart,
	})
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
