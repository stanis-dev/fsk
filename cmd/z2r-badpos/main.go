// z2r-badpos is a deliberately sloppy SIGN IT integration — the kind of POS
// code the judge exists to catch. It reuses idempotency keys, skips
// commissioning, posts a transaction for an intention that never existed,
// ignores every error, and never confirms its records reached AdE.
//
// Run it against the simulator, then watch the judge condemn it:
//
//	go run ./cmd/z2r-sim --scenario ade-outage   # terminal 1
//	go run ./cmd/z2r-badpos                      # terminal 2
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"z2r/internal/audit"
	"z2r/internal/fiskaly"
)

type badPOS struct {
	base   string
	bearer string
	trail  *audit.Trail
}

// call is the bad POS's only HTTP helper: it records everything it does but
// checks nothing — errors are someone else's problem.
func (p *badPOS) call(method, path, idemKey string, body any) map[string]any {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(map[string]any{"content": body})
	}
	reqBody := buf.Bytes()
	req, _ := http.NewRequest(method, p.base+path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Version", "2026-02-03")
	if idemKey != "" {
		req.Header.Set("X-Idempotency-Key", idemKey)
	}
	if p.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+p.bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  %-5s %-22s network error (ignored, of course)\n", method, path)
		return nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	p.trail.AddCall(fiskaly.CallRecord{
		At: time.Now(), Host: p.base, Method: method, Path: path,
		IdempotencyKey: idemKey, Status: resp.StatusCode,
		TraceID:     resp.Header.Get("X-Trace-Identifier"),
		RequestBody: reqBody, ResponseBody: respBody,
	})
	fmt.Printf("  %-5s %-22s %d\n", method, path, resp.StatusCode)

	var parsed struct {
		Content map[string]any `json:"content"`
	}
	json.Unmarshal(respBody, &parsed)
	return parsed.Content
}

func id(content map[string]any) string {
	if content == nil {
		return uuid.NewString() // who needs the real id anyway
	}
	s, _ := content["id"].(string)
	return s
}

func main() {
	base := flag.String("base-url", "http://127.0.0.1:8484", "simulator base URL (refuses fiskaly hosts)")
	flag.Parse()
	if bytes.Contains([]byte(*base), []byte("fiskaly.com")) {
		fmt.Println("badpos misbehaves on purpose — it only runs against the local simulator")
		os.Exit(1)
	}

	pos := &badPOS{base: *base, trail: &audit.Trail{}}

	fmt.Println("== sloppy POS integration at work ==")
	tok := pos.call("POST", "/tokens", uuid.NewString(), map[string]any{"type": "API_KEY", "key": "k", "secret": "s"})
	if auth, ok := tok["authentication"].(map[string]any); ok {
		pos.bearer, _ = auth["bearer"].(string)
	}

	org := pos.call("POST", "/organizations", uuid.NewString(), map[string]any{"type": "UNIT", "name": "Sloppy Pizza"})
	_ = org

	// One idempotency key is plenty for everybody, right?
	sharedKey := uuid.NewString()
	taxpayer := pos.call("POST", "/taxpayers", sharedKey, map[string]any{
		"type": "COMPANY", "name": map[string]any{"legal": "Sloppy Pizza S.r.l.", "trade": "Sloppy Pizza"},
	})
	pos.call("POST", "/locations", sharedKey, map[string]any{ // 422: key reused — ignored
		"type": "BRANCH", "taxpayer": map[string]any{"id": id(taxpayer)}, "name": "Roma",
	})
	location := pos.call("POST", "/locations", uuid.NewString(), map[string]any{
		"type": "BRANCH", "taxpayer": map[string]any{"id": id(taxpayer)}, "name": "Roma",
	})
	system := pos.call("POST", "/systems", uuid.NewString(), map[string]any{
		"type": "FISCAL_DEVICE", "location": map[string]any{"id": id(location)},
		"producer": map[string]any{"type": "MPN", "number": "BAD-1", "details": map[string]any{"name": "BadPOS"}},
		"software": map[string]any{"name": "badpos", "version": "0.0.1"},
	})
	systemID := id(system)

	// Commissioning? Never heard of it. (405 — ignored.)
	intent := pos.call("POST", "/records", uuid.NewString(), map[string]any{
		"type": "INTENTION", "system": map[string]any{"id": systemID},
		"operation": map[string]any{"type": "TRANSACTION"},
	})
	_ = intent

	// Fine, commission the system since the API insists.
	pos.call("PATCH", "/systems/"+systemID, uuid.NewString(), map[string]any{"state": "COMMISSIONED"})

	// A transaction referencing an intention that never existed (404 — ignored).
	pos.call("POST", "/records", uuid.NewString(), map[string]any{
		"type": "TRANSACTION", "record": map[string]any{"id": uuid.NewString()},
		"operation": receiptOp("1"),
	})

	// And one "successful" receipt: intention, transaction, fire and forget —
	// never polled, never confirmed. If AdE was down, nobody will ever know.
	intent2 := pos.call("POST", "/records", uuid.NewString(), map[string]any{
		"type": "INTENTION", "system": map[string]any{"id": systemID},
		"operation": map[string]any{"type": "TRANSACTION"},
	})
	pos.call("POST", "/records", uuid.NewString(), map[string]any{
		"type": "TRANSACTION", "record": map[string]any{"id": id(intent2)},
		"operation": receiptOp("2"),
	})

	fmt.Println("\n== the judge reviews the session ==")
	calls, _ := pos.trail.Snapshot()
	report := audit.Run(calls)
	fmt.Println()
	fmt.Println(report.Markdown())
	if report.Verdict != "PASS" {
		os.Exit(1)
	}
}

func receiptOp(number string) map[string]any {
	op, _ := fiskaly.BuildReceipt(number, []fiskaly.ReceiptItem{{Text: "Pizza margherita", Gross: "8.50"}})
	return map[string]any{
		"type": op.Type, "document": op.Document, "entries": op.Entries, "payments": op.Payments,
	}
}
