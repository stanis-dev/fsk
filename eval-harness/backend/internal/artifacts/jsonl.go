package artifacts

import (
	"bufio"
	"encoding/json"
	"strings"
)

// scanJSONL decodes each non-empty line of jsonl into a raw object map and calls
// fn with it; blank lines and lines that are not a JSON object are skipped.
func scanJSONL(jsonl string, fn func(map[string]json.RawMessage)) {
	sc := bufio.NewScanner(strings.NewReader(jsonl))
	sc.Buffer(make([]byte, 16*1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		fn(m)
	}
}
