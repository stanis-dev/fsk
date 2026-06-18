// Command judge checks a fiskaly integration against the verified SIGN IT
// contract (research/api-probes/NOTES.md). It is a deterministic, offline
// conformance gate: every rule maps to a probed fact, prints PASS/FAIL with a
// citation, and exits non-zero on any failure.
//
// This is the static first cut: it inspects the integration source, not live
// behavior. The behavioral judge (replaying real records from fiskaly TEST, or a
// local conformance stub) is the stronger successor and supersedes these rules
// once integrations actually drive a real or controllable endpoint.
//
// A rule is positive (every pattern in `want` must appear in the integration
// source) and/or negative (`deny` must NOT appear — used to catch an agent that
// fell for a red herring, e.g. inventing a /refunds endpoint or shipping the
// legacy /assets resources). Each scenario selects the rule subset that encodes
// its acceptance bar with -rules; with no flag the original five base rules run,
// so `judge <dir>` behaves exactly as before.
//
// Usage: judge [-rules=id1,id2,...] [-list] <integration-dir>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type rule struct {
	id   string
	want []*regexp.Regexp // every pattern must appear in the source to pass
	deny *regexp.Regexp   // if set, must NOT appear in the source (negative rule)
	desc string
	cite string
	hint string // shown on failure
}

func (r rule) pass(src string) bool {
	for _, w := range r.want {
		if !w.MatchString(src) {
			return false
		}
	}
	if r.deny != nil && r.deny.MatchString(src) {
		return false
	}
	return true
}

func re(p string) *regexp.Regexp { return regexp.MustCompile(p) }

