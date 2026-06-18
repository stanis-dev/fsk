package main

import (
	"encoding/json"
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

// Outcome is the result of running all three checks against one scenario's
// pristine fixture.
type Outcome struct {
	Build StepResult
	Test  StepResult
	Judge StepResult
}

// baselineSpec is the build/tests/judge expectation a scenario declares in its
// scenario.json baseline block, expressed as the judge's own verdict words.
type baselineSpec struct {
	Build string `json:"build"`
	Tests string `json:"tests"`
	Judge string `json:"judge"`
}

// canonicalBaseline is the invariant every pristine scenario seed must hold: it
// builds and tests green, yet the deterministic judge reports NON-COMPLIANT
// because no integration work has been done.
var canonicalBaseline = baselineSpec{Build: "PASS", Tests: "PASS", Judge: "NON-COMPLIANT"}

// scenario locates one entry in the scenario library.
type scenario struct {
	id               string
	dir              string
	fixtureDir       string
	scenarioJSON     string
	declaredBaseline baselineSpec
}

// observedBaseline expresses an Outcome in the same verdict words a scenario.json
// baseline block uses, so the two compare directly.
func observedBaseline(o Outcome) baselineSpec {
	return baselineSpec{Build: passFail(o.Build.OK), Tests: passFail(o.Test.OK), Judge: verdict(o.Judge.OK)}
}

// baselineHolds reports whether a scenario meets the baseline invariant: the
// observed build/tests/judge match the canonical baseline, and the scenario's
// own declared baseline matches it too. The second clause keeps the scenario.json
// baseline block honest, so a future scenario that declares a different bar fails
// loudly here instead of being graded against the wrong expectation.
func baselineHolds(s scenario, o Outcome) bool {
	return observedBaseline(o) == canonicalBaseline && s.declaredBaseline == canonicalBaseline
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// verdict translates the judge's exit status into its reported verdict: a
// non-zero exit means NON-COMPLIANT, which is the expected baseline state.
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
		declared, err := readBaseline(scenarioJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, scenario{
			id:               e.Name(),
			dir:              dir,
			fixtureDir:       fixture,
			scenarioJSON:     scenarioJSON,
			declaredBaseline: declared,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no runnable scenarios found under %s", scenariosDir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

// readBaseline parses the baseline block a scenario.json declares. A missing
// block yields the zero value, which baselineHolds then flags as a violation.
func readBaseline(path string) (baselineSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return baselineSpec{}, fmt.Errorf("reading scenario: %w", err)
	}
	var s struct {
		Baseline baselineSpec `json:"baseline"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return baselineSpec{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return s.Baseline, nil
}

// copyDir recursively copies the tree at src into dst, preserving file modes.
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
