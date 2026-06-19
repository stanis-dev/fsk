package orchestrator

import (
	"context"
	"io"

	"backend/internal/scenarios"
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
	runner, err := NewRunner(cfg)
	if err != nil {
		return 2, err
	}
	discovered, err := scenarios.Discover(cfg.ScenariosDir)
	if err != nil {
		return 2, err
	}
	if len(cfg.IDs) > 0 {
		discovered, err = filterScenarios(discovered, cfg.IDs)
		if err != nil {
			return 2, err
		}
	}
	return runAll(context.Background(), discovered, cfg.RunsBase, runner.judgeBin, runner.ag, runner.cfg, true, cfg.Out), nil
}
