package scenarios

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

var ErrScenarioNotFound = errors.New("scenario not found")

type Expectation struct {
	ID          string `json:"id"`
	Expectation string `json:"expectation"`
}

type JudgeSpec struct {
	Checks       json.RawMessage `json:"checks"`
	Expectations []Expectation   `json:"expectations"`
}

type Config struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Traps []any     `json:"traps"`
	Judge JudgeSpec `json:"judge"`
}

type Scenario struct {
	ID           string
	Dir          string
	FixtureDir   string
	ScenarioJSON string
}

var numericPrefix = regexp.MustCompile(`^[0-9]`)

// Discover returns every runnable scenario under scenariosDir: a
// numeric-prefixed directory that has both a fixture/ subdir and a
// scenario.json. Results are sorted ascending by ID. Errors if none found.
func Discover(scenariosDir string) ([]Scenario, error) {
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("reading scenarios dir: %w", err)
	}
	var out []Scenario
	for _, e := range entries {
		if !e.IsDir() || !numericPrefix.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(scenariosDir, e.Name())
		fixture := filepath.Join(dir, "fixture")
		scenarioJSON := filepath.Join(dir, "scenario.json")
		if !isDir(fixture) || !isFile(scenarioJSON) {
			continue
		}
		out = append(out, Scenario{
			ID:           e.Name(),
			Dir:          dir,
			FixtureDir:   fixture,
			ScenarioJSON: scenarioJSON,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no runnable scenarios found under %s", scenariosDir)
	}
	slices.SortFunc(out, func(a, b Scenario) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

// List discovers all scenarios and parses each scenario.json into a Config.
func List(scenariosDir string) ([]Config, error) {
	scenarios, err := Discover(scenariosDir)
	if err != nil {
		return nil, err
	}
	out := make([]Config, 0, len(scenarios))
	for _, s := range scenarios {
		raw, err := os.ReadFile(s.ScenarioJSON)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", s.ScenarioJSON, err)
		}
		var c Config
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", s.ScenarioJSON, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func scenarioDir(scenariosDir, id string) (string, error) {
	scenarios, err := Discover(scenariosDir)
	if err != nil {
		return "", err
	}
	for _, s := range scenarios {
		if s.ID == id {
			return s.Dir, nil
		}
	}
	return "", ErrScenarioNotFound
}

func Load(scenariosDir, id string) (*Config, string, error) {
	dir, err := scenarioDir(scenariosDir, id)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	if err != nil {
		return nil, "", fmt.Errorf("reading scenario.json: %w", err)
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, "", fmt.Errorf("parsing scenario.json: %w", err)
	}
	taskBytes, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return nil, "", fmt.Errorf("reading task.md: %w", err)
	}
	return &c, string(taskBytes), nil
}

// checksSpec mirrors the recognised judge.checks fields. It is used to validate
// the otherwise-untyped Checks payload and to tell whether any check is active.
type checksSpec struct {
	GroundedBeforeWrite bool     `json:"groundedBeforeWrite"`
	ToolsCalled         []string `json:"toolsCalled"`
	DocsFetched         []string `json:"docsFetched"`
	MaxMcpErrors        *int     `json:"maxMcpErrors"`
}

func (c checksSpec) active() bool {
	return c.GroundedBeforeWrite || len(c.ToolsCalled) > 0 || len(c.DocsFetched) > 0 || c.MaxMcpErrors != nil
}

// Validate reports why config is not a usable scenario, or "" if it is valid.
// Field types are guaranteed by the Config type, so the only rule left to enforce
// is that the judge has something to assert: a non-empty check or an expectation.
func Validate(config Config) string {
	var checks checksSpec
	if len(config.Judge.Checks) > 0 {
		if err := json.Unmarshal(config.Judge.Checks, &checks); err != nil {
			return "judge.checks must be an object"
		}
	}
	if !checks.active() && len(config.Judge.Expectations) == 0 {
		return "judge must have at least one non-empty checks field or a non-empty expectations array"
	}
	return ""
}

// AssignExpectationIds returns a new Config with empty expectation IDs filled in
// as e1, e2, ... skipping any already-used IDs. Existing non-empty IDs are
// preserved. The input Config is not mutated.
func AssignExpectationIds(c Config) Config {
	used := map[string]bool{}
	for _, e := range c.Judge.Expectations {
		if e.ID != "" {
			used[e.ID] = true
		}
	}
	n := 1
	nextID := func() string {
		for {
			id := fmt.Sprintf("e%d", n)
			n++
			if !used[id] {
				used[id] = true
				return id
			}
		}
	}
	exps := make([]Expectation, len(c.Judge.Expectations))
	for i, e := range c.Judge.Expectations {
		if e.ID != "" {
			exps[i] = e
		} else {
			exps[i] = Expectation{ID: nextID(), Expectation: e.Expectation}
		}
	}
	judge := JudgeSpec{Checks: c.Judge.Checks, Expectations: exps}
	return Config{ID: c.ID, Title: c.Title, Traps: c.Traps, Judge: judge}
}

// Save writes config and task into <scenariosDir>/<id>/scenario.json and task.md.
// It rejects unknown ids, id mismatches, and configs that fail Validate.
// AssignExpectationIds is run before validation and writing.
func Save(scenariosDir, id string, config Config, task string) error {
	dir, err := scenarioDir(scenariosDir, id)
	if errors.Is(err, ErrScenarioNotFound) {
		return fmt.Errorf("unknown scenario %q", id)
	}
	if err != nil {
		return err
	}
	if config.ID != id {
		return fmt.Errorf("config.id %q does not match path id %q", config.ID, id)
	}
	config = AssignExpectationIds(config)
	if msg := Validate(config); msg != "" {
		return errors.New(msg)
	}
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(dir, "scenario.json"), raw, 0o644); err != nil {
		return fmt.Errorf("writing scenario.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(task), 0o644); err != nil {
		return fmt.Errorf("writing task.md: %w", err)
	}
	return nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
