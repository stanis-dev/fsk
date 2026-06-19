package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"backend/internal/scenarios"
)

// RunOptions are per-run inputs. Empty Model/Effort fall back to the Runner's defaults.
type RunOptions struct {
	Model    string
	Effort   string
	Detached bool
	// OnStart is called once, right after the run dir is created and before the
	// long coder run. The caller can use it to record the run dir while the run
	// is in flight (e.g. so a concurrent Cancel knows where to write a marker).
	OnStart func(runDir string)
}

// Runner holds a fully-initialised pipeline: toolchain verified, judge built,
// Docker image built once. Multiple scenarios can be run without rebuilding.
type Runner struct {
	defaultModel  string
	defaultEffort string
	token         string
	ag            agent
	judgeBin      string
	runsBase      string
	dockerCtx     string
	scenariosDir  string
}

// NewRunner validates the toolchain, loads config, builds the judge binary, and
// builds the Docker image. All subsequent RunScenario calls reuse the image.
func NewRunner(cfg Config) (*Runner, error) {
	ctx := dockerContext()
	if err := checkBinaries("docker", "go", "git"); err != nil {
		return nil, err
	}
	if err := dockerReachable(ctx); err != nil {
		return nil, err
	}
	rc, err := loadConfig(cfg.RepoRoot, cfg.Model, cfg.Effort)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "runner-judge-")
	if err != nil {
		return nil, err
	}
	judgeBin, err := buildJudge(cfg.JudgeDir, tempDir)
	if err != nil {
		return nil, err
	}
	ag := dockerAgent{
		repoRoot:       cfg.RepoRoot,
		dockerfilePath: cfg.DockerfilePath,
		context:        ctx,
		image:          cfg.Image,
	}
	if err := ag.build(context.Background()); err != nil {
		return nil, fmt.Errorf("building image: %w", err)
	}
	return &Runner{
		defaultModel:  rc.model,
		defaultEffort: rc.effort,
		token:         rc.token,
		ag:            ag,
		judgeBin:      judgeBin,
		runsBase:      cfg.RunsBase,
		dockerCtx:     ctx,
		scenariosDir:  cfg.ScenariosDir,
	}, nil
}

// RunScenario runs one scenario through the pipeline. The image is already
// built; this call only runs the container. ctx cancellation kills the run.
func (r *Runner) RunScenario(ctx context.Context, s scenarios.Scenario, opts RunOptions) (runDir string, err error) {
	model := opts.Model
	if model == "" {
		model = r.defaultModel
	}
	effort := opts.Effort
	if effort == "" {
		effort = r.defaultEffort
	}
	rc := runConfig{model: model, effort: effort, token: r.token}
	res, err := runScenario(ctx, s, r.runsBase, r.judgeBin, r.ag, rc, opts.Detached, opts.OnStart)
	if err != nil {
		return "", err
	}
	return res.runDir, nil
}

// Resolve discovers scenarios under r.scenariosDir and returns the one whose ID
// matches id exactly or has id as a numeric prefix (e.g. "06" matches
// "06-fire-and-forget"). ok is false if nothing matches.
func (r *Runner) Resolve(id string) (scenarios.Scenario, bool) {
	all, err := scenarios.Discover(r.scenariosDir)
	if err != nil {
		return scenarios.Scenario{}, false
	}
	for _, s := range all {
		if s.ID == id || strings.HasPrefix(s.ID, id+"-") {
			return s, true
		}
	}
	return scenarios.Scenario{}, false
}

// DockerContext returns the Docker context the Runner is pinned to.
func (r *Runner) DockerContext() string { return r.dockerCtx }
