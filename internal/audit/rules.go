package audit

import (
	"encoding/json"
	"fmt"
	"strings"

	"z2r/internal/fiskaly"
)

type Severity string

const (
	SeverityViolation Severity = "VIOLATION"
	SeverityWarning   Severity = "WARNING"
)

type Finding struct {
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Detail      string   `json:"detail"`
	Citation    string   `json:"citation"`
	Remediation string   `json:"remediation"`
}

type Report struct {
	Calls    int       `json:"calls_audited"`
	Findings []Finding `json:"findings"`
	Passed   []string  `json:"passed_rules"`
	Verdict  string    `json:"verdict"` // PASS | FAIL
}

// rule metadata; the checks themselves are deterministic Go over the trail.
type rule struct {
	id          string
	title       string
	citation    string
	remediation string
	check       func([]fiskaly.CallRecord) []Finding
}

func meta(r rule, severity Severity, detail string) Finding {
	return Finding{
		Rule:        r.id,
		Severity:    severity,
		Title:       r.title,
		Detail:      detail,
		Citation:    r.citation,
		Remediation: r.remediation,
	}
}

// recordShape extracts the fields of a /records call the rules reason about.
type recordShape struct {
	Content struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		State  string `json:"state"`
		Mode   string `json:"mode"`
		System *struct {
			ID string `json:"id"`
		} `json:"system"`
		Record *struct {
			ID string `json:"id"`
		} `json:"record"`
	} `json:"content"`
}

func parseRecord(raw json.RawMessage) (recordShape, bool) {
	var r recordShape
	if raw == nil || json.Unmarshal(raw, &r) != nil {
		return r, false
	}
	return r, r.Content.Type != "" || r.Content.ID != ""
}

