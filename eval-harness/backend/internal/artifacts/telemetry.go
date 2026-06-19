package artifacts

import (
	"bufio"
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// ParseTelemetry parses a JSONL telemetry file into typed events.
func ParseTelemetry(jsonl string) []TelemetryEvent {
	var out []TelemetryEvent
	sc := bufio.NewScanner(strings.NewReader(jsonl))
	sc.Buffer(make([]byte, 16*1024*1024), 16*1024*1024)
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			continue
		}
		var r map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &r); err != nil {
			continue
		}

		e := TelemetryEvent{Args: map[string]any{}}
		if v, ok := r["ts"]; ok {
			_ = json.Unmarshal(v, &e.Ts)
		}
		if v, ok := r["session_id"]; ok {
			_ = json.Unmarshal(v, &e.SessionID)
		}
		if v, ok := r["tool"]; ok {
			_ = json.Unmarshal(v, &e.Tool)
		}
		if v, ok := r["args"]; ok {
			var args map[string]any
			if err := json.Unmarshal(v, &args); err == nil {
				e.Args = args
			}
		}
		if v, ok := r["result_count"]; ok {
			_ = json.Unmarshal(v, &e.ResultCount)
		}
		if v, ok := r["is_error"]; ok {
			_ = json.Unmarshal(v, &e.IsError)
		}
		if v, ok := r["error"]; ok {
			_ = json.Unmarshal(v, &e.Error)
		}
		if v, ok := r["latency_ms"]; ok {
			_ = json.Unmarshal(v, &e.LatencyMs)
		}
		out = append(out, e)
	}
	return out
}

// SummarizeTelemetry aggregates telemetry events into a summary.
func SummarizeTelemetry(events []TelemetryEvent) TelemetrySummary {
	byTool := map[string]*TelemetryToolStat{}
	toolOrder := []string{}
	latencies := make([]int, 0, len(events))
	queriesSeen := map[string]bool{}
	var queries []string
	docsSeen := map[string]bool{}
	var docs []string
	errors := 0

	for _, e := range events {
		st, exists := byTool[e.Tool]
		if !exists {
			st = &TelemetryToolStat{Tool: e.Tool}
			byTool[e.Tool] = st
			toolOrder = append(toolOrder, e.Tool)
		}
		st.Calls++
		if e.IsError {
			st.Errors++
			errors++
		}
		latencies = append(latencies, e.LatencyMs)
		if e.Tool == "search_fiskaly_docs" {
			if q, ok := e.Args["query"].(string); ok && !queriesSeen[q] {
				queriesSeen[q] = true
				queries = append(queries, q)
			}
		}
		if e.Tool == "fetch_fiskaly_doc" {
			if id, ok := e.Args["id"].(string); ok && !docsSeen[id] {
				docsSeen[id] = true
				docs = append(docs, id)
			}
		}
	}

	sorted := make([]TelemetryToolStat, 0, len(toolOrder))
	for _, t := range toolOrder {
		sorted = append(sorted, *byTool[t])
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Calls > sorted[j].Calls
	})

	sort.Ints(latencies)

	if queries == nil {
		queries = []string{}
	}
	if docs == nil {
		docs = []string{}
	}

	return TelemetrySummary{
		Total:        len(events),
		Errors:       errors,
		ByTool:       sorted,
		P50LatencyMs: percentile(latencies, 50),
		P95LatencyMs: percentile(latencies, 95),
		Queries:      queries,
		DocsFetched:  docs,
	}
}

func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Min(float64(len(sorted)-1), math.Floor(float64(p)/100.0*float64(len(sorted)))))
	return sorted[idx]
}
