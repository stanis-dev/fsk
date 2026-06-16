// Command dashboard is a minimal local UI for the fiskaly eval harness: list runs,
// inspect a run's session transcript and judge verdict, and trigger a new run.
//
// Usage (from the repo root): ./evals/dashboard.sh   then open http://localhost:8080
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	runsDir = flag.String("runs", filepath.Join(os.Getenv("HOME"), ".cache/fiskaly-eval"), "runs directory")
	script  = flag.String("script", "evals/run-eval-docker.sh", "path to the run script")
	addr    = flag.String("addr", ":8080", "listen address")
)

type summary struct {
	ID      string
	Created time.Time
	Status  string // running | done
	Coder   string
	Harness string
	Model   string
	Effort  string
	Build   string
	Tests   string
	Judge   string
	Turns   string
	Cost    string
}

func main() {
	flag.Parse()
	if abs, err := filepath.Abs(*script); err == nil {
		*script = abs
	}
	http.HandleFunc("/", handleList)
	http.HandleFunc("/run/", handleDetail)
	http.HandleFunc("/trigger", handleTrigger)
	fmt.Printf("dashboard: http://localhost%s   runs=%s   script=%s\n", *addr, *runsDir, *script)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func listRuns() []summary {
	dirs, _ := filepath.Glob(filepath.Join(*runsDir, "run.*"))
	var out []summary
	for _, d := range dirs {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, summarize(d, fi.ModTime()))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func summarize(dir string, created time.Time) summary {
	s := summary{ID: filepath.Base(dir), Created: created, Status: "running"}
	// Prefer the run log (the session's own init record) for model and harness;
	// meta.json is only a fallback. cwd=/work is the Docker container workdir.
	logModel, cwd, ccver := logInfo(filepath.Join(dir, "transcript.jsonl"))
	hMeta, cMeta, mMeta, eff := readMeta(dir)
	s.Effort = firstNonEmpty(eff, "—") // effort is a CLI flag, not in the session log; meta.json only
	s.Model = firstNonEmpty(logModel, mMeta)
	logCoder := ""
	if ccver != "" {
		logCoder = "claude-code" // the init record carries claude_code_version
	}
	s.Coder = firstNonEmpty(logCoder, cMeta, "?")
	switch {
	case cwd == "/work":
		s.Harness = "docker"
	case cwd != "":
		s.Harness = "local"
	default:
		s.Harness = firstNonEmpty(hMeta, "?")
	}

	judge := readFile(filepath.Join(dir, "judge.txt"))
	if judge == "" {
		return s // judge.txt is the last step; absent means still in progress
	}
	s.Status = "done"
	switch {
	case strings.Contains(judge, "conformant"):
		s.Judge = "PASS"
	case strings.Contains(judge, "NON-COMPLIANT"):
		s.Judge = "FAIL"
	}
	if strings.TrimSpace(readFile(filepath.Join(dir, "build.txt"))) == "" {
		s.Build = "PASS"
	} else {
		s.Build = "FAIL"
	}
	tt := readFile(filepath.Join(dir, "test.txt"))
	if tt != "" && !strings.Contains(tt, "FAIL") && strings.Contains(tt, "ok") {
		s.Tests = "PASS"
	} else {
		s.Tests = "FAIL"
	}
	s.Turns, s.Cost = parseResult(filepath.Join(dir, "transcript.jsonl"))
	return s
}

func parseResult(path string) (turns, cost string) {
	for _, b := range scan(path) {
		var m map[string]any
		if json.Unmarshal(b, &m) != nil || m["type"] != "result" {
			continue
		}
		if v, ok := m["num_turns"].(float64); ok {
			turns = fmt.Sprintf("%.0f", v)
		}
		if v, ok := m["total_cost_usd"].(float64); ok {
			cost = fmt.Sprintf("$%.2f", v)
		}
	}
	return
}

type event struct{ Kind, Text string }

func renderTranscript(path string) []event {
	var evs []event
	for _, b := range scan(path) {
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		switch m["type"] {
		case "assistant":
			for _, c := range content(m) {
				cm, _ := c.(map[string]any)
				switch cm["type"] {
				case "thinking":
					if t, _ := cm["thinking"].(string); strings.TrimSpace(t) != "" {
						evs = append(evs, event{"thinking", t})
					}
				case "text":
					if t, _ := cm["text"].(string); strings.TrimSpace(t) != "" {
						evs = append(evs, event{"assistant", t})
					}
				case "tool_use":
					name, _ := cm["name"].(string)
					inp, _ := cm["input"].(map[string]any)
					evs = append(evs, event{"tool", summarizeTool(name, inp)})
				}
			}
		case "user":
			for _, c := range content(m) {
				cm, _ := c.(map[string]any)
				if cm["type"] == "tool_result" {
					txt := flatten(cm["content"])
					if e, _ := cm["is_error"].(bool); e {
						txt = "error: " + txt
					}
					evs = append(evs, event{"result", truncate(txt, 600)})
				}
			}
		case "result":
			if t, _ := m["result"].(string); t != "" {
				evs = append(evs, event{"final", t})
			}
		}
	}
	return evs
}

func renderDiff(raw string) template.HTML {
	if strings.TrimSpace(raw) == "" {
		return template.HTML("—")
	}
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		cls := "ctx"
		switch {
		case strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			cls = "meta"
		case strings.HasPrefix(line, "@@"):
			cls = "hunk"
		case strings.HasPrefix(line, "+"):
			cls = "add"
		case strings.HasPrefix(line, "-"):
			cls = "del"
		}
		fmt.Fprintf(&b, `<span class="dl %s">%s</span>`+"\n", cls, template.HTMLEscapeString(line))
	}
	return template.HTML(b.String())
}

func handleList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl.ExecuteTemplate(w, "list", listRuns())
}

func handleDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/run/")
	if !strings.HasPrefix(id, "run.") || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		http.Error(w, "bad run id", http.StatusBadRequest)
		return
	}
	dir := filepath.Join(*runsDir, id)
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	data := struct {
		summary
		JudgeLog   string
		BuildLog   string
		TestLog    string
		Err        string
		Diff       template.HTML
		Transcript []event
	}{
		summary:    summarize(dir, fi.ModTime()),
		JudgeLog:   orDash(readFile(filepath.Join(dir, "judge.txt"))),
		BuildLog:   orDash(readFile(filepath.Join(dir, "build.txt"))),
		TestLog:    orDash(readFile(filepath.Join(dir, "test.txt"))),
		Err:        readFile(filepath.Join(dir, "claude.err")),
		Diff:       renderDiff(readFile(filepath.Join(dir, "changes.diff"))),
		Transcript: renderTranscript(filepath.Join(dir, "transcript.jsonl")),
	}
	tmpl.ExecuteTemplate(w, "detail", data)
}

func handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		logf, _ := os.Create(filepath.Join(*runsDir, "trigger.log"))
		cmd := exec.Command("bash", *script)
		if logf != nil {
			cmd.Stdout, cmd.Stderr = logf, logf
		}
		_ = cmd.Start() // async; the run appears in the list once its dir is created
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// helpers

func readMeta(dir string) (harness, coder, model, effort string) {
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return
	}
	var m struct{ Harness, Coder, Model, Effort string }
	_ = json.Unmarshal(b, &m)
	return m.Harness, m.Coder, m.Model, m.Effort
}

// logInfo reads the session's init record from the transcript: the model the run
// actually used and its working directory (which reveals the harness).
func logInfo(path string) (model, cwd, ccver string) {
	for _, b := range scan(path) {
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		if m["type"] == "system" {
			model, _ = m["model"].(string)
			cwd, _ = m["cwd"].(string)
			ccver, _ = m["claude_code_version"].(string)
			return
		}
	}
	return
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func scan(path string) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<25)
	for sc.Scan() {
		b := make([]byte, len(sc.Bytes()))
		copy(b, sc.Bytes())
		out = append(out, b)
	}
	return out
}

func content(m map[string]any) []any {
	msg, _ := m["message"].(map[string]any)
	c, _ := msg["content"].([]any)
	return c
}

