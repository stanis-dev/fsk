package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/artifacts"
	"backend/internal/scenarios"
)

func (cfg Config) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := artifacts.ListRuns(cfg.RunsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (cfg Config) getRun(w http.ResponseWriter, r *http.Request) {
	detail, ok := artifacts.LoadRun(cfg.RunsDir, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
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
	c, task, err := scenarios.Load(cfg.ScenariosDir, r.PathValue("id"))
	if errors.Is(err, scenarios.ErrScenarioNotFound) {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scenarioDetail{Config: c, Task: task})
}

func (cfg Config) putScenario(w http.ResponseWriter, r *http.Request) {
	var body scenarioDetail
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Config == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := scenarios.Save(cfg.ScenariosDir, r.PathValue("id"), *body.Config, body.Task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg Config) postRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScenarioID string `json:"scenarioId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioID == "" {
		writeError(w, http.StatusBadRequest, "scenarioId required")
		return
	}
	id, err := cfg.Service.Enqueue(body.ScenarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"runId": id})
}

func (cfg Config) cancelRun(w http.ResponseWriter, r *http.Request) {
	ok, err := cfg.Service.Cancel(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no live run to cancel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
