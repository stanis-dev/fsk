package main

import (
	"fmt"
	"strings"
)

type toolReq struct {
	Name string `json:"name"`
	Min  int    `json:"min"`
}

// judgeChecks is the deterministic gate declared in scenario.json judge.checks.
// All fields are optional; an unset field asserts nothing.
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

func runChecks(c judgeChecks, t Trajectory) []checkResult {
	var out []checkResult

	if c.GroundedBeforeWrite {
		searchAt := indexOf(t.ToolUses, func(n string) bool { return n == "search_fiskaly_docs" })
		writeAt := indexOf(t.ToolUses, func(n string) bool { return writeTools[n] })
		out = append(out, groundedResult(searchAt, writeAt))
	}

	for _, req := range c.ToolsCalled {
		min := req.Min
		if min < 1 {
			min = 1
		}
		got := countOccurrences(t.ToolUses, req.Name)
		out = append(out, checkResult{
			ID:     "toolsCalled:" + req.Name,
			Pass:   got >= min,
			Detail: fmt.Sprintf("called %dx (min %d)", got, min),
		})
	}

	if len(c.DocsFetched) > 0 {
		hay := telemetryArgsText(t.Telemetry)
		for _, want := range c.DocsFetched {
			ok := strings.Contains(hay, want)
			out = append(out, checkResult{
				ID:     "docsFetched:" + want,
				Pass:   ok,
				Detail: ternary(ok, "found in fetched docs/queries", "not found in any MCP call args"),
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

func indexOf(xs []string, pred func(string) bool) int {
	for i, x := range xs {
		if pred(x) {
			return i
		}
	}
	return -1
}

func countOccurrences(xs []string, name string) int {
	n := 0
	for _, x := range xs {
		if x == name {
			n++
		}
	}
	return n
}

func telemetryArgsText(tel []telemetryEntry) string {
	var b strings.Builder
	for _, e := range tel {
		for _, v := range e.Args {
			fmt.Fprintf(&b, "%v ", v)
		}
	}
	return b.String()
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