// catalog is the full set of conformance rules. Each encodes one fact established
// by live probing (see NOTES.md) or one documented contract a scenario exercises.
// They are necessary conditions, not sufficient: passing them means the
// integration is shaped like the real contract, not that it is correct end to
// end. That is the behavioral judge's job.
var catalog = []rule{
	// --- the five base rules (the default set; do not reorder) ---------------
	{
		id:   "fiskaly-host",
		want: []*regexp.Regexp{re(`(?i)\b(test|live)\.api\.fiskaly\.com\b`)},
		desc: "targets the real fiskaly API host",
		cite: "NOTES.md: host is test.api.fiskaly.com / live.api.fiskaly.com",
		hint: "uses a non-existent host (e.g. fiscal.fiskaly.com) or none",
	},
	{
		id:   "token-exchange",
		want: []*regexp.Regexp{re(`/tokens\b`)},
		desc: "exchanges credentials for a JWT at POST /tokens",
		cite: "NOTES.md step 1: POST /tokens with API_KEY key+secret -> JWT",
		hint: "auth is POST /tokens, not an invented /auth endpoint",
	},
	{
		id:   "idempotency-key",
		want: []*regexp.Regexp{re(`(?i)X-Idempotency-Key`)},
		desc: "sets X-Idempotency-Key on writes",
		cite: "NOTES.md addendum: required on every POST incl /tokens (lowercase UUID v3/v4)",
		hint: "every POST needs an X-Idempotency-Key or fiskaly returns 400",
	},
	{
		id:   "api-version",
		want: []*regexp.Regexp{re(`(?i)X-Api-Version`)},
		desc: "sends the dated X-Api-Version header",
		cite: "NOTES.md: X-Api-Version: 2026-02-03 required on all calls",
		hint: "all calls need the dated version header",
	},
	{
		id:   "records-flow",
		want: []*regexp.Regexp{re(`/records\b`)},
		desc: "issues the receipt through the records endpoint",
		cite: "NOTES.md steps 10-11: POST /records INTENTION, then TRANSACTION (RECEIPT)",
		hint: "a receipt is a two-call records flow, not a single /receipts POST",
	},

	// --- scenario rules (opt in via -rules) ----------------------------------
	{
		id:   "scope-identifier",
		want: []*regexp.Regexp{re(`(?i)X-Scope-Identifier`)},
		desc: "scopes subject creation to the UNIT with X-Scope-Identifier",
		cite: "NOTES.md step 4: POST /subjects with X-Scope-Identifier:<unit-id> mints UNIT-scoped credentials",
		hint: "creating a taxpayer needs a UNIT-scoped subject first, or fiskaly returns 405 E_METHOD_NOT_ALLOWED",
	},
	{
		id:   "commissioning",
		want: []*regexp.Regexp{re(`(?i)\bCOMMISSIONED\b`)},
		desc: "commissions taxpayer -> location -> system before issuing",
		cite: "NOTES.md step 9: PATCH .../{id} {state: COMMISSIONED}; mode flips to OPERATIVE",
		hint: "resources are created INACTIVE; an INACTIVE system cannot issue records",
	},
	{
		id:   "cancellation-ref",
		want: []*regexp.Regexp{re(`(?i)\bCANCELLATION\b`)},
		desc: "voids via a CANCELLATION record referencing the original",
		cite: "NOTES.md: the record-type taxonomy; a void is a records flow referencing the original record.id",
		hint: "a refund/void is a CANCELLATION record, not a delete or a /refunds POST",
	},
	{
		id:   "no-invented-refunds",
		deny: re(`(?i)/refunds\b`),
		desc: "does not invent a /refunds endpoint",
		cite: "NOTES.md + specs: there is no /refunds; corrections go through /records",
		hint: "the docs describe no /refunds endpoint — voiding is a CANCELLATION record",
	},
	{
		id:   "polling",
		want: []*regexp.Regexp{re(`(?i)\bFINISHED\b`)},
		desc: "polls the record to the FINISHED terminal state",
		cite: "NOTES.md step 11: LIVE is async -> poll the record until state/mode FINISHED",
		hint: "fire-and-forget on PROCESSING never reaches the tax authority; you must poll to FINISHED",
	},
	{
		// Match the breakdown being CONSTRUCTED — every field as a quoted JSON key or
		// struct tag ("percentage, "amount, "exclusive, "inclusive) — not merely
		// mentioned. readSource strips comments, so prose like "VAT-exclusive" cannot
		// satisfy this; and the API derives none of the four, so all four are required.
		id:   "vat-breakdown",
		want: []*regexp.Regexp{re(`"percentage`), re(`"amount`), re(`"exclusive`), re(`"inclusive`)},
		desc: "sends the full VatRateCategory breakdown (percentage/amount/exclusive/inclusive)",
		cite: "NOTES.md: VatRateCategory requires ALL of percentage, amount, exclusive, inclusive — the API derives none",
		hint: "the API derives no VAT field; the integration must compute and send all four — percentage, amount, exclusive, inclusive",
	},
	{
		id:   "no-legacy-resources",
		deny: re(`(?i)/(assets|entities)\b`),
		desc: "uses the current resource names, not the renamed legacy ones",
		cite: "OPPORTUNITIES.md #4: /entities and /assets were renamed to /organizations, /taxpayers, /locations, /systems",
		hint: "/entities and /assets are the pre-rename resources; migrate to the current names",
	},
	{
		// Require the header name AND the current date, so dropping X-Api-Version
		// while leaving the date in a constant cannot pass.
		id:   "api-version-current",
		want: []*regexp.Regexp{re(`(?i)X-Api-Version`), re(`2026-02-03`)},
		desc: "sends the X-Api-Version header pinned to the current date",
		cite: "NOTES.md: X-Api-Version: 2026-02-03 required on all calls",
		hint: "an older X-Api-Version date (or a missing header) targets a superseded contract",
	},
}

// defaultRules is the original five-rule set, used when -rules is not supplied so
// that `judge <dir>` keeps behaving exactly as it did before scenarios existed.
var defaultRules = []string{"fiskaly-host", "token-exchange", "idempotency-key", "api-version", "records-flow"}

// ruleResult is one deterministic gate rule's outcome, for the structured report.
type ruleResult struct {
	ID   string `json:"id"`
	Desc string `json:"desc"`
	Pass bool   `json:"pass"`
}

