package fiskaly

import (
	"encoding/json"
	"fmt"
)

var secretFields = map[string]bool{
	"bearer":   true,
	"secret":   true,
	"password": true,
	"pin":      true,
	"key":      true,
}

// redactSecrets replaces credential values in a JSON body so the audit
// trail (CallRecord) can be persisted and shared safely.
func redactSecrets(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return json.RawMessage(`"<unparseable>"`)
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil {
		return json.RawMessage(`"<unparseable>"`)
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok && secretFields[k] {
				t[k] = fmt.Sprintf("<redacted:%d chars>", len(s))
				continue
			}
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	default:
		return v
	}
}
