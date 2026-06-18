package main

import "testing"

func traj() Trajectory {
	return Trajectory{
		ToolUses: []string{"search_fiskaly_docs", "Edit", "search_fiskaly_docs"},
		Telemetry: []telemetryEntry{
			{Tool: "search_fiskaly_docs", Args: map[string]any{"query": "records receipt"}, IsError: false},
			{Tool: "fetch_fiskaly_doc", Args: map[string]any{"id": "tokens"}, IsError: true},
		},
	}
}

func resByID(rs []checkResult, id string) checkResult {
	for _, r := range rs {
		if r.ID == id {
			return r
		}
	}
	return checkResult{ID: id, Pass: false, Detail: "MISSING"}
}

func TestRunChecks(t *testing.T) {
	max := 0
	c := judgeChecks{
		GroundedBeforeWrite: true,
		ToolsCalled:         []toolReq{{Name: "search_fiskaly_docs", Min: 2}, {Name: "Bash", Min: 1}},
		DocsFetched:         []string{"records", "tokens", "missing-doc"},
		MaxMcpErrors:        &max,
	}
	rs := runChecks(c, traj())

	if !resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("grounded should pass (search at 0 < edit at 1)")
	}
	if !resByID(rs, "toolsCalled:search_fiskaly_docs").Pass {
		t.Error("search called 2x should meet min 2")
	}
	if resByID(rs, "toolsCalled:Bash").Pass {
		t.Error("Bash never called should fail")
	}
	if !resByID(rs, "docsFetched:records").Pass || !resByID(rs, "docsFetched:tokens").Pass {
		t.Error("records + tokens are in telemetry args")
	}
	if resByID(rs, "docsFetched:missing-doc").Pass {
		t.Error("missing-doc not fetched should fail")
	}
	if resByID(rs, "maxMcpErrors").Pass {
		t.Error("1 error > max 0 should fail")
	}
	if checksPassed(rs) {
		t.Error("overall should fail (Bash, missing-doc, maxMcpErrors failed)")
	}
}

func TestRunChecks_EmptyConfigPasses(t *testing.T) {
	rs := runChecks(judgeChecks{}, Trajectory{})
	if !checksPassed(rs) {
		t.Errorf("no configured checks → vacuously passes, got %+v", rs)
	}
}

func TestGroundedFailsWhenWriteFirst(t *testing.T) {
	tr := Trajectory{ToolUses: []string{"Write", "search_fiskaly_docs"}}
	rs := runChecks(judgeChecks{GroundedBeforeWrite: true}, tr)
	if resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("write before search should fail grounded")
	}
}

func TestMCPPrefixedToolNames(t *testing.T) {
	tr := Trajectory{
		ToolUses: []string{"mcp__fiskaly__search_fiskaly_docs", "Edit"},
	}
	c := judgeChecks{
		GroundedBeforeWrite: true,
		ToolsCalled:         []toolReq{{Name: "search_fiskaly_docs", Min: 1}},
	}
	rs := runChecks(c, tr)
	if !resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("mcp-prefixed search before Edit should pass groundedBeforeWrite")
	}
	if !resByID(rs, "toolsCalled:search_fiskaly_docs").Pass {
		t.Error("mcp-prefixed search_fiskaly_docs should satisfy toolsCalled min:1")
	}
}
