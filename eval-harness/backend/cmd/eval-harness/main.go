// Command eval-harness runs scenarios through the eval pipeline or serves the read-only API.
//
// Usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]
//
//	eval-harness serve [-addr host:port] [-root dir] [-cors-origin origin]
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"backend/internal/api"
	"backend/internal/orchestrator"
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
	root := fs.String("root", "", "eval-harness root (dir with scenarios/ and backend/); default: discovered from cwd")
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

	code, err := orchestrator.Run(orchestrator.Config{
		ScenariosDir:   filepath.Join(ehRoot, "scenarios"),
		JudgeDir:       filepath.Join(ehRoot, "backend", "cmd", "judge"),
		RepoRoot:       filepath.Dir(ehRoot),
		DockerfilePath: filepath.Join(ehRoot, "evals", "Dockerfile"),
		RunsBase:       filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval"),
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
// nearest ancestor of cwd containing both scenarios/ and backend/.
func resolveRoot(flagRoot string) (string, error) {
	if flagRoot != "" {
		return filepath.Abs(flagRoot)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if isDir(filepath.Join(dir, "scenarios")) && isDir(filepath.Join(dir, "backend")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate eval-harness root (a dir with scenarios/ and backend/) from %s; pass -root", wd)
		}
		dir = parent
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8090", "listen address (bind localhost; auth is out of scope)")
	root := fs.String("root", "", "eval-harness root; default: discovered from cwd")
	corsOrigin := fs.String("cors-origin", "http://localhost:8080", "allowed browser origin for the dashboard")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ehRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 2
	}
	runsDir := os.Getenv("FISKALY_RUNS_DIR")
	if runsDir == "" {
		runsDir = filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval")
	}
	h := api.Handler(api.Config{
		RunsDir:      runsDir,
		ScenariosDir: filepath.Join(ehRoot, "scenarios"),
		CORSOrigin:   *corsOrigin,
	})
	fmt.Fprintf(os.Stderr, "eval-harness: serving on http://%s (cors: %s)\n", *addr, *corsOrigin)
	if err := http.ListenAndServe(*addr, h); err != nil {
		fmt.Fprintln(os.Stderr, "eval-harness:", err)
		return 1
	}
	return 0
}
