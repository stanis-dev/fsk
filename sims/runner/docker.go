package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// agent runs the coder against a prepared work dir. The real implementation
// drives Docker; tests inject a fake.
type agent interface {
	run(rd runDir, task string, cfg runConfig) error
}

// dockerAgent runs the coder hermetically: only the work dir is mounted, so the
// container cannot reach the repo, the MCP/judge source, or research/.
type dockerAgent struct {
	repoRoot string
	simsRoot string
	context  string
	image    string
}

func (a dockerAgent) run(rd runDir, task string, cfg runConfig) error {
	build := exec.Command("docker", "build",
		"-f", filepath.Join(a.simsRoot, "evals", "Dockerfile"),
		"-t", a.image, a.repoRoot)
	build.Env = dockerEnv(a.context)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, out)
	}

	transcript, err := os.Create(filepath.Join(rd.path, "transcript.jsonl"))
	if err != nil {
		return err
	}
	defer transcript.Close()
	stderr, err := os.Create(filepath.Join(rd.path, "claude.err"))
	if err != nil {
		return err
	}
	defer stderr.Close()

	run := exec.Command("docker", "run", "--rm",
		"-e", "CLAUDE_CODE_OAUTH_TOKEN="+cfg.token,
		"-e", "IS_SANDBOX=1",
		"-e", "RUN_MODEL="+cfg.model,
		"-e", "RUN_EFFORT="+cfg.effort,
		"-e", "FISKALY_MCP_TELEMETRY=/work/mcp-telemetry.jsonl",
		"-v", rd.work+":/work",
		a.image, task)
	run.Env = dockerEnv(a.context)
	run.Stdout = transcript
	run.Stderr = stderr
	// The agent exiting non-zero is recorded in claude.err, not fatal: an agent
	// failure is a result to observe, matching the Bash harness.
	_ = run.Run()

	tele := filepath.Join(rd.work, "mcp-telemetry.jsonl")
	if _, err := os.Stat(tele); err == nil {
		if err := os.Rename(tele, filepath.Join(rd.path, "mcp-telemetry.jsonl")); err != nil {
			return fmt.Errorf("moving telemetry: %w", err)
		}
	}
	return nil
}

func dockerEnv(context string) []string {
	return append(os.Environ(), "DOCKER_CONTEXT="+context)
}

// checkBinaries verifies each named tool is on PATH.
func checkBinaries(names ...string) error {
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			return fmt.Errorf("%s not found on PATH", n)
		}
	}
	return nil
}
