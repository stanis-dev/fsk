// Command judge_eval is the meta-evaluation for the expectation judge: it runs
// the judge (with -expect) over a gold set of integrations whose correct verdict
// is known, and gates on genuine separation — every good fixture conformant, every
// bad fixture caught by at least one UNMET expectation criterion (not mere
// abstention), and zero errors. Every gold fixture passes the deterministic gate
// by construction, so only the expectation layer can separate good from bad —
// this measures the expectation layer, not the gate.
//
// Requires the claude CLI to be authenticated (the expectation layer calls it).
// Run from anywhere: paths are resolved relative to this source file.
//
// Usage: go run ./judge_eval   (from sims/judge)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type goldCase struct {
	scenario         string
	variant          string // good | bad
	expectConformant bool
}

var cases = []goldCase{
	{"05-outage-resilience", "good", true},
	{"05-outage-resilience", "bad", false},
	{"07-wrong-vat", "good", true},
	{"07-wrong-vat", "bad", false},
	{"10-credential-expiry", "good", true},
	{"10-credential-expiry", "bad", false},
}

// evalReport mirrors the judge.json fields the meta-eval needs.
type evalReport struct {
	Verdict      string `json:"verdict"`
	Expectations *struct {
		Criteria []struct {
			Verdict string `json:"verdict"`
		} `json:"criteria"`
	} `json:"expectations"`
}

func unmetCount(r evalReport) int {
	if r.Expectations == nil {
		return 0
	}
	n := 0
	for _, c := range r.Expectations.Criteria {
		if c.Verdict == "UNMET" {
			n++
		}
	}
	return n
}

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	evalDir := filepath.Dir(thisFile)
	judgeDir := filepath.Dir(evalDir)
	simsDir := filepath.Dir(judgeDir)
	scenariosDir := filepath.Join(simsDir, "scenarios")
	goldDir := filepath.Join(judgeDir, "testdata", "goldset")

	bin := filepath.Join(os.TempDir(), "judge-eval-bin")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = judgeDir
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build judge: %v\n%s", err, out)
		os.Exit(2)
	}
	reportPath := filepath.Join(os.TempDir(), "judge-eval-report.json")

	// matrix[expected][actual]; index 0 = conformant, 1 = NON-COMPLIANT.
	var matrix [2][2]int
	falsePass, falseFail, abstentionOnly, errs := 0, 0, 0, 0

	fmt.Printf("meta-eval: %d gold fixtures (good reps=1, bad reps=3)\n\n", len(cases))
	for _, c := range cases {
		scenario := filepath.Join(scenariosDir, c.scenario, "scenario.json")
		work := filepath.Join(goldDir, c.scenario, c.variant)
		// Re-run bad fixtures more: a false-PASS is the cell we must keep empty.
		reps := 1
		if !c.expectConformant {
			reps = 3
		}
		for r := 0; r < reps; r++ {
			_ = os.Remove(reportPath)
			cmd := exec.Command(bin, "-expect", "-json", reportPath, "-scenario", scenario, "-run", work)
			out, _ := cmd.CombinedOutput()
			code := cmd.ProcessState.ExitCode()

			if code == 2 {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (exit 2)\n%s\n", c.scenario, c.variant, r, out)
				continue
			}

			var rep evalReport
			if data, err := os.ReadFile(reportPath); err != nil {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (no judge.json: %v)\n", c.scenario, c.variant, r, err)
				continue
			} else if err := json.Unmarshal(data, &rep); err != nil {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (bad judge.json: %v)\n", c.scenario, c.variant, r, err)
				continue
			}

			actualConformant := code == 0
			expectedIdx, actualIdx := 0, 0
			if !c.expectConformant {
				expectedIdx = 1
			}
			if !actualConformant {
				actualIdx = 1
			}
			matrix[expectedIdx][actualIdx]++

			label, mark := "conformant", "ok"
			if !actualConformant {
				label = "NON-COMPLIANT"
			}
			if c.variant == "bad" {
				switch {
				case actualConformant:
					falsePass++
					mark = "FALSE-PASS"
				case unmetCount(rep) == 0:
					abstentionOnly++
					mark = "caught-without-UNMET(abstention)"
				default:
					mark = fmt.Sprintf("caught(%d UNMET)", unmetCount(rep))
				}
			} else if !actualConformant {
				falseFail++
				mark = "FALSE-FAIL"
			}
			fmt.Printf("%-22s/%-4s rep%d -> %-13s [%s]\n", c.scenario, c.variant, r, label, mark)
		}
	}

	fmt.Printf("\nconfusion matrix (rows=expected, cols=actual):\n")
	fmt.Printf("                      actual:conformant   actual:NON-COMPLIANT\n")
	fmt.Printf("  expect:conformant         %3d                 %3d\n", matrix[0][0], matrix[0][1])
	fmt.Printf("  expect:NON-COMPLIANT      %3d                 %3d\n", matrix[1][0], matrix[1][1])
	fmt.Printf("\nfalse-PASS: %d   false-FAIL: %d   abstention-only catches: %d   errors: %d\n",
		falsePass, falseFail, abstentionOnly, errs)

	if falsePass > 0 || falseFail > 0 || abstentionOnly > 0 || errs > 0 {
		fmt.Println("\nMETA-EVAL FAILED")
		os.Exit(1)
	}
	fmt.Println("\nMETA-EVAL PASSED (good=conformant, bad=caught by active UNMET, zero false-PASS)")
}
