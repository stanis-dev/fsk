// Package sim is a local stand-in for the fiskaly SIGN IT TEST API,
// faithful to the behaviors learned from live probing: content envelopes,
// idempotency-key enforcement with a replay cache, composite record types,
// the INTENTION→TRANSACTION pattern and the commissioning lifecycle.
//
// Its purpose is twofold: a demo safety net (the MCP server runs against it
// with --base-url) and chaos engineering — fault scenarios the real TEST
// environment cannot produce on demand, like an AdE outage.
package sim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Scenario string

const (
	ScenarioHappy     Scenario = "happy"
	ScenarioAdeOutage Scenario = "ade-outage" // transactions go FAILED: AdE unreachable
	ScenarioSlowAde   Scenario = "slow-ade"   // transactions stay PROCESSING; complete after polls
)

type resource struct {
	Type  string
	State string
	Mode  string
	Body  map[string]any
}

type record struct {
	ID         string
	Type       string // composite, e.g. TRANSACTION::RECEIPT
	State      string
	Mode       string
	SystemID   string
	RecordID   string
	Operation  string
	polls      int // for slow-ade: completes on 2nd poll
	Compliance map[string]any
}

type Server struct {
	Scenario Scenario

	mu         sync.Mutex
	resources  map[string]map[string]*resource // collection -> id -> resource
	records    map[string]*record
	idempotent map[string]idemEntry // idempotency key -> first use
	docSeq     int
}

type idemEntry struct {
	signature string // method+path+payload hash
	response  []byte
	status    int
}

func New(scenario Scenario) *Server {
	if scenario == "" {
		scenario = ScenarioHappy
	}
	return &Server{
		Scenario: scenario,
		resources: map[string]map[string]*resource{
			"organizations": {}, "subjects": {}, "taxpayers": {}, "locations": {}, "systems": {},
		},
		records:    map[string]*record{},
		idempotent: map[string]idemEntry{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tokens", s.createToken)
	for _, col := range []string{"organizations", "subjects", "taxpayers", "locations", "systems"} {
		col := col
		mux.HandleFunc("POST /"+col, func(w http.ResponseWriter, r *http.Request) { s.createResource(w, r, col) })
		mux.HandleFunc("PATCH /"+col+"/{id}", func(w http.ResponseWriter, r *http.Request) { s.patchResource(w, r, col) })
		mux.HandleFunc("GET /"+col+"/{id}", func(w http.ResponseWriter, r *http.Request) { s.getResource(w, r, col) })
	}
	mux.HandleFunc("POST /records", s.createRecord)
	mux.HandleFunc("GET /records/{id}", s.getRecord)
	return s.protocol(mux)
}

// protocol enforces the cross-cutting rules the real API enforces.
func (s *Server) protocol(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Version", "2026-02-03")
		w.Header().Set("X-Trace-Identifier", uuid.NewString())
		if r.Header.Get("X-Api-Version") == "" {
			s.fail(w, 400, "E_BAD_REQUEST", "header X-Api-Version is required")
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			key := r.Header.Get("X-Idempotency-Key")
			if key == "" {
				s.fail(w, 400, "E_BAD_REQUEST", "header X-Idempotency-Key is required")
				return
			}
			if uuid.Validate(key) != nil {
				s.fail(w, 400, "E_BAD_REQUEST", "header X-Idempotency-Key must be a UUID")
				return
			}
			s.mu.Lock()
			entry, seen := s.idempotent[key]
			s.mu.Unlock()
			if seen {
				if entry.signature != r.Method+" "+r.URL.Path {
					s.fail(w, 422, "E_UNPROCESSABLE_CONTENT", "X-Idempotency-Key was already used with a different request")
					return
				}
				w.Header().Set("X-Idempotency-Replayed", "true")
				w.WriteHeader(entry.status)
				w.Write(entry.response)
				return
			}
			rec := &responseRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)
			s.mu.Lock()
			s.idempotent[key] = idemEntry{signature: r.Method + " " + r.URL.Path, response: rec.buf, status: rec.status}
			s.mu.Unlock()
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	buf    []byte
}

func (r *responseRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.buf = append(r.buf, b...)
	return r.ResponseWriter.Write(b)
}

func (s *Server) fail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"content": map[string]any{"status": status, "code": code, "error": http.StatusText(status), "message": msg},
	})
}

