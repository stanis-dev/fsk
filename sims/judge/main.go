// Command judge checks a fiskaly integration against a per-scenario rubric. It
// runs a deterministic checks gate first (trajectory-derived signals from the
// run dir) and, when that passes and -expect is set, an LLM expectation layer.
// The gate is hard: any failing check marks the run NON-COMPLIANT and skips the
// LLM entirely.
//
// Usage: judge -scenario <path> -run <runDir> [-expect] [-json <out>]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// checksReport is the structured outcome of the deterministic gate layer.
type checksReport struct {
	Passed  bool          `json:"passed"`
	Results []checkResult `json:"results"`
}

// judgeReport is the structured verdict written to judge.json. The process exit
// code remains the authoritative pass/fail signal.
type judgeReport struct {
	Scenario     string        `json:"scenario"`
	Verdict      string        `json:"verdict"`
	Checks       checksReport  `json:"checks"`
	Expectations *rubricReport `json:"expectations"`
	Note         string        `json:"note"`
}

func buildReport(scenario string, checks checksReport, rep *rubricReport, verdict string) judgeReport {
	var r judgeReport
	r.Scenario = scenario
	r.Checks = checks
	r.Expectations = rep
	r.Verdict = verdict
	r.Note = "LLM expectation layer is nondeterministic; conformance requires all deterministic checks to pass AND every expectation to be a cited MET"
	return r
}

func renderExpectations(rep rubricReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nEXPECTATIONS (model: %s)\n", rep.Model)
	for _, v := range rep.Criteria {
		fmt.Fprintf(&b, "%-13s %s\n", v.Verdict, v.ID)
		if v.Reasoning != "" {
			fmt.Fprintf(&b, "      %s\n", v.Reasoning)
		}
		if v.EvidenceQuote != "" {
			fmt.Fprintf(&b, "      evidence: %s\n", v.EvidenceQuote)
		}
		if v.Cite != "" {
			fmt.Fprintf(&b, "      cite: %s\n", v.Cite)
		}
	}
	return b.String()
}

func main() {
	var (
		scenarioFlag = flag.String("scenario", "", "path to a scenario.json (required)")
		runFlag      = flag.String("run", "", "path to a run dir containing transcript.jsonl (required)")
		expectFlag   = flag.Bool("expect", false, "after the gate passes, run the LLM expectation layer (requires the scenario to declare judge.expectations and the claude CLI)")
		jsonFlag     = flag.String("json", "", "write the structured verdict to this path as JSON")
	)
	flag.Parse()

	if *scenarioFlag == "" {
		fmt.Fprintln(os.Stderr, "judge: -scenario is required")
		os.Exit(2)
	}
	if *runFlag == "" {
		fmt.Fprintln(os.Stderr, "judge: -run is required")
		os.Exit(2)
	}

	scenarioName := filepath.Base(filepath.Dir(*scenarioFlag))

	traj, err := parseTrajectory(*runFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
		os.Exit(2)
	}

	checks, err := parseScenarioChecks(*scenarioFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
		os.Exit(2)
	}

	results := runChecks(checks, traj)
	for _, r := range results {
		status := "PASS"
		if !r.Pass {
			status = "FAIL"
		}
		fmt.Printf("%-4s  %-30s %s\n", status, r.ID, r.Detail)
	}

	gatePassed := checksPassed(results)
	cr := checksReport{Passed: gatePassed, Results: results}

	if !gatePassed {
		fmt.Println("VERDICT: NON-COMPLIANT (gate). exit 1")
		writeReport(*jsonFlag, buildReport(scenarioName, cr, nil, "NON-COMPLIANT"))
		os.Exit(1)
	}

	verdict := "conformant"
	exitCode := 0
	var rep *rubricReport
	var exps []expectation

	if *expectFlag {
		exps, err = expectationsFromScenario(*scenarioFlag)
		if err != nil {
			failInfra(*jsonFlag, scenarioName, cr, err)
		}
		if len(exps) > 0 {
			dir := *runFlag
			raw, err := readSourceRaw(dir)
			if err != nil {
				failInfra(*jsonFlag, scenarioName, cr, err)
			}
			r, err := runExpectations(traj, raw, stripCommentsKeepLayout(raw), exps, claudeModel, judgeModelID)
			if err != nil {
				failInfra(*jsonFlag, scenarioName, cr, fmt.Errorf("expectation layer: %w", err))
			}
			rep = &r
			fmt.Print(renderExpectations(r))
			if !conformant(r.Criteria) {
				verdict = "NON-COMPLIANT"
				exitCode = 1
			}
		}
	}

	// Config guard: a scenario that asserts nothing is a misconfiguration.
	if len(results) == 0 && len(exps) == 0 {
		failInfra(*jsonFlag, scenarioName, cr, fmt.Errorf("scenario declares neither checks nor expectations"))
	}

	if exitCode == 0 {
		fmt.Println("VERDICT: conformant. exit 0")
	} else {
		fmt.Println("VERDICT: NON-COMPLIANT (expectations). exit 1")
	}
	writeReport(*jsonFlag, buildReport(scenarioName, cr, rep, verdict))
	os.Exit(exitCode)
}

