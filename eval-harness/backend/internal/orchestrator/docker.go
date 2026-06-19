package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"backend/internal/artifacts"
)

// agent runs the coder against a prepared work dir. The real implementation
// drives Docker; tests inject a fake.
type agent interface {
	build(ctx context.Context) error
	run(ctx context.Context, rd runDir, task string, cfg runConfig) error
}

// containerName derives a deterministic, per-run container name so a run can be
// cancelled with `docker kill` even though it was spawned detached.
func containerName(runPath string) string {
	return "fiskaly-eval-" + filepath.Base(runPath)
}

// ContainerName is the deterministic container name for a run dir.
func ContainerName(runDir string) string { return containerName(runDir) }

// KillContainer stops a running coder container by its deterministic name.
func KillContainer(container, dockerCtx string) error {
	cmd := exec.Command("docker", "kill", container)
	cmd.Env = dockerEnv(dockerCtx)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker kill %s: %w\n%s", container, err, out)
	}
	return nil
}

// dockerAgent runs the coder hermetically: only the work dir is mounted, so the
// container cannot reach the repo, the MCP/judge source, or research/.
type dockerAgent struct {
	repoRoot       string
	dockerfilePath string
	context        string
	image          string
}

func (a dockerAgent) build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "build",
		"-f", a.dockerfilePath,
		"-t", a.image, a.repoRoot)
	cmd.Env = dockerEnv(a.context)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, out)
	}
	return nil
}

func (a dockerAgent) run(ctx context.Context, rd runDir, task string, cfg runConfig) error {
	transcript, err := os.Create(filepath.Join(rd.path, artifacts.TranscriptFile))
	if err != nil {
		return err
	}
	defer transcript.Close()
	stderr, err := os.Create(filepath.Join(rd.path, artifacts.CoderErrFile))
	if err != nil {
		return err
	}
	defer stderr.Close()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--name", containerName(rd.path),
		"-e", "CLAUDE_CODE_OAUTH_TOKEN="+cfg.token,
		"-e", "IS_SANDBOX=1",
		"-e", "RUN_MODEL="+cfg.model,
		"-e", "RUN_EFFORT="+cfg.effort,
		"-e", "FISKALY_MCP_TELEMETRY=/work/mcp-telemetry.jsonl",
		"-v", rd.work+":/work",
		a.image, task)
	cmd.Env = dockerEnv(a.context)
	cmd.Stdout = transcript
	cmd.Stderr = stderr
	// The agent exiting non-zero is recorded in claude.err, not fatal: an agent
	// failure is a result to observe, matching the Bash harness.
	_ = cmd.Run()

	tele := filepath.Join(rd.work, "mcp-telemetry.jsonl")
	if _, err := os.Stat(tele); err == nil {
		if err := os.Rename(tele, filepath.Join(rd.path, artifacts.TelemetryFile)); err != nil {
			return fmt.Errorf("moving telemetry: %w", err)
		}
	}
	return nil
}

func dockerEnv(context string) []string {
	return append(os.Environ(), "DOCKER_CONTEXT="+context)
}

// dockerReachable verifies the Docker daemon is up on the given context, so a
// run fails early with a clear message instead of deep inside the first build.
func dockerReachable(context string) error {
	cmd := exec.Command("docker", "info")
	cmd.Env = dockerEnv(context)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker daemon not reachable on context %q: %w\n%s", context, err, out)
	}
	return nil
}

func checkBinaries(names ...string) error {
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			return fmt.Errorf("%s not found on PATH", n)
		}
	}
	return nil
}
