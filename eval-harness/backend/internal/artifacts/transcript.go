package artifacts

import (
	"bufio"
	"encoding/json"
	"strings"
)

// ParseTranscript parses a JSONL transcript into typed display events.
func ParseTranscript(jsonl string) []TranscriptEvent {
	events := []TranscriptEvent{}
	sc := bufio.NewScanner(strings.NewReader(jsonl))
	sc.Buffer(make([]byte, 16*1024*1024), 16*1024*1024)
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			continue
		}
		var typ string
		if err := json.Unmarshal(m["type"], &typ); err != nil {
			continue
		}
		switch typ {
		case "assistant":
			for _, c := range messageContent(m) {
				switch c.Type {
				case "thinking":
					if t := strings.TrimSpace(c.Thinking); t != "" {
						events = append(events, TranscriptEvent{Kind: "thinking", Text: t})
					}
				case "text":
					if t := strings.TrimSpace(c.Text); t != "" {
						events = append(events, TranscriptEvent{Kind: "assistant", Text: t})
					}
				case "tool_use":
					events = append(events, TranscriptEvent{Kind: "tool", Text: summarizeTool(c.Name, c.Input)})
				}
			}
		case "user":
			for _, c := range messageContent(m) {
				if c.Type == "tool_result" {
					txt := flattenContent(c.Content)
					if c.IsError {
						txt = "error: " + txt
					}
					events = append(events, TranscriptEvent{Kind: "result", Text: truncate(txt, 600)})
				}
			}
		case "result":
			var result string
			if raw, ok := m["result"]; ok {
				_ = json.Unmarshal(raw, &result)
			}
			if result != "" {
				events = append(events, TranscriptEvent{Kind: "final", Text: result})
			}
		}
	}
	return events
}

type contentItem struct {
	Type     string
	Thinking string
	Text     string
	Name     string
	Input    map[string]any
	IsError  bool
	Content  json.RawMessage
}

func messageContent(m map[string]json.RawMessage) []contentItem {
	var msg struct {
		Content []json.RawMessage `json:"content"`
	}
	if raw, ok := m["message"]; !ok {
		return nil
	} else if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	var out []contentItem
	for _, raw := range msg.Content {
		var c struct {
			Type     string          `json:"type"`
			Thinking string          `json:"thinking"`
			Text     string          `json:"text"`
			Name     string          `json:"name"`
			Input    map[string]any  `json:"input"`
			IsError  bool            `json:"is_error"`
			Content  json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		out = append(out, contentItem{
			Type:     c.Type,
			Thinking: c.Thinking,
			Text:     c.Text,
			Name:     c.Name,
			Input:    c.Input,
			IsError:  c.IsError,
			Content:  c.Content,
		})
	}
	return out
}

func flattenContent(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var b strings.Builder
		for _, e := range arr {
			b.WriteString(e.Text)
		}
		return b.String()
	}
	return ""
}

// SummarizeTool produces a short display string for a tool invocation.
func SummarizeTool(name string, input map[string]any) string {
	return summarizeTool(name, input)
}

func summarizeTool(name string, input map[string]any) string {
	sv := func(k string) string {
		if v, ok := input[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	switch name {
	case "Bash":
		return "Bash  $ " + oneLine(sv("command"))
	case "Read":
		return "Read  " + sv("file_path")
	case "Write":
		return "Write  " + sv("file_path")
	case "Edit", "MultiEdit":
		return name + "  " + sv("file_path")
	case "Grep":
		p := sv("pattern")
		if path := sv("path"); path != "" {
			p += "  in " + path
		}
		return "Grep  " + p
	case "Glob":
		return "Glob  " + sv("pattern")
	case "TodoWrite":
		return "TodoWrite  (updated task list)"
	case "WebFetch":
		return "WebFetch  " + sv("url")
	case "WebSearch":
		return "WebSearch  " + sv("query")
	case "Task", "Agent":
		desc := sv("description")
		if desc == "" {
			desc = sv("subagent_type")
		}
		return name + "  " + desc
	case "ToolSearch":
		return "ToolSearch  " + sv("query")
	default:
		b, _ := json.Marshal(input)
		return name + "  " + truncate(oneLine(string(b)), 200)
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + " …"
	}
	return s
}

func oneLine(s string) string {
	return truncate(strings.Join(strings.Fields(s), " "), 300)
}
