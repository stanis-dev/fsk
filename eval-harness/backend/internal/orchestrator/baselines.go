package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type stepResult struct {
	OK     bool
	Output string
}

type outcome struct {
	Build stepResult
	Test  stepResult
	Judge stepResult
}

type scenario struct {
	id           string
	dir          string
	fixtureDir   string
	scenarioJSON string
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
