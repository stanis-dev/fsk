package orchestrator

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"backend/internal/scenarios"
)

// filterScenarios selects scenarios by id, accepting an exact id or a numeric
// prefix (e.g. "06" matches "06-fire-and-forget"). An id matching nothing is an
// error so a typo fails loudly.
func filterScenarios(all []scenarios.Scenario, ids []string) ([]scenarios.Scenario, error) {
	var out []scenarios.Scenario
	for _, want := range ids {
		matched := false
		for _, s := range all {
			if s.ID == want || strings.HasPrefix(s.ID, want+"-") {
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
// only if all completed without a harness error. An
// agent failure is recorded in claude.err, not counted as a failure here.
func runAll(ctx context.Context, ss []scenarios.Scenario, runsBase, judgeBin string, ag agent, cfg runConfig, detached bool, w io.Writer) int {
	failed := 0
	for _, s := range ss {
		res, err := runScenario(ctx, s, runsBase, judgeBin, ag, cfg, detached)
		if err != nil {
			fmt.Fprintf(w, "%-22s ERROR: %v\n", s.ID, err)
			failed++
			continue
		}
		judgeVerdict := "NON-COMPLIANT"
		if res.obs.Judge.OK {
			judgeVerdict = "conformant"
		}
		fmt.Fprintf(w, "%-22s run=%s judge=%s\n",
			s.ID, filepath.Base(res.runDir), judgeVerdict)
	}
	total := len(ss)
	fmt.Fprintln(w)
	if failed == 0 {
		fmt.Fprintf(w, "%d/%d scenarios ran.\n", total, total)
		return 0
	}
	fmt.Fprintf(w, "%d/%d scenarios ran; %d failed.\n", total-failed, total, failed)
	return 1
}
