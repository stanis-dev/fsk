package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// stream subscribes to the service and writes events as SSE until the client
// disconnects. filter, if non-empty, restricts to events for that run id.
func (cfg Config) stream(w http.ResponseWriter, r *http.Request, filter string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsubscribe := cfg.Service.Subscribe()
	defer unsubscribe()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			if filter != "" && ev.RunID != filter {
				continue
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (cfg Config) streamRuns(w http.ResponseWriter, r *http.Request) {
	cfg.stream(w, r, "")
}

func (cfg Config) streamRun(w http.ResponseWriter, r *http.Request) {
	cfg.stream(w, r, r.PathValue("id"))
}
