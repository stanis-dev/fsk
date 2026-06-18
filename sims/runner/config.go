package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultModel  = "claude-sonnet-4-6"
	defaultEffort = "medium"
)

type runConfig struct {
	model  string
	effort string
	token  string
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

func loadConfig(repoRoot, model, effort string) (runConfig, error) {
	token, err := readEnvToken(filepath.Join(repoRoot, ".env"))
	if err != nil {
		return runConfig{}, err
	}
	return runConfig{model: model, effort: effort, token: token}, nil
}

// dockerContext pins to Docker Desktop unless overridden, so a run cannot land
// on another configured engine.
func dockerContext() string {
	if c := os.Getenv("DOCKER_CONTEXT"); c != "" {
		return c
	}
	return "desktop-linux"
}