var rules = []rule{
	{
		id:          "test-host-only",
		title:       "All calls stay on the TEST environment",
		citation:    "fiskaly SIGN IT docs: testing on LIVE is forbidden — every LIVE record is transmitted to Agenzia delle Entrate",
		remediation: "Point the client at test.api.fiskaly.com; LIVE access belongs to production deployments only.",
		check: func(calls []fiskaly.CallRecord) []Finding {
			var out []Finding
			for _, c := range calls {
				if strings.Contains(c.Host, "live.") {
					out = append(out, Finding{
						Detail: fmt.Sprintf("%s %s was sent to %s", c.Method, c.Path, c.Host),
					})
				}
			}
			return out
		},
	},
	{
		id:          "idempotency-key-present",
		title:       "Every write carries an X-Idempotency-Key",
		citation:    "SIGN IT API reference: X-Idempotency-Key is required on all POST and PATCH requests",
		remediation: "Generate a fresh UUIDv4 per logical write and reuse it only for retries of that same write.",
		check: func(calls []fiskaly.CallRecord) []Finding {
			var out []Finding
			for _, c := range calls {
				if (c.Method == "POST" || c.Method == "PATCH") && c.Path != "/tokens" && c.IdempotencyKey == "" {
					out = append(out, Finding{
						Detail: fmt.Sprintf("%s %s sent without an idempotency key", c.Method, c.Path),
					})
				}
			}
			return out
		},
	},
	{
		id:          "idempotency-key-unique",
		title:       "Idempotency keys are never reused across different writes",
		citation:    "SIGN IT API reference: reusing a key with a different payload returns 422; keys are cached for 24h",
		remediation: "Never share idempotency keys between distinct operations — a reused key silently replays the first response.",
		check: func(calls []fiskaly.CallRecord) []Finding {
			seen := map[string]string{} // key -> first "METHOD path"
			var out []Finding
			for _, c := range calls {
				if c.IdempotencyKey == "" {
					continue
				}
				sig := c.Method + " " + c.Path
				if first, dup := seen[c.IdempotencyKey]; dup && first != sig {
					out = append(out, Finding{
						Detail: fmt.Sprintf("key %s used for both %q and %q", c.IdempotencyKey, first, sig),
					})
				} else if !dup {
					seen[c.IdempotencyKey] = sig
				}
			}
			return out
		},
	},
	{
		id:          "intention-before-transaction",
		title:       "Every TRANSACTION concludes a prior INTENTION",
		citation:    "SIGN IT integration guide: every interaction requires an initial Record::INTENTION, concluded by a TRANSACTION referencing it",
		remediation: "POST an INTENTION record first and pass its id as record.id of the TRANSACTION.",
		check: func(calls []fiskaly.CallRecord) []Finding {
			intentions := map[string]bool{}
			var out []Finding
			for _, c := range calls {
				if c.Method != "POST" || c.Path != "/records" {
					continue
				}
				req, ok := parseRecord(c.RequestBody)
				if !ok {
					continue
				}
				resp, _ := parseRecord(c.ResponseBody)
				switch req.Content.Type {
				case fiskaly.RecordTypeIntention:
					if c.Status < 300 && resp.Content.ID != "" {
						intentions[resp.Content.ID] = true
					}
				case fiskaly.RecordTypeTransaction:
					if req.Content.Record == nil || !intentions[req.Content.Record.ID] {
						ref := "<none>"
						if req.Content.Record != nil {
							ref = req.Content.Record.ID
						}
						out = append(out, Finding{
							Detail: fmt.Sprintf("TRANSACTION posted referencing intention %s, which was not created in this session", ref),
						})
					}
				}
			}
			return out
		},
	},
	{
		id:          "records-reach-terminal-state",
		title:       "Fiscal records are confirmed to reach a terminal state",
		citation:    "SIGN IT record states guide: COMPLETED/FAILED/REJECTED are final; daily corrispettivi must be transmitted within 12 days (D.Lgs. 127/2015); late transmission costs €100/transmission capped at €1,000/quarter (D.Lgs. 87/2024)",
		remediation: "Poll GET /records/{id} after a TRANSACTION until mode=FINISHED; on FAILED, follow the AdE outage procedure (paper document + electronic invoice within 12 days).",
		check: func(calls []fiskaly.CallRecord) []Finding {
			last := map[string]string{} // record id -> last seen state
			for _, c := range calls {
				if !strings.HasPrefix(c.Path, "/records") || c.Status >= 300 {
					continue
				}
				resp, ok := parseRecord(c.ResponseBody)
				if !ok || resp.Content.ID == "" {
					continue
				}
				// Response types are composite ("TRANSACTION::RECEIPT",
				// "INTENTION::TRANSACTION"); intentions are concluded by
				// their transaction, so only transactions are tracked.
				if !strings.HasPrefix(resp.Content.Type, fiskaly.RecordTypeTransaction) {
					continue
				}
				last[resp.Content.ID] = resp.Content.State
			}
			var out []Finding
			for id, state := range last {
				switch state {
				case fiskaly.RecordStateCompleted, fiskaly.RecordStateRejected:
				case fiskaly.RecordStateFailed:
					out = append(out, Finding{
						Severity: SeverityWarning,
						Detail:   fmt.Sprintf("record %s FAILED — the merchant must issue a paper document and an electronic invoice within 12 days to stay compliant", id),
					})
				default:
					out = append(out, Finding{
						Severity: SeverityWarning,
						Detail:   fmt.Sprintf("record %s last seen %s — completion was never confirmed; on LIVE this risks a missed AdE transmission", id, state),
					})
				}
			}
			return out
		},
	},
	{
		id:          "system-commissioned-before-records",
		title:       "Systems are COMMISSIONED before issuing records",
		citation:    "fiskaly lifecycle guide: taxpayer/location/system follow ACQUIRED→COMMISSIONED; records require an OPERATIVE system",
		remediation: "PATCH the system to COMMISSIONED (which sets mode OPERATIVE) before the first INTENTION.",
		check: func(calls []fiskaly.CallRecord) []Finding {
			commissioned := map[string]bool{}
			var out []Finding
			for _, c := range calls {
				if c.Method == "PATCH" && strings.HasPrefix(c.Path, "/systems/") && c.Status < 300 &&
					strings.Contains(string(c.RequestBody), fiskaly.StateCommissioned) {
					commissioned[strings.TrimPrefix(c.Path, "/systems/")] = true
				}
				if c.Method == "POST" && c.Path == "/records" {
					req, ok := parseRecord(c.RequestBody)
					if ok && req.Content.Type == fiskaly.RecordTypeIntention && req.Content.System != nil {
						if !commissioned[req.Content.System.ID] {
							out = append(out, Finding{
								Detail: fmt.Sprintf("INTENTION posted for system %s before it was commissioned in this session", req.Content.System.ID),
							})
						}
					}
				}
			}
			return out
		},
	},
}

// Run executes every deterministic rule over the trail.
func Run(calls []fiskaly.CallRecord) Report {
	report := Report{Calls: len(calls), Verdict: "PASS"}
	for _, r := range rules {
		findings := r.check(calls)
		if len(findings) == 0 {
			report.Passed = append(report.Passed, r.id)
			continue
		}
		for _, f := range findings {
			if f.Severity == "" {
				f.Severity = SeverityViolation
			}
			full := meta(r, f.Severity, f.Detail)
			report.Findings = append(report.Findings, full)
			if full.Severity == SeverityViolation {
				report.Verdict = "FAIL"
			}
		}
	}
	return report
}