// judgeReport is the structured verdict written to judge.json for the dashboard.
// The exit code remains the harness's source of truth; judge.json is the dashboard's
// authoritative verdict (preferred over scanning judge.txt).
type judgeReport struct {
	Scenario string `json:"scenario"`
	Gate     struct {
		Passed bool         `json:"passed"`
		Rules  []ruleResult `json:"rules"`
	} `json:"gate"`
	Rubric  *rubricReport `json:"rubric"`
	Verdict string        `json:"verdict"` // conformant | NON-COMPLIANT
	Note    string        `json:"note"`
}

func buildReport(scenario string, gate []ruleResult, gatePassed bool, rep *rubricReport, verdict string) judgeReport {
	var r judgeReport
	r.Scenario = scenario
	r.Gate.Passed = gatePassed
	r.Gate.Rules = gate
	r.Rubric = rep
	r.Verdict = verdict
	r.Note = "LLM rubric layer is nondeterministic; conformance requires the deterministic gate to pass AND every rubric criterion to be a cited MET"
	return r
}

// renderRubric formats the rubric outcome for the human-readable judge.txt block.
func renderRubric(rep rubricReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nRUBRIC (model: %s)\n", rep.Model)
	for _, v := range rep.Criteria {
		fmt.Fprintf(&b, "%-13s %s\n", v.Verdict, v.ID)
		if v.Reasoning != "" {
			fmt.Fprintf(&b, "      %s\n", v.Reasoning)
		}
		if v.EvidenceQuote != "" {
			fmt.Fprintf(&b, "      evidence: %s\n", v.EvidenceQuote)
		}
		if v.Cite != "" {
			fmt.Fprintf(&b, "      cite: %s\n", v.Cite)
		}
	}
	return b.String()
}

func main() {
	var (
		rulesFlag    = flag.String("rules", "", "comma-separated rule ids to run (default: the five base rules)")
		scenarioFlag = flag.String("scenario", "", "path to a scenario.json; uses its judge.rules (overrides -rules)")
		list         = flag.Bool("list", false, "list every rule id in the catalog and exit")
		rubricFlag   = flag.Bool("rubric", false, "after the gate passes, run the LLM rubric layer (requires the scenario to declare judge.rubric and the claude CLI)")
		jsonFlag     = flag.String("json", "", "write the structured verdict to this path as JSON")
	)
	flag.Parse()

	byID := map[string]rule{}
	for _, r := range catalog {
		byID[r.id] = r
	}

	if *list {
		ids := make([]string, 0, len(catalog))
		for _, r := range catalog {
			ids = append(ids, r.id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Printf("%-22s %s\n", id, byID[id].desc)
		}
		return
	}

	spec := *rulesFlag
	if *scenarioFlag != "" {
		s, err := rulesFromScenario(*scenarioFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "judge:", err)
			os.Exit(2)
		}
		spec = s
	}

	selected, err := selectRules(byID, spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
		os.Exit(2)
	}

	dir := "."
	if args := flag.Args(); len(args) > 0 {
		dir = args[0]
	}

	src, err := readSource(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
		os.Exit(2)
	}

	fmt.Printf("fiskaly contract conformance: %s\n\n", dir)
	fails := 0
	ruleResults := make([]ruleResult, 0, len(selected))
	for _, r := range selected {
		ok := r.pass(src)
		ruleResults = append(ruleResults, ruleResult{ID: r.id, Desc: r.desc, Pass: ok})
		if ok {
			fmt.Printf("PASS  %-20s %s\n", r.id, r.desc)
			continue
		}
		fails++
		fmt.Printf("FAIL  %-20s %s\n", r.id, r.desc)
		fmt.Printf("      cite: %s\n", r.cite)
		fmt.Printf("      hint: %s\n", r.hint)
	}
	fmt.Printf("\n%d/%d rules passed.\n", len(selected)-fails, len(selected))

	scenarioName := ""
	if *scenarioFlag != "" {
		scenarioName = filepath.Base(filepath.Dir(*scenarioFlag))
	}
	gatePassed := fails == 0

	// Gate is the hard pre-gate: any deterministic failure is NON-COMPLIANT and the
	// LLM layer never runs (it can only add FAILs, never override a gate FAIL).
	if !gatePassed {
		fmt.Printf("VERDICT: NON-COMPLIANT (%d gate failures). exit 1\n", fails)
		writeReport(*jsonFlag, buildReport(scenarioName, ruleResults, false, nil, "NON-COMPLIANT"))
		os.Exit(1)
	}

	verdict := "conformant"
	exitCode := 0
	var rep *rubricReport

	if *rubricFlag && *scenarioFlag != "" {
		crits, err := rubricFromScenario(*scenarioFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "judge:", err)
			os.Exit(2)
		}
		if len(crits) > 0 {
			raw, err := readSourceRaw(dir)
			if err != nil {
				fmt.Fprintln(os.Stderr, "judge:", err)
				os.Exit(2)
			}
			// No silent fallback: a missing/failed model is a hard error, never a
			// gate-only pass dressed up as conformant.
			r, err := runRubric(raw, src, crits, claudeModel, judgeModelID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "judge: rubric layer:", err)
				os.Exit(2)
			}
			rep = &r
			fmt.Print(renderRubric(r))
			if !conformant(r.Criteria) {
				verdict = "NON-COMPLIANT"
				exitCode = 1
			}
		}
	}

	if exitCode == 0 {
		fmt.Println("VERDICT: conformant. exit 0")
	} else {
		fmt.Println("VERDICT: NON-COMPLIANT (rubric). exit 1")
	}
	writeReport(*jsonFlag, buildReport(scenarioName, ruleResults, true, rep, verdict))
	os.Exit(exitCode)
}

