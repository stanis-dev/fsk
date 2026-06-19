package orchestrator

import "testing"

func TestCheckBinaries(t *testing.T) {
	if err := checkBinaries("go", "git"); err != nil {
		t.Errorf("go and git should be present: %v", err)
	}
	if err := checkBinaries("definitely-not-a-real-binary-xyz"); err == nil {
		t.Error("expected error for a missing binary")
	}
}
