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
// Usage: judge <integration-dir>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type rule struct {
	id   string
	want *regexp.Regexp // must appear in the integration source to pass
	desc string
	cite string
	hint string // shown on failure
}

// Each rule encodes one fact established by live probing (see NOTES.md). They are
// necessary conditions, not sufficient: passing them means the integration is
// shaped like the real contract, not that it is correct end to end. That is the
// behavioral judge's job.
var rules = []rule{
	{
		id:   "fiskaly-host",
		want: regexp.MustCompile(`(?i)\b(test|live)\.api\.fiskaly\.com\b`),
		desc: "targets the real fiskaly API host",
		cite: "NOTES.md: host is test.api.fiskaly.com / live.api.fiskaly.com",
		hint: "uses a non-existent host (e.g. fiscal.fiskaly.com) or none",
	},
	{
		id:   "token-exchange",
		want: regexp.MustCompile(`/tokens\b`),
		desc: "exchanges credentials for a JWT at POST /tokens",
		cite: "NOTES.md step 1: POST /tokens with API_KEY key+secret -> JWT",
		hint: "auth is POST /tokens, not an invented /auth endpoint",
	},
	{
		id:   "idempotency-key",
		want: regexp.MustCompile(`(?i)X-Idempotency-Key`),
		desc: "sets X-Idempotency-Key on writes",
		cite: "NOTES.md addendum: required on every POST incl /tokens (lowercase UUID v3/v4)",
		hint: "every POST needs an X-Idempotency-Key or fiskaly returns 400",
	},
	{
		id:   "api-version",
		want: regexp.MustCompile(`(?i)X-Api-Version`),
		desc: "sends the dated X-Api-Version header",
		cite: "NOTES.md: X-Api-Version: 2026-02-03 required on all calls",
		hint: "all calls need the dated version header",
	},
	{
		id:   "records-flow",
		want: regexp.MustCompile(`/records\b`),
		desc: "issues the receipt through the records endpoint",
		cite: "NOTES.md steps 10-11: POST /records INTENTION, then TRANSACTION (RECEIPT)",
		hint: "a receipt is a two-call records flow, not a single /receipts POST",
	},
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	src, err := readSource(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "judge:", err)
		os.Exit(2)
	}

	fmt.Printf("fiskaly contract conformance: %s\n\n", dir)
	fails := 0
	for _, r := range rules {
		if r.want.MatchString(src) {
			fmt.Printf("PASS  %-16s %s\n", r.id, r.desc)
			continue
		}
		fails++
		fmt.Printf("FAIL  %-16s %s\n", r.id, r.desc)
		fmt.Printf("      cite: %s\n", r.cite)
		fmt.Printf("      hint: %s\n", r.hint)
	}

	fmt.Printf("\n%d/%d rules passed.\n", len(rules)-fails, len(rules))
	if fails > 0 {
		fmt.Printf("VERDICT: NON-COMPLIANT (%d failures). exit 1\n", fails)
		os.Exit(1)
	}
	fmt.Println("VERDICT: conformant. exit 0")
}

// readSource concatenates non-test Go source under dir. Tests are excluded so a
// mock that mimics an invented API cannot satisfy a rule.
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
		b.Write(data)
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}
