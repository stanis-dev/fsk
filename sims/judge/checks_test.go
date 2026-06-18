package main

import "testing"

func traj() Trajectory {
	return Trajectory{
		ToolUses: []string{"search_fiskaly_docs", "Edit", "search_fiskaly_docs"},
		Telemetry: []telemetryEntry{
			{Tool: "search_fiskaly_docs", Args: map[string]any{"query": "records receipt"}, IsError: false},
			{Tool: "fetch_fiskaly_doc", Args: map[string]any{"id": "probe:records-flow"}, IsError: true},
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
		DocsFetched:         []string{"probe:records-flow", "probe:auth-and-headers", "missing-doc"},
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
	// probe:records-flow was fetched (exact id match); probe:auth-and-headers was not
	if !resByID(rs, "docsFetched:probe:records-flow").Pass {
		t.Error("probe:records-flow fetched doc id should pass")
	}
	if resByID(rs, "docsFetched:probe:auth-and-headers").Pass {
		t.Error("probe:auth-and-headers not fetched should fail")
	}
	if resByID(rs, "docsFetched:missing-doc").Pass {
		t.Error("missing-doc not fetched should fail")
	}
	if resByID(rs, "maxMcpErrors").Pass {
		t.Error("1 error > max 0 should fail")
	}
	if checksPassed(rs) {
		t.Error("overall should fail (Bash, probe:auth-and-headers, missing-doc, maxMcpErrors failed)")
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
		Telemetry: []telemetryEntry{
			{Tool: "mcp__fiskaly__fetch_fiskaly_doc", Args: map[string]any{"id": "probe:records-flow"}, IsError: false},
		},
	}
	c := judgeChecks{
		GroundedBeforeWrite: true,
		ToolsCalled:         []toolReq{{Name: "search_fiskaly_docs", Min: 1}},
		DocsFetched:         []string{"probe:records-flow"},
	}
	rs := runChecks(c, tr)
	if !resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("mcp-prefixed search before Edit should pass groundedBeforeWrite")
	}
	if !resByID(rs, "toolsCalled:search_fiskaly_docs").Pass {
		t.Error("mcp-prefixed search_fiskaly_docs should satisfy toolsCalled min:1")
	}
	if !resByID(rs, "docsFetched:probe:records-flow").Pass {
		t.Error("mcp-prefixed fetch_fiskaly_doc should satisfy docsFetched")
	}
}

func TestGroundedPassesWithSearchNoWrite(t *testing.T) {
	tr := Trajectory{ToolUses: []string{"search_fiskaly_docs"}}
	rs := runChecks(judgeChecks{GroundedBeforeWrite: true}, tr)
	if !resByID(rs, "groundedBeforeWrite").Pass {
		t.Error("search with no write tool should pass groundedBeforeWrite")
	}
}

func TestToolsCalledMinZeroTreatedAsOne(t *testing.T) {
	tr := Trajectory{ToolUses: []string{}}
	c := judgeChecks{ToolsCalled: []toolReq{{Name: "search_fiskaly_docs", Min: 0}}}
	rs := runChecks(c, tr)
	if resByID(rs, "toolsCalled:search_fiskaly_docs").Pass {
		t.Error("min:0 should be treated as min:1; tool never called should fail")
	}
}
