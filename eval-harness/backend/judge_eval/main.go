// Command judge_eval runs the expectation judge against gold fixtures.
//
// Usage: go run ./judge_eval   (from eval-harness/backend)
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"backend/internal/judge"
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

func unmetCount(r judge.Report) int {
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
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "locating judge_eval source file")
		os.Exit(2)
	}
	evalDir := filepath.Dir(thisFile)   // .../backend/judge_eval
	backendDir := filepath.Dir(evalDir) // .../backend
	scenariosDir := filepath.Join(backendDir, "scenarios")
	goldDir := filepath.Join(backendDir, "cmd", "judge", "testdata", "goldset")

	// matrix[expected][actual]; index 0 = conformant, 1 = NON-COMPLIANT.
	var matrix [2][2]int
	falsePass, falseFail, abstentionOnly, errs := 0, 0, 0, 0

	fmt.Printf("meta-eval: %d gold fixtures (good reps=1, bad reps=3)\n\n", len(cases))
	for _, c := range cases {
		scenario := filepath.Join(scenariosDir, c.scenario, "scenario.json")
		work := filepath.Join(goldDir, c.scenario, c.variant)
		reps := 1
		if !c.expectConformant {
			reps = 3
		}
		for r := 0; r < reps; r++ {
			rep, code, err := judge.Evaluate(judge.Options{
				ScenarioPath:   scenario,
				IntegrationDir: work,
				Expect:         true,
			}, io.Discard)
			if code == 2 {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (exit 2: %v)\n", c.scenario, c.variant, r, err)
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
