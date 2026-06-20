package scenarios

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildFixture creates a minimal valid scenario dir tree under root.
// Returns root.
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	s := filepath.Join(root, "01-demo")
	if err := os.MkdirAll(filepath.Join(s, "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"id": "01-demo", "title": "Demo",
		"traps": []any{},
		"judge": map[string]any{
			"checks":       map[string]any{"groundedBeforeWrite": true},
			"expectations": []any{map[string]any{"id": "x", "expectation": "y"}},
		},
	}
	raw, _ := json.Marshal(cfg)
	writeFile(t, filepath.Join(s, "scenario.json"), string(raw))
	writeFile(t, filepath.Join(s, "task.md"), "do the task")
	// non-numeric dir that must be ignored
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscover_NumericPrefixOnly(t *testing.T) {
	root := buildFixture(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].ID != "01-demo" {
		t.Fatalf("got %v, want [01-demo]", got)
	}
	if filepath.Base(got[0].FixtureDir) != "fixture" {
		t.Errorf("FixtureDir = %q, want .../fixture", got[0].FixtureDir)
	}
	if filepath.Base(got[0].ScenarioJSON) != "scenario.json" {
		t.Errorf("ScenarioJSON = %q, want .../scenario.json", got[0].ScenarioJSON)
	}
}

func TestDiscover_MissingFixtureOrJSON(t *testing.T) {
	root := t.TempDir()
	// numeric dir without fixture — should be ignored
	noFix := filepath.Join(root, "03-no-fixture")
	if err := os.MkdirAll(noFix, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(noFix, "scenario.json"), `{"id":"03"}`)

	// numeric dir without scenario.json — should be ignored
	noJSON := filepath.Join(root, "04-no-json")
	if err := os.MkdirAll(filepath.Join(noJSON, "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Discover(root); err == nil {
		t.Fatal("expected error when no runnable scenarios found")
	}
}

func TestDiscover_NoneIsError(t *testing.T) {
	if _, err := Discover(t.TempDir()); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestList_ParsesScenarioJSON(t *testing.T) {
	root := buildFixture(t)
	configs, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("len = %d, want 1", len(configs))
	}
	if configs[0].ID != "01-demo" || configs[0].Title != "Demo" {
		t.Errorf("config = %+v", configs[0])
	}
	if len(configs[0].Judge.Expectations) != 1 || configs[0].Judge.Expectations[0].ID != "x" {
		t.Errorf("expectations = %+v", configs[0].Judge.Expectations)
	}
}

func TestList_ErrorOnMalformedJSON(t *testing.T) {
	root := t.TempDir()
	s := filepath.Join(root, "02-broken")
	if err := os.MkdirAll(filepath.Join(s, "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(s, "scenario.json"), "{ not json")
	if _, err := List(root); err == nil {
		t.Fatal("expected error on malformed scenario.json")
	}
}

func TestLoad(t *testing.T) {
	root := buildFixture(t)
	cfg, task, err := Load(root, "01-demo")
	if err != nil {
		t.Fatalf("Load(01-demo): %v", err)
	}
	if cfg.Title != "Demo" {
		t.Errorf("Title = %q, want Demo", cfg.Title)
	}
	if task != "do the task" {
		t.Errorf("task = %q, want 'do the task'", task)
	}

	_, _, err = Load(root, "99-nope")
	if !errors.Is(err, ErrScenarioNotFound) {
		t.Fatalf("Load(99-nope) error = %v, want ErrScenarioNotFound", err)
	}
}

func TestValidate_Accept(t *testing.T) {
	good := func(judge map[string]any) []byte {
		raw, _ := json.Marshal(map[string]any{
			"id": "01-demo", "title": "Demo", "traps": []any{},
			"judge": judge,
		})
		return raw
	}

	// good config
	if msg := Validate(good(map[string]any{
		"checks":       map[string]any{"groundedBeforeWrite": true},
		"expectations": []any{map[string]any{"id": "x", "expectation": "y"}},
	})); msg != "" {
		t.Errorf("full good: %q", msg)
	}

	// only checks, no expectations
	if msg := Validate(good(map[string]any{
		"checks":       map[string]any{"groundedBeforeWrite": true},
		"expectations": []any{},
	})); msg != "" {
		t.Errorf("only-checks: %q", msg)
	}

	// only expectations, empty checks
	if msg := Validate(good(map[string]any{
		"checks":       map[string]any{},
		"expectations": []any{map[string]any{"id": "x", "expectation": "y"}},
	})); msg != "" {
		t.Errorf("only-expectations: %q", msg)
	}
}

func TestValidate_Reject(t *testing.T) {
	base := map[string]any{
		"id": "01-demo", "title": "Demo", "traps": []any{},
		"judge": map[string]any{
			"checks":       map[string]any{"groundedBeforeWrite": true},
			"expectations": []any{map[string]any{"id": "x", "expectation": "y"}},
		},
	}
	marshal := func(m map[string]any) []byte { b, _ := json.Marshal(m); return b }
	copyMap := func(m map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			out[k] = v
		}
		return out
	}

	// null / non-object
	if msg := Validate([]byte("null")); msg == "" {
		t.Error("null: expected error")
	}

	// bad title
	m := copyMap(base)
	m["title"] = 1
	if msg := Validate(marshal(m)); msg == "" || !strings.Contains(msg, "title") {
		t.Errorf("bad title: %q", msg)
	}

	// bad traps
	m = copyMap(base)
	m["traps"] = "none"
	if msg := Validate(marshal(m)); msg == "" || !strings.Contains(msg, "traps") {
		t.Errorf("bad traps: %q", msg)
	}

	// bad judge (not object)
	m = copyMap(base)
	m["judge"] = map[string]any{}
	if msg := Validate(marshal(m)); msg == "" || !strings.Contains(msg, "judge") {
		t.Errorf("bad judge (missing checks): %q", msg)
	}

	// non-array expectations
	m = copyMap(base)
	m["judge"] = map[string]any{
		"checks":       map[string]any{"groundedBeforeWrite": true},
		"expectations": "not-array",
	}
	if msg := Validate(marshal(m)); msg == "" || !strings.Contains(msg, "expectations") {
		t.Errorf("non-array expectations: %q", msg)
	}

	// empty checks + empty expectations
	m = copyMap(base)
	m["judge"] = map[string]any{
		"checks":       map[string]any{},
		"expectations": []any{},
	}
	if msg := Validate(marshal(m)); msg == "" || !strings.Contains(msg, "judge") {
		t.Errorf("empty checks+expectations: %q", msg)
	}
}

func TestAssignExpectationIds(t *testing.T) {
	input := Config{
		ID: "01-demo", Title: "Demo",
		Traps: []any{},
		Judge: JudgeSpec{
			Expectations: []Expectation{
				{ID: "e1", Expectation: "a"},
				{ID: "", Expectation: "b"},
				{ID: "kept", Expectation: "c"},
				{ID: "", Expectation: "d"},
			},
		},
	}

	out := AssignExpectationIds(input)
	ids := make([]string, len(out.Judge.Expectations))
	for i, e := range out.Judge.Expectations {
		ids[i] = e.ID
	}

	if ids[0] != "e1" {
		t.Errorf("ids[0] = %q, want e1", ids[0])
	}
	if ids[2] != "kept" {
		t.Errorf("ids[2] = %q, want kept", ids[2])
	}
	if ids[1] == "e1" {
		t.Errorf("ids[1] = e1, must skip already-used e1")
	}

	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			t.Error("empty id in output")
		}
		if seen[id] {
			t.Errorf("duplicate id %q", id)
		}
		seen[id] = true
	}

	// input not mutated
	if input.Judge.Expectations[1].ID != "" {
		t.Error("input was mutated")
	}
}

func TestSave_ValidRoundtrip(t *testing.T) {
	root := buildFixture(t)
	cfg := Config{
		ID: "01-demo", Title: "Updated",
		Traps: []any{},
		Judge: JudgeSpec{
			Checks:       json.RawMessage(`{"groundedBeforeWrite":true}`),
			Expectations: []Expectation{{ID: "", Expectation: "must do the thing"}},
		},
	}
	if err := Save(root, "01-demo", cfg, "updated task\n"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// both files must be present and reloadable
	loaded, task, err := Load(root, "01-demo")
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.Title != "Updated" {
		t.Errorf("Title = %q, want Updated", loaded.Title)
	}
	if task != "updated task\n" {
		t.Errorf("task = %q, want 'updated task\\n'", task)
	}
	// expectation id must have been assigned
	if len(loaded.Judge.Expectations) != 1 || loaded.Judge.Expectations[0].ID == "" {
		t.Errorf("expectation id not assigned: %+v", loaded.Judge.Expectations)
	}
	// scenario.json must end with a newline
	raw, err := os.ReadFile(filepath.Join(root, "01-demo", "scenario.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Error("scenario.json does not end with newline")
	}
}

func TestSave_IDMismatch(t *testing.T) {
	root := buildFixture(t)
	cfg := Config{
		ID: "99-wrong", Title: "X",
		Traps: []any{},
		Judge: JudgeSpec{
			Checks:       json.RawMessage(`{"groundedBeforeWrite":true}`),
			Expectations: []Expectation{},
		},
	}
	if err := Save(root, "01-demo", cfg, ""); err == nil {
		t.Fatal("expected error for id mismatch")
	}
}

func TestSave_UnknownID(t *testing.T) {
	root := buildFixture(t)
	cfg := Config{ID: "99-nope", Title: "X", Traps: []any{},
		Judge: JudgeSpec{Checks: json.RawMessage(`{"groundedBeforeWrite":true}`), Expectations: []Expectation{}}}
	if err := Save(root, "99-nope", cfg, ""); err == nil {
		t.Fatal("expected error for unknown scenario id")
	}
}

func TestSave_InvalidConfig_FilesNotWritten(t *testing.T) {
	root := buildFixture(t)
	// Read original scenario.json content before Save attempt.
	origPath := filepath.Join(root, "01-demo", "scenario.json")
	origBytes, err := os.ReadFile(origPath)
	if err != nil {
		t.Fatal(err)
	}
	// Empty checks + empty expectations fails Validate.
	cfg := Config{
		ID: "01-demo", Title: "Demo",
		Traps: []any{},
		Judge: JudgeSpec{
			Checks:       json.RawMessage(`{}`),
			Expectations: []Expectation{},
		},
	}
	if err := Save(root, "01-demo", cfg, ""); err == nil {
		t.Fatal("expected validation error for empty checks+expectations")
	}
	// File must be unchanged.
	afterBytes, err := os.ReadFile(origPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterBytes) != string(origBytes) {
		t.Error("scenario.json was modified despite validation failure")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