func flatten(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, e := range t {
			if em, ok := e.(map[string]any); ok {
				if s, ok := em["text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		return b.String()
	}
	return ""
}

func sv(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}

func oneLine(s string) string {
	return truncate(strings.Join(strings.Fields(s), " "), 300)
}

// summarizeTool turns a tool call into a human-readable one-liner.
func summarizeTool(name string, in map[string]any) string {
	switch name {
	case "Bash":
		return "Bash  $ " + oneLine(sv(in, "command"))
	case "Read":
		return "Read  " + sv(in, "file_path")
	case "Write":
		return "Write  " + sv(in, "file_path")
	case "Edit", "MultiEdit":
		return name + "  " + sv(in, "file_path")
	case "Grep":
		p := sv(in, "pattern")
		if path := sv(in, "path"); path != "" {
			p += "  in " + path
		}
		return "Grep  " + p
	case "Glob":
		return "Glob  " + sv(in, "pattern")
	case "TodoWrite":
		return "TodoWrite  (updated task list)"
	case "WebFetch":
		return "WebFetch  " + sv(in, "url")
	case "WebSearch":
		return "WebSearch  " + sv(in, "query")
	case "Task", "Agent":
		return name + "  " + firstNonEmpty(sv(in, "description"), sv(in, "subagent_type"))
	case "ToolSearch":
		return "ToolSearch  " + sv(in, "query")
	default:
		b, _ := json.Marshal(in)
		return name + "  " + truncate(oneLine(string(b)), 200)
	}
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + " …"
	}
	return s
}

var tmpl = template.Must(template.New("").Parse(`
{{define "list"}}<!doctype html><meta charset=utf-8><meta http-equiv=refresh content=10>
<title>fiskaly evals</title>
<style>body{font:14px ui-monospace,monospace;margin:2rem;max-width:64rem}
table{border-collapse:collapse;width:100%}td,th{border-bottom:1px solid #ddd;padding:.4rem .6rem;text-align:left}
a{color:#0645ad;text-decoration:none}.PASS{color:#0a7d00;font-weight:bold}.FAIL{color:#c00;font-weight:bold}.running{color:#a60}
form{display:inline}button{font:inherit;padding:.4rem .8rem;cursor:pointer}</style>
<h1>fiskaly eval runs</h1>
<form method=post action=/trigger><button>▶ trigger run</button></form>
<span style=color:#888> (auto-refreshes; a new run appears once it starts)</span>
<table><tr><th>run<th>when<th>coder<th>harness<th>model<th>build<th>tests<th>judge<th>turns<th>cost</tr>
{{range .}}<tr>
<td><a href="/run/{{.ID}}">{{.ID}}</a>
<td>{{.Created.Format "01-02 15:04"}}
<td>{{.Coder}}
<td>{{.Harness}}
<td>{{.Model}}
{{if eq .Status "running"}}<td colspan=5 class=running>running…</td>
{{else}}<td class="{{.Build}}">{{.Build}}<td class="{{.Tests}}">{{.Tests}}<td class="{{.Judge}}">{{.Judge}}<td>{{.Turns}}<td>{{.Cost}}{{end}}
</tr>{{else}}<tr><td colspan=9>no runs yet</td></tr>{{end}}</table>
{{end}}

{{define "detail"}}<!doctype html><meta charset=utf-8><title>{{.ID}}</title>
<style>body{font:13px ui-monospace,monospace;margin:2rem;max-width:72rem}
a{color:#0645ad}h2{margin:1.4rem 0 .4rem}
.hdr{background:#f6f8fa;border:1px solid #d0d7de;border-radius:6px;padding:.8rem 1rem;margin:.6rem 0}
.hdr .k{color:#888;margin-left:1rem}.hdr .k:first-child{margin-left:0}
.badges{margin-top:.5rem;font-size:14px}.badges b{margin-right:1.2rem}
.PASS{color:#0a7d00}.FAIL{color:#c00}
pre{background:#f6f8fa;padding:.8rem;overflow:auto;white-space:pre-wrap;word-break:break-word;border-radius:6px}
details{margin:.6rem 0;border:1px solid #d0d7de;border-radius:6px;padding:.2rem .6rem}
summary{cursor:pointer;font-weight:bold;padding:.4rem}
.ev{margin:.4rem 0;padding-left:.6rem;border-left:3px solid #ccc;display:flex;gap:.6rem}
.tag{flex:0 0 4.5rem;color:#888}
.txt{flex:1;white-space:pre-wrap;word-break:break-word}
.assistant{color:#111}.tool{color:#06c}.result{color:#690}.final{color:#000;font-weight:bold}.thinking{color:#8250df;font-style:italic}
pre.diff{padding:.4rem 0}.dl{display:block;padding:0 .8rem}
.add{background:#e6ffec}.del{background:#ffebe9}.hunk{color:#0550ae;background:#ddf4ff}.meta{color:#57606a;font-weight:bold}</style>
<a href="/">&larr; runs</a><h1>{{.ID}}</h1>
<div class=hdr>
  <span class=k>coder</span> {{.Coder}}
  <span class=k>model</span> {{.Model}}
  <span class=k>harness</span> {{.Harness}}
  <span class=k>effort</span> {{.Effort}}
  <span class=k>turns</span> {{.Turns}}
  <span class=k>cost</span> {{.Cost}}
  <div class=badges>
    build <b class="{{.Build}}">{{.Build}}</b>
    tests <b class="{{.Tests}}">{{.Tests}}</b>
    judge <b class="{{.Judge}}">{{.Judge}}</b>
  </div>
</div>
<h2>judge verdict</h2><pre>{{.JudgeLog}}</pre>
<details><summary>build · tests{{if .Err}} · stderr{{end}}</summary>
<pre>build:
{{.BuildLog}}
tests:
{{.TestLog}}{{if .Err}}
stderr:
{{.Err}}{{end}}</pre></details>
<details><summary>session transcript ({{len .Transcript}} events)</summary>
{{range .Transcript}}<div class="ev"><span class=tag>{{.Kind}}</span><span class="txt {{.Kind}}">{{.Text}}</span></div>{{end}}
</details>
<details><summary>diff</summary><pre class=diff>{{.Diff}}</pre></details>
{{end}}
`))
