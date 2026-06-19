// Command judge checks a fiskaly integration against a per-scenario rubric. It
// runs a deterministic checks gate first (trajectory-derived signals from the
// run dir) and, when that passes and -expect is set, an LLM expectation layer.
// The gate is hard: any failing check marks the run NON-COMPLIANT and skips the
// LLM entirely.
//
// Usage: judge -scenario <path> [-run <runDir>] [-expect] [-json <out>] <integrationDir>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"backend/internal/judge"
)

func main() {
	var (
		scenarioFlag = flag.String("scenario", "", "path to a scenario.json (required)")
		runFlag      = flag.String("run", "", "path to a run dir with transcript.jsonl + mcp-telemetry.jsonl; omit for source-only evaluation")
		expectFlag   = flag.Bool("expect", false, "after the gate passes, run the LLM expectation layer (requires the claude CLI)")
		jsonFlag     = flag.String("json", "", "write the structured verdict to this path as JSON")
	)
	flag.Parse()

	if *scenarioFlag == "" {
		fmt.Fprintln(os.Stderr, "judge: -scenario is required")
		os.Exit(2)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "judge: missing integration dir")
		os.Exit(2)
	}

	report, code, err := judge.Evaluate(judge.Options{
		ScenarioPath:   *scenarioFlag,
		RunDir:         *runFlag,
		IntegrationDir: flag.Arg(0),
		Expect:         *expectFlag,
	}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
	}
	if *jsonFlag != "" {
		writeReport(*jsonFlag, report)
	}
	os.Exit(code)
}

func writeReport(path string, report judge.Report) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge: marshaling report:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "judge: writing report:", err)
		os.Exit(2)
	}
}
