package judge

import (
	"encoding/json"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type checksReport struct {
	Passed  bool          `json:"passed"`
	Results []checkResult `json:"results"`
}

// Report is written to judge.json; the process exit code stays the
// authoritative pass/fail signal.
type Report struct {
	Scenario     string        `json:"scenario"`
	Verdict      string        `json:"verdict"`
	Checks       checksReport  `json:"checks"`
	Expectations *rubricReport `json:"expectations"`
	Note         string        `json:"note"`
}

// WriteReport marshals r as indented JSON (with a trailing newline) to path.
func WriteReport(path string, r Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func buildReport(scenario string, checks checksReport, rep *rubricReport, verdict string) Report {
	var r Report
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
	}
	return b.String()
}

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

func stripCommentsKeepLayout(src string) string {
	// Normalize first; go/scanner drops lone \r from COMMENT literals.
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