// failInfra reports a checks/expectation-layer infrastructure error: it writes a
// NON-COMPLIANT judge.json and exits 2. Conservative: an infra failure cannot
// certify conformance.
func failInfra(jsonPath, scenario string, cr checksReport, err error) {
	fmt.Fprintln(os.Stderr, "judge:", err)
	rep := buildReport(scenario, cr, nil, "NON-COMPLIANT")
	rep.Note = "infra error (no verdict computed): " + err.Error()
	writeReport(jsonPath, rep)
	os.Exit(2)
}

// writeReport marshals the structured verdict to path (no-op when path is empty).
// A write failure is loud (exit 2).
func writeReport(path string, report judgeReport) {
	if path == "" {
		return
	}
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

// readSource concatenates non-test Go source under dir, with comments stripped.
// Tests are excluded so a mock that mimics an invented API cannot satisfy a rule;
// comments are excluded so rules match the code an integration actually runs.
func readSource(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString(stripComments(data))
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}

// readSourceRaw concatenates non-test Go source under dir with comments retained,
// for the LLM expectation layer (the model reasons over comments; the citation
// check later validates evidence against the comment-stripped source). Tests are
// still excluded.
func readSourceRaw(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(data)
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}

// stripCommentsKeepLayout removes comment spans from src while preserving the
// rest of the code byte-for-byte. Unlike stripComments (which re-emits
// space-separated tokens for regex gates), this keeps verbatim code intact so
// the expectation layer's citation check can match an evidence_quote the model
// copied from the real source, while still excluding comment text.
func stripCommentsKeepLayout(src string) string {
	// Normalize line endings first: go/scanner drops lone \r from a COMMENT
	// literal, so start+len(lit) would undercount the span and leak trailing
	// comment bytes (a CR-padded comment could otherwise smuggle text into the
	// citation source — the input is untrusted).
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, []byte(src), nil, scanner.ScanComments)
	type span struct{ start, end int }
	var spans []span
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			start := fset.Position(pos).Offset
			spans = append(spans, span{start, start + len(lit)})
		}
	}
	if len(spans) == 0 {
		return src
	}
	var b strings.Builder
	prev := 0
	for _, sp := range spans {
		if sp.start < prev || sp.end > len(src) {
			continue
		}
		b.WriteString(src[prev:sp.start])
		b.WriteByte(' ') // keep tokens from gluing across a removed comment
		prev = sp.end
	}
	b.WriteString(src[prev:])
	return b.String()
}

// stripComments returns the Go source with comment tokens removed. It lexes with
// go/scanner so string literals are preserved intact. Falls back to the raw
// bytes if scanning yields nothing.
func stripComments(src []byte) string {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, src, nil, 0)
	var b strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if lit != "" {
			b.WriteString(lit)
		} else {
			b.WriteString(tok.String())
		}
		b.WriteByte(' ')
	}
	if b.Len() == 0 {
		return string(src)
	}
	return b.String()
}
