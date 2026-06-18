// Command judge_eval is the meta-evaluation for the rubric judge: it runs the
// judge (with -rubric) over a gold set of integrations whose correct verdict is
// known, and gates on zero false-PASS (a trap-fallen "bad" fixture judged
// conformant). Every gold fixture passes the deterministic gate by construction,
// so only the rubric layer can separate good from bad — this measures the rubric.
//
// Requires the claude CLI to be authenticated (the rubric layer calls it).
// Run from anywhere: paths are resolved relative to this source file.
//
// Usage: go run ./judge_eval   (from sims/judge)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// repsFor concentrates repetitions on the dangerous direction: a false-PASS on a
// bad fixture is the cell we must keep empty, so re-run bad fixtures more often.
func repsFor(c goldCase) int {
	if c.expectConformant {
		return 1
	}
	return 3
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

	// matrix[expected][actual]; index 0 = conformant, 1 = NON-COMPLIANT.
	var matrix [2][2]int
	falsePass, errs, uncited := 0, 0, 0

	fmt.Printf("meta-eval: %d gold fixtures (good reps=1, bad reps=3)\n\n", len(cases))
	for _, c := range cases {
		scenario := filepath.Join(scenariosDir, c.scenario, "scenario.json")
		work := filepath.Join(goldDir, c.scenario, c.variant)
		for r := 0; r < repsFor(c); r++ {
			cmd := exec.Command(bin, "-rubric", "-scenario", scenario, work)
			out, _ := cmd.CombinedOutput()
			code := cmd.ProcessState.ExitCode()

			if code == 2 {
				errs++
				fmt.Printf("ERROR  %-22s/%-4s rep%d (exit 2)\n%s\n", c.scenario, c.variant, r, out)
				continue
			}
			actualConformant := code == 0
			ei, ai := 0, 0
			if !c.expectConformant {
				ei = 1
			}
			if !actualConformant {
				ai = 1
			}
			matrix[ei][ai]++

			label, mark := "conformant", "ok"
			if !actualConformant {
				label = "NON-COMPLIANT"
			}
			if c.variant == "bad" {
				if actualConformant {
					falsePass++
					mark = "FALSE-PASS"
				} else if !strings.Contains(string(out), "UNMET") {
					uncited++
					mark = "caught-without-UNMET"
				} else {
					mark = "caught(UNMET)"
				}
			} else if !actualConformant {
				mark = "FALSE-FAIL"
			}
			fmt.Printf("%-22s/%-4s rep%d -> %-13s [%s]\n", c.scenario, c.variant, r, label, mark)
		}
	}

	fmt.Printf("\nconfusion matrix (rows=expected, cols=actual):\n")
	fmt.Printf("                      actual:conformant   actual:NON-COMPLIANT\n")
	fmt.Printf("  expect:conformant         %3d                 %3d\n", matrix[0][0], matrix[0][1])
	fmt.Printf("  expect:NON-COMPLIANT      %3d                 %3d\n", matrix[1][0], matrix[1][1])
	fmt.Printf("\nfalse-PASS (bad judged conformant): %d   errors: %d   caught-without-UNMET: %d\n", falsePass, errs, uncited)

	if falsePass > 0 || errs > 0 {
		fmt.Println("\nMETA-EVAL FAILED")
		os.Exit(1)
	}
	fmt.Println("\nMETA-EVAL PASSED (zero false-PASS)")
}
