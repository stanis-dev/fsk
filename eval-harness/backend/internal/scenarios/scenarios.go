package scenarios

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
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

func Validate(raw []byte) string {
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "config must be an object"
	}
	m, ok := obj.(map[string]any)
	if !ok {
		return "config must be an object"
	}
	if _, ok := m["id"].(string); !ok {
		return "id must be a string"
	}
	if _, ok := m["title"].(string); !ok {
		return "title must be a string"
	}
	if _, ok := m["traps"].([]any); !ok {
		return "traps must be an array"
	}
	judgeAny, exists := m["judge"]
	if !exists {
		return "judge must be an object"
	}
	judge, ok := judgeAny.(map[string]any)
	if !ok {
		return "judge must be an object"
	}
	checksAny, exists := judge["checks"]
	if !exists {
		return "judge.checks must be an object"
	}
	checks, ok := checksAny.(map[string]any)
	if !ok {
		return "judge.checks must be an object"
	}
	expsAny, exists := judge["expectations"]
	if !exists {
		return "judge.expectations must be an array"
	}
	exps, ok := expsAny.([]any)
	if !ok {
		return "judge.expectations must be an array"
	}
	if !isExpectationArray(exps) {
		return "judge.expectations must be an array of {id, expectation}"
	}
	if !hasNonEmptyChecks(checks) && len(exps) == 0 {
		return "judge must have at least one non-empty checks field or a non-empty expectations array"
	}
	return ""
}

func isExpectationArray(v []any) bool {
	for _, x := range v {
		m, ok := x.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := m["id"].(string); !ok {
			return false
		}
		if _, ok := m["expectation"].(string); !ok {
			return false
		}
	}
	return true
}

func hasNonEmptyChecks(checks map[string]any) bool {
	if v, ok := checks["groundedBeforeWrite"].(bool); ok && v {
		return true
	}
	if arr, ok := checks["toolsCalled"].([]any); ok && len(arr) > 0 {
		return true
	}
	if arr, ok := checks["docsFetched"].([]any); ok && len(arr) > 0 {
		return true
	}
	if _, ok := checks["maxMcpErrors"].(float64); ok {
		return true
	}
	return false
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
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if msg := Validate(raw); msg != "" {
		return errors.New(msg)
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
