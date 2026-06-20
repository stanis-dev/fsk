// Command eval-harness runs scenarios through the eval pipeline or serves the API.
//
// Usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]
//
//	eval-harness serve [-addr host:port] [-root dir] [-cors-origin origin] [-model m] [-effort e] [-workers n]
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"backend/internal/api"
	"backend/internal/jobs"
	"backend/internal/orchestrator"
	"backend/internal/scenarios"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: eval-harness <run|serve> [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "serve":
		os.Exit(cmdServe(os.Args[2:]))
	default:
		fmt.Fprintln(os.Stderr, "usage: eval-harness <run|serve> [flags]")
		os.Exit(2)
	}
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	root := fs.String("root", "", "eval-harness root (dir with backend/ and dashboard/); default: discovered from cwd")
	model := fs.String("model", orchestrator.DefaultModel, "coder model")
	effort := fs.String("effort", orchestrator.DefaultEffort, "coder effort")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ehRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}

	runsDir, err := resolveRunsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}
	code, err := orchestrator.Run(orchestrator.Config{
		ScenariosDir:   filepath.Join(ehRoot, "backend", "scenarios"),
		JudgeDir:       filepath.Join(ehRoot, "backend", "cmd", "judge"),
		RepoRoot:       filepath.Dir(ehRoot),
		DockerfilePath: filepath.Join(ehRoot, "backend", "sandbox", "Dockerfile"),
		RunsBase:       runsDir,
		Image:          "fiskaly-eval",
		Model:          *model,
		Effort:         *effort,
		IDs:            fs.Args(),
		Out:            os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
	}
	return code
}

// resolveRoot returns the eval-harness root: the -root flag if set, else the
// nearest ancestor of cwd containing both backend/ and dashboard/.
func resolveRoot(flagRoot string) (string, error) {
	if flagRoot != "" {
		return filepath.Abs(flagRoot)
	}
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
			return "", fmt.Errorf("could not locate eval-harness root (a dir with backend/ and dashboard/) from %s; pass -root", wd)
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

func (a runnerAdapter) RunScenario(ctx context.Context, s scenarios.Scenario, model, effort string, detached bool, onStart func(runDir string)) (string, error) {
	return a.r.RunScenario(ctx, s, orchestrator.RunOptions{
		Model:    model,
		Effort:   effort,
		Detached: detached,
		OnStart:  onStart,
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

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address (bind localhost; auth is out of scope)")
	root := fs.String("root", "", "eval-harness root; default: discovered from cwd")
	corsOrigin := fs.String("cors-origin", "http://localhost:8080", "allowed browser origin for the dashboard")
	model := fs.String("model", orchestrator.DefaultModel, "coder model")
	effort := fs.String("effort", orchestrator.DefaultEffort, "coder effort")
	workers := fs.Int("workers", 1, "number of concurrent scenario workers")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ehRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}
	runsDir, err := resolveRunsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "eval-harness: -workers must be at least 1")
		return 2
	}

	runner, err := orchestrator.NewRunner(orchestrator.Config{
		ScenariosDir:   filepath.Join(ehRoot, "backend", "scenarios"),
		JudgeDir:       filepath.Join(ehRoot, "backend", "cmd", "judge"),
		RepoRoot:       filepath.Dir(ehRoot),
		DockerfilePath: filepath.Join(ehRoot, "backend", "sandbox", "Dockerfile"),
		RunsBase:       runsDir,
		Image:          "fiskaly-eval",
		Model:          *model,
		Effort:         *effort,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}

	svc := jobs.NewService(runnerAdapter{runner}, runsDir, *workers)
	svc.Start()

	h := api.Handler(api.Config{
		RunsDir:      runsDir,
		ScenariosDir: filepath.Join(ehRoot, "backend", "scenarios"),
		CORSOrigin:   *corsOrigin,
		Service:      svc,
	})
	fmt.Fprintf(os.Stderr, "eval-harness: serving on http://%s (cors: %s)\n", *addr, *corsOrigin)
	if err := http.ListenAndServe(*addr, h); err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 1
	}
	return 0
}
