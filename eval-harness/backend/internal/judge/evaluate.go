package judge

import (
	"fmt"
	"io"
	"path/filepath"
)

// Options is the fully-resolved input to one judge evaluation.
type Options struct {
	ScenarioPath   string // path to a scenario.json
	RunDir         string // run dir with transcript.jsonl + mcp-telemetry.jsonl; "" = source-only (skip the trajectory gate)
	IntegrationDir string // root of the integration source under review
	Expect         bool   // after the gate passes, run the LLM expectation layer
}

// Evaluate runs the deterministic checks gate and, when Expect is set and the
// scenario declares expectations, the LLM expectation layer. It writes the
// human-readable report to out and returns the structured report plus the
// process exit code: 0 conformant, 1 NON-COMPLIANT, 2 infra error. The returned
// error is non-nil iff the code is 2.
func Evaluate(opts Options, out io.Writer) (Report, int, error) {
	scenarioName := filepath.Base(filepath.Dir(opts.ScenarioPath))

	var traj trajectory
	var results []checkResult
	gatePassed := true
	if opts.RunDir != "" {
		var err error
		traj, err = parseTrajectory(opts.RunDir)
		if err != nil {
			return infra(scenarioName, checksReport{}, err)
		}
		checks, err := parseScenarioChecks(opts.ScenarioPath)
		if err != nil {
			return infra(scenarioName, checksReport{}, err)
		}
		results = runChecks(checks, traj)
		for _, r := range results {
			status := "PASS"
			if !r.Pass {
				status = "FAIL"
			}
			fmt.Fprintf(out, "%-4s  %-30s %s\n", status, r.ID, r.Detail)
		}
		gatePassed = checksPassed(results)
	}

	cr := checksReport{Passed: gatePassed, Results: results}

	if !gatePassed {
		fmt.Fprintln(out, "VERDICT: NON-COMPLIANT (gate). exit 1")
		return buildReport(scenarioName, cr, nil, "NON-COMPLIANT"), 1, nil
	}

	verdict := "conformant"
	exitCode := 0
	var rep *rubricReport
	var exps []expectation

	if opts.Expect {
		var err error
		exps, err = expectationsFromScenario(opts.ScenarioPath)
		if err != nil {
			return infra(scenarioName, cr, err)
		}
		if len(exps) > 0 {
			raw, err := readSourceRaw(opts.IntegrationDir)
			if err != nil {
				return infra(scenarioName, cr, err)
			}
			r, err := runExpectations(traj, raw, stripCommentsKeepLayout(raw), exps, claudeModel, rubricModelID)
			if err != nil {
				return infra(scenarioName, cr, fmt.Errorf("expectation layer: %w", err))
			}
			rep = &r
			fmt.Fprint(out, renderExpectations(r))
			if !conformant(r.Criteria) {
				verdict = "NON-COMPLIANT"
				exitCode = 1
			}
		}
	}

	if len(results) == 0 && len(exps) == 0 {
		return infra(scenarioName, cr, fmt.Errorf("scenario declares neither checks nor expectations"))
	}

	if exitCode == 0 {
		fmt.Fprintln(out, "VERDICT: conformant. exit 0")
	} else {
		fmt.Fprintln(out, "VERDICT: NON-COMPLIANT (expectations). exit 1")
	}
	return buildReport(scenarioName, cr, rep, verdict), exitCode, nil
}

// infra builds the shared infra-error result (exit 2): a NON-COMPLIANT report
// whose Note records that no verdict was computed.
func infra(scenario string, cr checksReport, err error) (Report, int, error) {
	rep := buildReport(scenario, cr, nil, "NON-COMPLIANT")
	rep.Note = "infra error (no verdict computed): " + err.Error()
	return rep, 2, err
}
