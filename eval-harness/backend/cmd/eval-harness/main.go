// Command eval-harness runs scenarios through the eval pipeline.
//
// Usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/orchestrator"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "usage: eval-harness run [-root dir] [-model m] [-effort e] [ids...]")
		os.Exit(2)
	}
	os.Exit(cmdRun(os.Args[2:]))
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