// writeReport marshals the structured verdict to path (no-op when path is empty).
// A write failure is loud (exit 2) — the dashboard depends on this artifact.
func writeReport(path string, report judgeReport) {
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge: marshaling report:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "judge: writing report:", err)
		os.Exit(2)
	}
}

// rulesFromScenario reads a scenario.json and returns its judge.rules as the
// comma-separated spec selectRules expects.
func rulesFromScenario(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading scenario: %w", err)
	}
	var s struct {
		Judge struct {
			Rules []string `json:"rules"`
		} `json:"judge"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("parsing scenario %s: %w", path, err)
	}
	if len(s.Judge.Rules) == 0 {
		return "", fmt.Errorf("scenario %s has no judge.rules", path)
	}
	return strings.Join(s.Judge.Rules, ","), nil
}

// selectRules resolves a comma-separated id list against the catalog, preserving
// the requested order. An empty spec yields the default five base rules.
func selectRules(byID map[string]rule, spec string) ([]rule, error) {
	ids := defaultRules
	if strings.TrimSpace(spec) != "" {
		ids = nil
		for _, raw := range strings.Split(spec, ",") {
			id := strings.TrimSpace(raw)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	out := make([]rule, 0, len(ids))
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown rule %q (try -list)", id)
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no rules selected")
	}
	return out, nil
}

// readSource concatenates non-test Go source under dir, with comments stripped.
// Tests are excluded so a mock that mimics an invented API cannot satisfy a rule;
// comments are excluded so rules match the code an integration actually runs, not
// explanatory prose — a deny rule must not fire on a comment like
// "do not call /refunds; use /records CANCELLATION", and a want rule must not be
// satisfied by a token that appears only in a comment.
func readSource(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString(stripComments(data))
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}

// readSourceRaw concatenates non-test Go source under dir with comments retained,
// for the LLM rubric layer (the model reasons over comments; the citation check
// later validates evidence against the comment-stripped source). Tests are still
// excluded, matching readSource's anti-gaming exclusion.
func readSourceRaw(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(data)
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}

// stripComments returns the Go source with comment tokens removed. It lexes with
// go/scanner so string literals are preserved intact — the // in
// "https://test.api.fiskaly.com" is part of a STRING token, not a line comment,
// and is not mangled. Falls back to the raw bytes if scanning yields nothing.
func stripComments(src []byte) string {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	// nil error handler + default mode: comments are skipped, errors are tolerated
	// (the judge must never crash on whatever the agent produced).
	s.Init(file, src, nil, 0)
	var b strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if lit != "" {
			b.WriteString(lit)
		} else {
			b.WriteString(tok.String())
		}
		b.WriteByte(' ')
	}
	if b.Len() == 0 {
		return string(src)
	}
	return b.String()
}
