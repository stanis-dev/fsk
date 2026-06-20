package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/jobs"
)

func buildRunsDir(t *testing.T, root string) string {
	t.Helper()
	runsDir := filepath.Join(root, "runs")
	runDir := filepath.Join(runsDir, "run.sample")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(runDir, "meta.json"), `{"scenario":"01-demo","effort":"low","harness":"local","coder":"test","model":"claude-test"}`)
	mustWrite(t, filepath.Join(runDir, "judge.txt"), "judge output line\n")
	mustWrite(t, filepath.Join(runDir, "judge.json"), `{"scenario":"01-demo","verdict":"conformant","checks":{"passed":true,"results":[]},"note":"ok"}`)
	mustWrite(t, filepath.Join(runDir, "build.txt"), "")
	mustWrite(t, filepath.Join(runDir, "test.txt"), "ok\n")
	mustWrite(t, filepath.Join(runDir, "transcript.jsonl"), "")

	return runsDir
}

func buildScenariosDir(t *testing.T, root string) string {
	t.Helper()
	scenDir := filepath.Join(root, "scenarios")
	demoDir := filepath.Join(scenDir, "01-demo")
	fixtureDir := filepath.Join(demoDir, "fixture")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(demoDir, "scenario.json"), `{
		"id": "01-demo",
		"title": "Demo scenario",
		"traps": [],
		"judge": {
			"checks": {"groundedBeforeWrite": true},
			"expectations": []
		}
	}`)

	mustWrite(t, filepath.Join(demoDir, "task.md"), "Do the demo task.\n")

	return scenDir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeService struct {
	enqueueErr   error
	cancelOK     bool
	cancelErr    error
	subCh        chan jobs.Event
	unsubscribed atomic.Bool
}

func (f *fakeService) Enqueue(scenarioID string) (string, error) {
	if f.enqueueErr != nil {
		return "", f.enqueueErr
	}
	return "job.1", nil
}

func (f *fakeService) Cancel(runID string) (bool, error) { return f.cancelOK, f.cancelErr }

func (f *fakeService) Subscribe() (<-chan jobs.Event, func()) {
	ch := f.subCh
	if ch == nil {
		ch = make(chan jobs.Event, 4)
	}
	return ch, func() { f.unsubscribed.Store(true) }
}

