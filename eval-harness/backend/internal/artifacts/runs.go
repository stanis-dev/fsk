package artifacts

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"backend/internal/judge"
)

// ParseJudgeReport parses a judge.json string into a judge.Report.
// Returns nil for empty input, parse errors, unknown verdicts, or malformed expectations.
func ParseJudgeReport(jsonText string) *judge.Report {
	if strings.TrimSpace(jsonText) == "" {
		return nil
	}
	var r judge.Report
	if err := json.Unmarshal([]byte(jsonText), &r); err != nil {
		return nil
	}
	if r.Verdict != "conformant" && r.Verdict != "NON-COMPLIANT" {
		return nil
	}
	if r.Expectations != nil && r.Expectations.Criteria == nil {
		return nil
	}
	return &r
}

func ListRuns(dir string) ([]Summary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading runs dir: %w", err)
	}
	var out []Summary
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "run.") {
			continue
		}
		d := filepath.Join(dir, e.Name())
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, SummarizeRun(d))
	}
	slices.SortFunc(out, func(a, b Summary) int {
		return cmp.Compare(b.UpdatedIso, a.UpdatedIso)
	})
	return out, nil
}

// SummarizeRun derives a Summary from the run directory.
func SummarizeRun(dir string) Summary {
	s := Summary{
		ID:         filepath.Base(dir),
		UpdatedIso: dirMtime(dir),
		Status:     "running",
	}

	log := logInfo(filepath.Join(dir, TranscriptFile))
	meta := readMeta(dir)
	s.Scenario = cmp.Or(meta.scenario, "-")
	s.Effort = cmp.Or(meta.effort, "-")
	s.Model = cmp.Or(log.model, meta.model)
	if log.ccver != "" {
		s.Coder = "claude-code"
	} else {
		s.Coder = cmp.Or(meta.coder, "?")
	}
	switch {
	case log.cwd == "/work":
		s.Harness = "docker"
	case log.cwd != "":
		s.Harness = "local"
	default:
		s.Harness = cmp.Or(meta.harness, "?")
	}

	if _, err := os.Stat(filepath.Join(dir, CancelledFile)); err == nil {
		s.Status = "cancelled"
		return s
	}

	judgeLog := readFile(filepath.Join(dir, JudgeLogFile))
	if judgeLog == "" {
		return s
	}
	s.Status = "done"

	report := ParseJudgeReport(readFile(filepath.Join(dir, JudgeJSONFile)))
	if report != nil {
		if report.Verdict == "conformant" {
			s.Judge = "PASS"
		} else {
			s.Judge = "FAIL"
		}
	}

	s.Build = "FAIL"
	if strings.TrimSpace(readFile(filepath.Join(dir, BuildFile))) == "" {
		s.Build = "PASS"
	}

	tt := readFile(filepath.Join(dir, TestFile))
	s.Tests = "FAIL"
	if tt != "" && !strings.Contains(tt, "FAIL") && strings.Contains(tt, "ok") {
		s.Tests = "PASS"
	}

	r := parseResult(filepath.Join(dir, TranscriptFile))
	s.Turns = r.turns
	s.Cost = r.cost
	return s
}

// LoadRun builds a RunDetail for the named run under baseDir.
// Returns nil, false if id is invalid or the directory does not exist.
func LoadRun(baseDir, id string) (*RunDetail, bool) {
	if !strings.HasPrefix(id, "run.") || strings.Contains(id, "/") || strings.Contains(id, "..") {
		return nil, false
	}
	d := filepath.Join(baseDir, id)
	fi, err := os.Stat(d)
	if err != nil || !fi.IsDir() {
		return nil, false
	}
	rd := &RunDetail{
		Summary:     SummarizeRun(d),
		JudgeLog:    readFile(filepath.Join(d, JudgeLogFile)),
		JudgeReport: ParseJudgeReport(readFile(filepath.Join(d, JudgeJSONFile))),
		BuildLog:    readFile(filepath.Join(d, BuildFile)),
		TestLog:     readFile(filepath.Join(d, TestFile)),
		Err:         readFile(filepath.Join(d, CoderErrFile)),
		Transcript:  ParseTranscript(readFile(filepath.Join(d, TranscriptFile))),
		Diff:        ClassifyDiff(readFile(filepath.Join(d, DiffFile))),
		Telemetry:   SummarizeTelemetry(ParseTelemetry(readFile(filepath.Join(d, TelemetryFile)))),
	}
	return rd, true
}

type resultInfo struct {
	turns string
	cost  string
}

func parseResult(file string) resultInfo {
	var ri resultInfo
	scanJSONL(readFile(file), func(m map[string]json.RawMessage) {
		var typ string
		if err := json.Unmarshal(m["type"], &typ); err != nil || typ != "result" {
			return
		}
		if raw, ok := m["num_turns"]; ok {
			var n float64
			if err := json.Unmarshal(raw, &n); err == nil {
				ri.turns = fmt.Sprintf("%d", int(math.Round(n)))
			}
		}
		if raw, ok := m["total_cost_usd"]; ok {
			var c float64
			if err := json.Unmarshal(raw, &c); err == nil {
				ri.cost = fmt.Sprintf("$%.2f", c)
			}
		}
	})
	return ri
}

type logInfoResult struct {
	model string
	cwd   string
	ccver string
}

func logInfo(file string) logInfoResult {
	var r logInfoResult
	found := false
	scanJSONL(readFile(file), func(m map[string]json.RawMessage) {
		if found {
			return
		}
		var typ string
		if err := json.Unmarshal(m["type"], &typ); err != nil || typ != "system" {
			return
		}
		found = true
		if v, ok := m["model"]; ok {
			_ = json.Unmarshal(v, &r.model)
		}
		if v, ok := m["cwd"]; ok {
			_ = json.Unmarshal(v, &r.cwd)
		}
		if v, ok := m["claude_code_version"]; ok {
			_ = json.Unmarshal(v, &r.ccver)
		}
	})
	return r
}

type metaInfo struct {
	harness  string
	coder    string
	model    string
	effort   string
	scenario string
}

func readMeta(dir string) metaInfo {
	data, err := os.ReadFile(filepath.Join(dir, MetaFile))
	if err != nil {
		return metaInfo{}
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return metaInfo{}
	}
	return metaInfo{
		harness:  m["harness"],
		coder:    m["coder"],
		model:    m["model"],
		effort:   m["effort"],
		scenario: m["scenario"],
	}
}

func readFile(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(data)
}

func dirMtime(dir string) string {
	fi, err := os.Stat(dir)
	if err != nil {
		return "1970-01-01T00:00:00.000Z"
	}
	return fi.ModTime().UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
