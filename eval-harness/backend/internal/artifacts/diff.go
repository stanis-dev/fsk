package artifacts

import "strings"

// ClassifyDiff classifies each line of a unified diff by its leading marker.
// Marker order matters: +++/--- are classified as meta before the +/- checks.
func ClassifyDiff(raw string) []DiffLine {
	if strings.TrimSpace(raw) == "" {
		return []DiffLine{}
	}
	lines := strings.Split(raw, "\n")
	out := make([]DiffLine, len(lines))
	for i, text := range lines {
		cls := "ctx"
		switch {
		case strings.HasPrefix(text, "diff "),
			strings.HasPrefix(text, "index "),
			strings.HasPrefix(text, "+++"),
			strings.HasPrefix(text, "---"):
			cls = "meta"
		case strings.HasPrefix(text, "@@"):
			cls = "hunk"
		case strings.HasPrefix(text, "+"):
			cls = "add"
		case strings.HasPrefix(text, "-"):
			cls = "del"
		}
		out[i] = DiffLine{Cls: cls, Text: text}
	}
	return out
}