func newServer(t *testing.T) (*httptest.Server, Config) {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      &fakeService{cancelOK: true},
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

func TestListRunsDirError(t *testing.T) {
	root := t.TempDir()
	cfg := Config{
		RunsDir:      filepath.Join(root, "missing"),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      &fakeService{cancelOK: true},
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	resp := get(t, srv, "/runs")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
	resp.Body.Close()
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

func TestGetScenarioMalformedJSON(t *testing.T) {
	root := t.TempDir()
	scenariosDir := buildScenariosDir(t, root)
	mustWrite(t, filepath.Join(scenariosDir, "01-demo", "scenario.json"), "{ not json")
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: scenariosDir,
		CORSOrigin:   "http://localhost:8080",
		Service:      &fakeService{cancelOK: true},
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	resp := get(t, srv, "/scenarios/01-demo")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
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

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPostRunOK(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := post(t, srv, "/runs", `{"scenarioId":"01-demo"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}
	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["runId"] == "" {
		t.Error("want non-empty runId")
	}
}

func TestPostRunEmptyBody(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := post(t, srv, "/runs", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestPostRunInvalidJSON(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := post(t, srv, "/runs", `not-json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestPostRunUnknownScenario(t *testing.T) {
	root := t.TempDir()
	svc := &fakeService{enqueueErr: fmt.Errorf("unknown scenario %q", "99-nope")}
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      svc,
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	resp := post(t, srv, "/runs", `{"scenarioId":"99-nope"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCancelRunOK(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := post(t, srv, "/runs/job.1/cancel", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
}

func TestCancelRunNotFound(t *testing.T) {
	root := t.TempDir()
	svc := &fakeService{cancelOK: false}
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      svc,
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	resp := post(t, srv, "/runs/job.99/cancel", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestCancelRunError(t *testing.T) {
	root := t.TempDir()
	svc := &fakeService{cancelOK: true, cancelErr: fmt.Errorf("kill failed")}
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      svc,
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	resp := post(t, srv, "/runs/job.1/cancel", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func put(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPutScenarioOK(t *testing.T) {
	srv, cfg := newServer(t)
	defer srv.Close()

	body := `{"config":{"id":"01-demo","title":"Updated","traps":[],"judge":{"checks":{"groundedBeforeWrite":true},"expectations":[]}},"task":"updated task\n"}`
	resp := put(t, srv, "/scenarios/01-demo", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", resp.StatusCode, bodyStr(t, resp))
	}
	raw, err := os.ReadFile(filepath.Join(cfg.ScenariosDir, "01-demo", "scenario.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"Updated"`) {
		t.Errorf("scenario.json not updated: %s", raw)
	}
	taskRaw, err := os.ReadFile(filepath.Join(cfg.ScenariosDir, "01-demo", "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(taskRaw) != "updated task\n" {
		t.Errorf("task.md not updated: %q", string(taskRaw))
	}
}

func TestPutScenarioIDMismatch(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	body := `{"config":{"id":"99-wrong","title":"X","traps":[],"judge":{"checks":{"groundedBeforeWrite":true},"expectations":[]}},"task":""}`
	resp := put(t, srv, "/scenarios/01-demo", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for id mismatch, got %d", resp.StatusCode)
	}
}

func TestPutScenarioBadBody(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp := put(t, srv, "/scenarios/01-demo", `not-json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for bad body, got %d", resp.StatusCode)
	}
}

func TestCORSPreflightIncludesPUT(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/scenarios/01-demo", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "PUT") {
		t.Errorf("want PUT in Allow-Methods, got %q", methods)
	}
}

func TestCORSPreflightIncludesPost(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/runs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Errorf("want POST in Allow-Methods, got %q", methods)
	}
}

func TestStreamRunsSSE(t *testing.T) {
	root := t.TempDir()
	evCh := make(chan jobs.Event, 4)
	svc := &fakeService{cancelOK: true, subCh: evCh}
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      svc,
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/runs/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		resp.Body.Close()
		t.Fatalf("want Content-Type text/event-stream, got %q", ct)
	}

	evCh <- jobs.Event{RunID: "job.1", ScenarioID: "01-demo", Phase: "running"}

	// Read lines until we find the data frame or the context expires.
	scanner := bufio.NewScanner(resp.Body)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if !strings.Contains(payload, `"phase":"running"`) {
				t.Errorf("want phase=running in payload: %s", payload)
			}
			if !strings.Contains(payload, `"runId":"job.1"`) {
				t.Errorf("want runId=job.1 in payload: %s", payload)
			}
			if !strings.Contains(payload, `"scenarioId":"01-demo"`) {
				t.Errorf("want scenarioId=01-demo in payload: %s", payload)
			}
			found = true
			break
		}
	}
	resp.Body.Close()
	if !found {
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			t.Fatalf("scanner error: %v", err)
		}
		t.Fatal("no data frame received before timeout")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.unsubscribed.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("unsubscribe was not called after client disconnect")
}

func TestStreamRunEventsFiltersOtherRuns(t *testing.T) {
	root := t.TempDir()
	evCh := make(chan jobs.Event, 4)
	svc := &fakeService{cancelOK: true, subCh: evCh}
	cfg := Config{
		RunsDir:      buildRunsDir(t, root),
		ScenariosDir: buildScenariosDir(t, root),
		CORSOrigin:   "http://localhost:8080",
		Service:      svc,
	}
	srv := httptest.NewServer(Handler(cfg))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/runs/job.1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("want Content-Type text/event-stream, got %q", ct)
	}

	evCh <- jobs.Event{RunID: "job.99", ScenarioID: "01-demo", Phase: "running"}
	evCh <- jobs.Event{RunID: "job.1", ScenarioID: "01-demo", Phase: "done"}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if strings.Contains(payload, "job.99") {
			t.Errorf("filtered event for job.99 leaked through: %s", payload)
			return
		}
		if strings.Contains(payload, `"phase":"done"`) {
			return
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		t.Fatalf("scanner error: %v", err)
	}
	t.Fatal("no data frame for job.1 received before timeout")
}
