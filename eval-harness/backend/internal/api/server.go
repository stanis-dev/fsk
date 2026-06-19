// Package api serves runs and scenarios over a JSON HTTP API.
package api

import (
	"encoding/json"
	"net/http"

	"backend/internal/jobs"
)

// RunService is the subset of jobs.Service the API layer depends on.
type RunService interface {
	Enqueue(scenarioID, model, effort string) (string, error)
	Cancel(runID string) (bool, error)
	Subscribe() (<-chan jobs.Event, func())
}

// Config is the resolved server configuration.
type Config struct {
	RunsDir      string     // dir holding run.* directories (e.g. ~/.cache/fiskaly-eval)
	ScenariosDir string     // dir holding NN-slug scenario directories
	CORSOrigin   string     // exact allowed browser origin (e.g. http://localhost:8080)
	Service      RunService // may be nil when running without a job worker pool
}

// Handler returns the API router with CORS applied.
func Handler(cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /runs", cfg.listRuns)
	mux.HandleFunc("GET /runs/{id}", cfg.getRun)
	mux.HandleFunc("GET /runs/{id}/logs/{name}", cfg.getRunLog)
	mux.HandleFunc("GET /scenarios", cfg.listScenarios)
	mux.HandleFunc("GET /scenarios/{id}", cfg.getScenario)
	mux.HandleFunc("PUT /scenarios/{id}", cfg.putScenario)
	mux.HandleFunc("POST /runs", cfg.postRun)
	mux.HandleFunc("POST /runs/{id}/cancel", cfg.cancelRun)
	mux.HandleFunc("GET /runs/stream", cfg.streamRuns)
	mux.HandleFunc("GET /runs/{id}/events", cfg.streamRun)
	return cors(cfg.CORSOrigin, mux)
}

// cors allows exactly the configured browser origin and answers preflight.
func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
