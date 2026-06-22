package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

type toolReq struct {
	Name string `json:"name"`
	Min  int    `json:"min"`
}

// judgeChecks is scenario.json's judge.checks; an unset field asserts nothing.
type judgeChecks struct {
	GroundedBeforeWrite bool      `json:"groundedBeforeWrite"`
	ToolsCalled         []toolReq `json:"toolsCalled"`
	DocsFetched         []string  `json:"docsFetched"`
	MaxMcpErrors        *int      `json:"maxMcpErrors"`
}

type checkResult struct {
	ID     string `json:"id"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

var writeTools = map[string]bool{"Write": true, "Edit": true, "MultiEdit": true}

// toolMatches accepts both the bare name (Write) and the MCP-prefixed form
// (mcp__server__name, e.g. mcp__fiskaly__search_fiskaly_docs).
func toolMatches(transcriptName, name string) bool {
	return transcriptName == name || strings.HasSuffix(transcriptName, "__"+name)
}

func runChecks(c judgeChecks, t trajectory) []checkResult {
	var out []checkResult

	if c.GroundedBeforeWrite {
		searchAt := slices.IndexFunc(t.ToolUses, func(n string) bool { return toolMatches(n, "search_fiskaly_docs") })
		writeAt := slices.IndexFunc(t.ToolUses, func(n string) bool { return writeTools[n] })
		out = append(out, groundedResult(searchAt, writeAt))
	}

	for _, req := range c.ToolsCalled {
		wantMin := req.Min
		if wantMin < 1 {
			wantMin = 1
		}
		got := countOccurrences(t.ToolUses, req.Name)
		out = append(out, checkResult{
			ID:     "toolsCalled:" + req.Name,
			Pass:   got >= wantMin,
			Detail: fmt.Sprintf("called %dx (min %d)", got, wantMin),
		})
	}

	if len(c.DocsFetched) > 0 {
		var fetchedIDs []string
		for _, e := range t.Telemetry {
			if toolMatches(e.Tool, "fetch_fiskaly_doc") {
				if id, ok := e.Args["id"].(string); ok {
					fetchedIDs = append(fetchedIDs, id)
				}
			}
		}
		for _, want := range c.DocsFetched {
			ok := false
			matchedID := ""
			for _, id := range fetchedIDs {
				if id == want || strings.Contains(id, want) {
					ok = true
					matchedID = id
					break
				}
			}
			detail := "no fetched doc matching " + want
			if ok {
				detail = "fetched " + matchedID
			}
			out = append(out, checkResult{
				ID:     "docsFetched:" + want,
				Pass:   ok,
				Detail: detail,
			})
		}
	}

	if c.MaxMcpErrors != nil {
		errs := 0
		for _, e := range t.Telemetry {
			if e.IsError {
				errs++
			}
		}
		out = append(out, checkResult{
			ID:     "maxMcpErrors",
			Pass:   errs <= *c.MaxMcpErrors,
			Detail: fmt.Sprintf("%d MCP errors (max %d)", errs, *c.MaxMcpErrors),
		})
	}

	return out
}

func checksPassed(rs []checkResult) bool {
	for _, r := range rs {
		if !r.Pass {
			return false
		}
	}
	return true
}

func groundedResult(searchAt, writeAt int) checkResult {
	r := checkResult{ID: "groundedBeforeWrite"}
	switch {
	case searchAt == -1:
		r.Detail = "agent never called search_fiskaly_docs"
	case writeAt == -1:
		r.Pass, r.Detail = true, "searched; no code-write tool used"
	case searchAt < writeAt:
		r.Pass, r.Detail = true, fmt.Sprintf("searched (tool %d) before first write (tool %d)", searchAt, writeAt)
	default:
		r.Detail = fmt.Sprintf("first write (tool %d) precedes first search (tool %d)", writeAt, searchAt)
	}
	return r
}

func countOccurrences(xs []string, name string) int {
	n := 0
	for _, x := range xs {
		if toolMatches(x, name) {
			n++
		}
	}
	return n
}

func parseScenarioChecks(path string) (judgeChecks, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return judgeChecks{}, fmt.Errorf("reading scenario: %w", err)
	}
	var s struct {
		Judge struct {
			Checks judgeChecks `json:"checks"`
		} `json:"judge"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return judgeChecks{}, fmt.Errorf("parsing scenario %s: %w", path, err)
	}
	return s.Judge.Checks, nil
}
