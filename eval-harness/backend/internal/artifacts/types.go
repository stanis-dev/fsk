package artifacts

import "backend/internal/judge"

type Summary struct {
	ID         string `json:"id"`
	UpdatedIso string `json:"updatedIso"`
	Status     string `json:"status"`
	Scenario   string `json:"scenario"`
	Coder      string `json:"coder"`
	Harness    string `json:"harness"`
	Model      string `json:"model"`
	Effort     string `json:"effort"`
	Build      string `json:"build"`
	Tests      string `json:"tests"`
	Judge      string `json:"judge"`
	Turns      string `json:"turns"`
	Cost       string `json:"cost"`
}

type TranscriptEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type DiffLine struct {
	Cls  string `json:"cls"`
	Text string `json:"text"`
}

type TelemetryEvent struct {
	Ts          string         `json:"ts"`
	Tool        string         `json:"tool"`
	Args        map[string]any `json:"args"`
	ResultCount int            `json:"resultCount"`
	IsError     bool           `json:"isError"`
	Error       string         `json:"error"`
	LatencyMs   int            `json:"latencyMs"`
}

type TelemetryToolStat struct {
	Tool   string `json:"tool"`
	Calls  int    `json:"calls"`
	Errors int    `json:"errors"`
}

type TelemetrySummary struct {
	Total        int                 `json:"total"`
	Errors       int                 `json:"errors"`
	ByTool       []TelemetryToolStat `json:"byTool"`
	P50LatencyMs int                 `json:"p50LatencyMs"`
	P95LatencyMs int                 `json:"p95LatencyMs"`
	Queries      []string            `json:"queries"`
	DocsFetched  []string            `json:"docsFetched"`
}

type RunDetail struct {
	Summary     Summary           `json:"summary"`
	JudgeLog    string            `json:"judgeLog"`
	JudgeReport *judge.Report     `json:"judgeReport"`
	BuildLog    string            `json:"buildLog"`
	TestLog     string            `json:"testLog"`
	Err         string            `json:"err"`
	Transcript  []TranscriptEvent `json:"transcript"`
	Diff        []DiffLine        `json:"diff"`
	Telemetry   TelemetrySummary  `json:"telemetry"`
}
