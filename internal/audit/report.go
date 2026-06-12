package audit

import (
	"fmt"
	"strings"
)

// Markdown renders the report for humans: the demo terminal, the MCP text
// content, and the interview memo all share this rendering.
func (r Report) Markdown() string {
	var b strings.Builder
	icon := "✅"
	if r.Verdict != "PASS" {
		icon = "❌"
	}
	fmt.Fprintf(&b, "# Compliance audit — %s %s\n\n", r.Verdict, icon)
	fmt.Fprintf(&b, "%d API calls audited · %d findings · %d rules passed\n\n", r.Calls, len(r.Findings), len(r.Passed))

	if len(r.Findings) > 0 {
		b.WriteString("## Findings\n\n")
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "### [%s] %s\n", f.Severity, f.Title)
			fmt.Fprintf(&b, "- **Rule**: `%s`\n", f.Rule)
			fmt.Fprintf(&b, "- **What happened**: %s\n", f.Detail)
			fmt.Fprintf(&b, "- **Why it matters**: %s\n", f.Citation)
			fmt.Fprintf(&b, "- **Fix**: %s\n\n", f.Remediation)
		}
	}
	if len(r.Passed) > 0 {
		b.WriteString("## Passed\n\n")
		for _, p := range r.Passed {
			fmt.Fprintf(&b, "- ✅ `%s`\n", p)
		}
	}
	return b.String()
}
