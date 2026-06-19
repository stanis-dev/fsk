// Command runner is the testable Go entrypoint for the eval workbench.
//
// Usage: runner run [ids...]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "runner: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: runner run [ids...]")
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	model := fs.String("model", defaultModel, "coder model")
	effort := fs.String("effort", defaultEffort, "coder effort")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	simsRoot, err := findSimsRoot(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	repoRoot := filepath.Dir(simsRoot)
	ctx := dockerContext()

	if err := checkBinaries("docker", "go", "git"); err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	if err := dockerReachable(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	cfg, err := loadConfig(repoRoot, *model, *effort)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	scenarios, err := discoverScenarios(filepath.Join(simsRoot, "scenarios"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	if ids := fs.Args(); len(ids) > 0 {
		scenarios, err = filterScenarios(scenarios, ids)
		if err != nil {
			fmt.Fprintln(os.Stderr, "runner:", err)
			return 2
		}
	}

	tempDir, err := os.MkdirTemp("", "runner-judge-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}
	judgeBin, err := buildJudge(filepath.Join(simsRoot, "judge"), tempDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner:", err)
		return 2
	}

	ag := dockerAgent{repoRoot: repoRoot, simsRoot: simsRoot, context: ctx, image: "fiskaly-eval"}
	runsBase := filepath.Join(os.Getenv("HOME"), ".cache", "fiskaly-eval")
	return runAll(scenarios, runsBase, judgeBin, ag, cfg, os.Stdout)
}

// findSimsRoot locates the sims directory by walking up from start, accepting
// either being inside sims or above it.
func findSimsRoot(start string) (string, error) {
	dir := start
	for {
		if isSimsDir(dir) {
			return dir, nil
		}
		if nested := filepath.Join(dir, "eval-harness"); isSimsDir(nested) {
			return nested, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate eval-harness/ (with scenarios/ and judge/) from %s", start)
		}
		dir = parent
	}
}

func isSimsDir(d string) bool {
	return isDir(filepath.Join(d, "scenarios")) && isDir(filepath.Join(d, "judge"))
}
