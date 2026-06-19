package artifacts

import (
	"strings"
	"testing"
)

func TestClassifyDiffByLeadingMarker(t *testing.T) {
	raw := strings.Join([]string{"diff --git a/x b/x", "@@ -1 +1 @@", "+added", "-removed", " context"}, "\n")
	lines := ClassifyDiff(raw)
	want := []string{"meta", "hunk", "add", "del", "ctx"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, l := range lines {
		if l.Cls != want[i] {
			t.Errorf("line %d: got cls %q, want %q", i, l.Cls, want[i])
		}
	}
}

func TestClassifyDiffMetaMarkers(t *testing.T) {
	if got := ClassifyDiff("--- a/x"); len(got) == 0 || got[0].Cls != "meta" {
		t.Errorf("--- a/x: got %v", got)
	}
	if got := ClassifyDiff("+++ b/x"); len(got) == 0 || got[0].Cls != "meta" {
		t.Errorf("+++ b/x: got %v", got)
	}
}

func TestClassifyDiffEmpty(t *testing.T) {
	if got := ClassifyDiff("   "); len(got) != 0 {
		t.Errorf("empty diff: got %v", got)
	}
}
