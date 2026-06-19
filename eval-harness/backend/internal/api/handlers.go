package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

func (cfg Config) listRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, artifacts.ListRuns(cfg.RunsDir))
}

func (cfg Config) getRun(w http.ResponseWriter, r *http.Request) {
	detail, ok := artifacts.LoadRun(cfg.RunsDir, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// allowedLogs is the set of raw artifact files servable by name.
var allowedLogs = map[string]bool{
	artifacts.MetaFile: true, artifacts.RunHandleFile: true, artifacts.BuildFile: true,
	artifacts.TestFile: true, artifacts.JudgeLogFile: true, artifacts.DiffFile: true,
	artifacts.TranscriptFile: true, artifacts.CoderErrFile: true,
	artifacts.TelemetryFile: true, artifacts.JudgeJSONFile: true,
}

func (cfg Config) getRunLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	if !strings.HasPrefix(id, "run.") || strings.Contains(id, "/") || strings.Contains(id, "..") || !allowedLogs[name] {
		writeError(w, http.StatusNotFound, "no such log")
		return
	}
	data, err := os.ReadFile(filepath.Join(cfg.RunsDir, id, name))
	if err != nil {
		writeError(w, http.StatusNotFound, "no such log")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (cfg Config) listScenarios(w http.ResponseWriter, r *http.Request) {
	list, err := scenarios.List(cfg.ScenariosDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type scenarioDetail struct {
	Config *scenarios.Config `json:"config"`
	Task   string            `json:"task"`
}

func (cfg Config) getScenario(w http.ResponseWriter, r *http.Request) {
	c, task, ok := scenarios.Load(cfg.ScenariosDir, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusOK, scenarioDetail{Config: c, Task: task})
}
