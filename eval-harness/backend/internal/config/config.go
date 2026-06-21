// Package config resolves the eval-harness process configuration from the
// environment, filesystem, and built-in defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	addr       = "127.0.0.1:8090"
	corsOrigin = "http://localhost:8080"
	image      = "fiskaly-eval"
	workers    = 1

	defaultModel  = "claude-sonnet-4-6"
	defaultEffort = "medium"

	scenariosSubdir   = "backend/scenarios"
	dockerfileSubpath = "backend/sandbox/Dockerfile"
)

// JudgeModel and JudgeEffort drive the LLM expectation layer in the judge.
const (
	JudgeModel  = "claude-opus-4-8"
	JudgeEffort = "high"
)

// Config is the fully-resolved process configuration.
type Config struct {
	Addr           string
	CORSOrigin     string
	Workers        int
	Image          string
	Model          string
	Effort         string
	Token          string
	DockerContext  string
	RepoRoot       string
	ScenariosDir   string
	DockerfilePath string
	RunsDir        string
}

// Load resolves the eval-harness root, repo root, OAuth token, runs directory,
// and Docker context, applying built-in defaults for the rest.
func Load() (Config, error) {
	root, err := resolveRoot()
	if err != nil {
		return Config{}, err
	}
	repoRoot := filepath.Dir(root)

	token, err := readEnvToken(filepath.Join(repoRoot, ".env"))
	if err != nil {
		return Config{}, err
	}

	runsDir, err := resolveRunsDir()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Addr:           addr,
		CORSOrigin:     corsOrigin,
		Workers:        workers,
		Image:          image,
		Model:          defaultModel,
		Effort:         defaultEffort,
		Token:          token,
		DockerContext:  resolveDockerContext(),
		RepoRoot:       repoRoot,
		ScenariosDir:   filepath.Join(root, scenariosSubdir),
		DockerfilePath: filepath.Join(root, dockerfileSubpath),
		RunsDir:        runsDir,
	}, nil
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

// readEnvToken extracts CLAUDE_CODE_OAUTH_TOKEN from a .env file, stripping
// optional surrounding quotes.
func readEnvToken(envPath string) (string, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", envPath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "CLAUDE_CODE_OAUTH_TOKEN=")
		if !ok {
			continue
		}
		if v = strings.Trim(v, `"'`); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("CLAUDE_CODE_OAUTH_TOKEN not found in %s", envPath)
}

// resolveDockerContext pins to Docker Desktop unless overridden, so a run cannot
// land on another configured engine.
func resolveDockerContext() string {
	if c := os.Getenv("DOCKER_CONTEXT"); c != "" {
		return c
	}
	return "desktop-linux"
}