func (s *Server) reply(w http.ResponseWriter, content any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"content": content})
}

func readContent(r *http.Request) (map[string]any, error) {
	var body struct {
		Content map[string]any `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Content == nil {
		return nil, fmt.Errorf("missing content envelope")
	}
	return body.Content, nil
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	content, err := readContent(r)
	if err != nil || content["key"] == "" || content["secret"] == "" {
		s.fail(w, 401, "E_UNAUTHORIZED_ACCESS", "invalid credentials")
		return
	}
	s.reply(w, map[string]any{
		"id": uuid.NewString(),
		"authentication": map[string]any{
			"type": "JWT", "bearer": "sim-" + uuid.NewString(),
			"issued_at": "2026-06-12T00:00:00Z", "expires_at": "2099-01-01T00:00:00Z",
		},
		"organization": map[string]any{"id": uuid.NewString()},
		"subject":      map[string]any{"id": uuid.NewString()},
	})
}

func (s *Server) createResource(w http.ResponseWriter, r *http.Request, col string) {
	content, err := readContent(r)
	if err != nil {
		s.fail(w, 400, "E_BAD_REQUEST", err.Error())
		return
	}
	id := uuid.NewString()
	res := &resource{Type: str(content["type"]), State: "ACQUIRED", Mode: "INACTIVE", Body: content}
	out := map[string]any{"id": id, "state": res.State, "mode": res.Mode, "type": res.Type}
	switch col {
	case "organizations":
		res.State, res.Mode = "ENABLED", ""
		out = map[string]any{"id": id, "state": "ENABLED", "type": res.Type, "name": content["name"]}
	case "subjects":
		res.State, res.Mode = "ENABLED", ""
		out = map[string]any{
			"id": id, "state": "ENABLED", "type": res.Type, "name": content["name"],
			"credentials": map[string]any{"key": "sim-key-" + id[:8], "secret": "sim-secret-" + id[:8]},
		}
	case "systems":
		if loc, ok := content["location"].(map[string]any); !ok || str(loc["id"]) == "" {
			s.fail(w, 400, "E_BAD_REQUEST", "system requires a location")
			return
		}
	}
	s.mu.Lock()
	s.resources[col][id] = res
	s.mu.Unlock()
	s.reply(w, out)
}

func (s *Server) patchResource(w http.ResponseWriter, r *http.Request, col string) {
	id := r.PathValue("id")
	s.mu.Lock()
	res, ok := s.resources[col][id]
	s.mu.Unlock()
	if !ok {
		s.fail(w, 404, "E_RESOURCE_NOT_FOUND", col+" not found")
		return
	}
	content, err := readContent(r)
	if err != nil {
		s.fail(w, 400, "E_BAD_REQUEST", err.Error())
		return
	}
	if state := str(content["state"]); state != "" {
		if res.State == "DECOMMISSIONED" {
			s.fail(w, 409, "E_RESOURCE_CONFLICT", "decommissioning is irreversible")
			return
		}
		res.State = state
		if state == "COMMISSIONED" {
			res.Mode = "OPERATIVE"
		}
	}
	s.reply(w, map[string]any{"id": id, "state": res.State, "mode": res.Mode, "type": res.Type})
}

func (s *Server) getResource(w http.ResponseWriter, r *http.Request, col string) {
	id := r.PathValue("id")
	s.mu.Lock()
	res, ok := s.resources[col][id]
	s.mu.Unlock()
	if !ok {
		s.fail(w, 404, "E_RESOURCE_NOT_FOUND", col+" not found")
		return
	}
	s.reply(w, map[string]any{"id": id, "state": res.State, "mode": res.Mode, "type": res.Type})
}

func (s *Server) createRecord(w http.ResponseWriter, r *http.Request) {
	content, err := readContent(r)
	if err != nil {
		s.fail(w, 400, "E_BAD_REQUEST", err.Error())
		return
	}
	op, _ := json.Marshal(content["operation"])
	id := uuid.NewString()

	switch str(content["type"]) {
	case "INTENTION":
		sys, _ := content["system"].(map[string]any)
		sysID := str(sys["id"])
		s.mu.Lock()
		system, ok := s.resources["systems"][sysID]
		s.mu.Unlock()
		if !ok {
			s.fail(w, 404, "E_RESOURCE_NOT_FOUND", "system not found")
			return
		}
		if system.Mode != "OPERATIVE" {
			s.fail(w, 405, "E_METHOD_NOT_ALLOWED", fmt.Sprintf("record operation invalid: system '%s' is not OPERATIVE (state %s)", sysID, system.State))
			return
		}
		rec := &record{ID: id, Type: "INTENTION::TRANSACTION", State: "ACCEPTED", Mode: "PROCESSING", SystemID: sysID, Operation: string(op)}
		s.mu.Lock()
		s.records[id] = rec
		s.mu.Unlock()
		s.reply(w, map[string]any{"id": id, "type": rec.Type, "state": rec.State, "mode": rec.Mode, "system": map[string]any{"id": sysID}, "operation": string(op)})

	case "TRANSACTION":
		ref, _ := content["record"].(map[string]any)
		refID := str(ref["id"])
		s.mu.Lock()
		intention, ok := s.records[refID]
		s.mu.Unlock()
		if !ok || !strings.HasPrefix(intention.Type, "INTENTION") {
			s.fail(w, 404, "E_RESOURCE_NOT_FOUND", fmt.Sprintf("record operation invalid: intention '%s' not found", refID))
			return
		}
		var opType struct {
			Type string `json:"type"`
		}
		json.Unmarshal(op, &opType)
		rec := &record{ID: id, Type: "TRANSACTION::" + opType.Type, State: "ACCEPTED", Mode: "PROCESSING", RecordID: refID, Operation: string(op)}

		switch s.Scenario {
		case ScenarioAdeOutage:
			rec.State, rec.Mode = "FAILED", "FINISHED"
			rec.Compliance = map[string]any{
				"data": "AdE web service unreachable: the documento commerciale was NOT transmitted",
			}
		case ScenarioSlowAde:
			// stays PROCESSING; completes after two polls in getRecord
		default:
			s.complete(rec)
		}
		s.mu.Lock()
		intention.State, intention.Mode = "COMPLETED", "FINISHED"
		s.records[id] = rec
		s.mu.Unlock()
		out := map[string]any{"id": id, "type": rec.Type, "state": rec.State, "mode": rec.Mode, "record": map[string]any{"id": refID}, "operation": string(op)}
		if rec.Compliance != nil {
			out["compliance"] = rec.Compliance
		}
		s.reply(w, out)

	default:
		s.fail(w, 400, "E_BAD_REQUEST", "record type must be INTENTION or TRANSACTION")
	}
}

func (s *Server) complete(rec *record) {
	s.docSeq++
	rec.State, rec.Mode = "COMPLETED", "FINISHED"
	rec.Compliance = map[string]any{
		"data": fmt.Sprintf("DCW0000/%04d-0000", s.docSeq),
		"url":  "https://ivaservizi.agenziaentrate.gov.it/ser/api/documenti/v1/doc/documenti/00000000/stampa/",
	}
}

func (s *Server) getRecord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	rec, ok := s.records[id]
	if ok && s.Scenario == ScenarioSlowAde && rec.Mode == "PROCESSING" && strings.HasPrefix(rec.Type, "TRANSACTION") {
		rec.polls++
		if rec.polls >= 2 {
			s.complete(rec)
		}
	}
	s.mu.Unlock()
	if !ok {
		s.fail(w, 404, "E_RESOURCE_NOT_FOUND", "record not found")
		return
	}
	out := map[string]any{"id": rec.ID, "type": rec.Type, "state": rec.State, "mode": rec.Mode, "operation": rec.Operation}
	if rec.Compliance != nil {
		out["compliance"] = rec.Compliance
	}
	s.reply(w, out)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
