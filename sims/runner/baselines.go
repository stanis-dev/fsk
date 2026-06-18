package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// StepResult is the outcome of one check (build, test, or judge): whether the
// process exited zero, plus its combined output for diagnostics.
type StepResult struct {
	OK     bool
	Output string
}

// Outcome is the result of running all three checks against one scenario's work dir.
type Outcome struct {
	Build StepResult
	Test  StepResult
	Judge StepResult
}

// scenario locates one entry in the scenario library.
type scenario struct {
	id           string
	dir          string
	fixtureDir   string
	scenarioJSON string
}

// verdict translates the judge's exit status into its reported verdict words.
func verdict(judgeOK bool) string {
	if judgeOK {
		return "conformant"
	}
	return "NON-COMPLIANT"
}

var scenarioID = regexp.MustCompile(`^[0-9]`)

// discoverScenarios returns every runnable scenario under scenariosDir: a
// numeric-prefixed directory that has both a fixture/ subdir and a scenario.json.
// Results are sorted by id. It errors if none are found.
func discoverScenarios(scenariosDir string) ([]scenario, error) {
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("reading scenarios dir: %w", err)
	}
	var out []scenario
	for _, e := range entries {
		if !e.IsDir() || !scenarioID.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(scenariosDir, e.Name())
		fixture := filepath.Join(dir, "fixture")
		scenarioJSON := filepath.Join(dir, "scenario.json")
		if !isDir(fixture) || !isFile(scenarioJSON) {
			continue
		}
		out = append(out, scenario{
			id:           e.Name(),
			dir:          dir,
			fixtureDir:   fixture,
			scenarioJSON: scenarioJSON,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no runnable scenarios found under %s", scenariosDir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
