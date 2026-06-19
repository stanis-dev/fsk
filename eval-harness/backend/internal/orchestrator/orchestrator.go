package orchestrator

import (
	"io"
	"os"
)

// Config is the fully-resolved input to a run batch. The orchestrator performs
// no path discovery; every location is supplied explicitly by the caller.
type Config struct {
	ScenariosDir   string
	JudgeDir       string
	RepoRoot       string
	DockerfilePath string
	RunsBase       string
	Image          string
	Model          string
	Effort         string
	IDs            []string
	Out            io.Writer
}

// Run discovers scenarios under cfg.ScenariosDir (optionally filtered by
// cfg.IDs), builds the judge from cfg.JudgeDir, and runs each scenario through
// the Docker pipeline. It returns the batch exit code (0 all ran, 1 some
// failed) or 2 with a non-nil error on a harness-level failure before the batch.
func Run(cfg Config) (int, error) {
	ctx := dockerContext()
	if err := checkBinaries("docker", "go", "git"); err != nil {
		return 2, err
	}
	if err := dockerReachable(ctx); err != nil {
		return 2, err
	}
	rc, err := loadConfig(cfg.RepoRoot, cfg.Model, cfg.Effort)
	if err != nil {
		return 2, err
	}
	scenarios, err := discoverScenarios(cfg.ScenariosDir)
	if err != nil {
		return 2, err
	}
	if len(cfg.IDs) > 0 {
		scenarios, err = filterScenarios(scenarios, cfg.IDs)
		if err != nil {
			return 2, err
		}
	}
	tempDir, err := os.MkdirTemp("", "runner-judge-")
	if err != nil {
		return 2, err
	}
	judgeBin, err := buildJudge(cfg.JudgeDir, tempDir)
	if err != nil {
		return 2, err
	}
	ag := dockerAgent{repoRoot: cfg.RepoRoot, dockerfilePath: cfg.DockerfilePath, context: ctx, image: cfg.Image}
	return runAll(scenarios, cfg.RunsBase, judgeBin, ag, rc, cfg.Out), nil
}
