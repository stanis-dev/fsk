package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// filterScenarios selects scenarios by id, accepting an exact id or a numeric
// prefix (e.g. "06" matches "06-fire-and-forget"). An id matching nothing is an
// error so a typo fails loudly.
func filterScenarios(all []scenario, ids []string) ([]scenario, error) {
	var out []scenario
	for _, want := range ids {
		matched := false
		for _, s := range all {
			if s.id == want || strings.HasPrefix(s.id, want+"-") {
				out = append(out, s)
				matched = true
			}
		}
		if !matched {
			return nil, fmt.Errorf("no scenario matches %q", want)
		}
	}
	return out, nil
}

// runAll runs each scenario through the single path independently and returns 0
// only if all completed without a preflight violation or harness error. An
// agent failure is recorded in claude.err, not counted as a failure here.
func runAll(scenarios []scenario, runsBase, judgeBin string, ag agent, cfg runConfig, w io.Writer) int {
	failed := 0
	for _, s := range scenarios {
		res, err := runScenario(s, runsBase, judgeBin, ag, cfg)
		if err != nil {
			fmt.Fprintf(w, "%-22s ERROR: %v\n", s.id, err)
			failed++
			continue
		}
		if res.preflightViolated {
			fmt.Fprintf(w, "%-22s PREFLIGHT VIOLATED (seed not build PASS/tests PASS/judge NON-COMPLIANT)\n", s.id)
			failed++
			continue
		}
		fmt.Fprintf(w, "%-22s run=%s judge=%s grounded=%v\n",
			s.id, filepath.Base(res.runDir), verdict(res.obs.Judge.OK), res.obs.groundedOK)
	}
	total := len(scenarios)
	fmt.Fprintln(w)
	if failed == 0 {
		fmt.Fprintf(w, "%d/%d scenarios ran.\n", total, total)
		return 0
	}
	fmt.Fprintf(w, "%d/%d scenarios ran; %d failed before eval.\n", total-failed, total, failed)
	return 1
}
