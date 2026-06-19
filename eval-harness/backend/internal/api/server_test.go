package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// buildRunsDir creates a runs/ tree under root with one run.sample directory.
// It writes the artifact files that SummarizeRun and LoadRun expect.
func buildRunsDir(t *testing.T, root string) string {
	t.Helper()
	runsDir := filepath.Join(root, "runs")
	runDir := filepath.Join(runsDir, "run.sample")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// meta.json — scenario field makes Status derivation deterministic.
	mustWrite(t, filepath.Join(runDir, "meta.json"), `{"scenario":"01-demo","effort":"low","harness":"local","coder":"test","model":"claude-test"}`)

	// judge.txt — non-empty signals status="done".
	mustWrite(t, filepath.Join(runDir, "judge.txt"), "judge output line\n")

	// judge.json — conformant verdict; no expectations (nil) keeps ParseJudgeReport happy.
	mustWrite(t, filepath.Join(runDir, "judge.json"), `{"scenario":"01-demo","verdict":"conformant","checks":{"passed":true,"results":[]},"note":"ok"}`)

	// build.txt — empty means Build=PASS.
	mustWrite(t, filepath.Join(runDir, "build.txt"), "")

	// test.txt — contains "ok" without "FAIL" means Tests=PASS.
	mustWrite(t, filepath.Join(runDir, "test.txt"), "ok\n")

	// transcript.jsonl — empty is fine.
	mustWrite(t, filepath.Join(runDir, "transcript.jsonl"), "")

	return runsDir
}

// buildScenariosDir creates a scenarios/ tree under root with one 01-demo scenario.
func buildScenariosDir(t *testing.T, root string) string {
	t.Helper()
	scenDir := filepath.Join(root, "scenarios")
	demoDir := filepath.Join(scenDir, "01-demo")
	fixtureDir := filepath.Join(demoDir, "fixture")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// scenario.json — must pass scenarios.Validate (id, title, traps, judge with checks+expectations).
	mustWrite(t, filepath.Join(demoDir, "scenario.json"), `{
		"id": "01-demo",
		"title": "Demo scenario",
		"traps": [],
		"judge": {
			"checks": {"groundedBeforeWrite": true},
			"expectations": []
		}
	}`)

	// task.md — returned by scenarios.Load.
	mustWrite(t, filepath.Join(demoDir, "task.md"), "Do the demo task.\n")

	return scenDir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newServer(t *testing.T) (*httptest.Server, Config) {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
	}
	return httptest.NewServer(Handler(cfg)), cfg
}

func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func bodyStr(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHealthz(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if m["status"] != "ok" {
		t.Errorf("want status=ok, got %q", m["status"])
	}
}

func TestListRuns(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/runs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("want 1 run, got %d", len(list))
	}
	if list[0]["id"] != "run.sample" {
		t.Errorf("want id=run.sample, got %v", list[0]["id"])
	}
	if list[0]["status"] != "done" {
		t.Errorf("want status=done, got %v", list[0]["status"])
	}
}

func TestGetRun(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/runs/run.sample")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var detail map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	summary, ok := detail["summary"].(map[string]any)
	if !ok {
		t.Fatal("missing summary field")
	}
	if summary["id"] != "run.sample" {
		t.Errorf("want summary.id=run.sample, got %v", summary["id"])
	}
	// judgeReport must be parsed (verdict=conformant).
	if detail["judgeReport"] == nil {
		t.Error("want non-nil judgeReport")
	}
}

func TestGetRunNotFound(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/runs/run.nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGetRunLog(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/runs/run.sample/logs/judge.txt")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body := bodyStr(t, resp)
	if body != "judge output line\n" {
		t.Errorf("unexpected body: %q", body)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("want text/plain content-type, got %q", ct)
	}
}

func TestGetRunLogNotAllowlisted(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/runs/run.sample/logs/secret")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for non-allowlisted name, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGetRunLogTraversalID(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	// id without run. prefix — guard rejects it.
	resp := get(t, srv, "/runs/../evil/logs/judge.txt")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for traversal id, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestListScenarios(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/scenarios")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("want 1 scenario, got %d", len(list))
	}
	if list[0]["id"] != "01-demo" {
		t.Errorf("want id=01-demo, got %v", list[0]["id"])
	}
}

func TestGetScenario(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/scenarios/01-demo")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var detail map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cfg, ok := detail["config"].(map[string]any)
	if !ok {
		t.Fatal("missing config field")
	}
	if cfg["id"] != "01-demo" {
		t.Errorf("want config.id=01-demo, got %v", cfg["id"])
	}
	task, _ := detail["task"].(string)
	if task == "" {
		t.Error("want non-empty task")
	}
}

func TestGetScenarioNotFound(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/scenarios/99-nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCORSPreflight(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/runs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 for OPTIONS, got %d", resp.StatusCode)
	}
	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:8080" {
		t.Errorf("want CORS origin header, got %q", origin)
	}
}

func TestCORSOnGET(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := get(t, srv, "/healthz")
	defer resp.Body.Close()
	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:8080" {
		t.Errorf("want CORS origin on GET, got %q", origin)
	}
}
