// z2r-smoke provisions a fresh SIGN IT merchant stack on the TEST
// environment and issues one fiscal receipt — the entire Zero-to-Receipt
// happy path through the typed Go client.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"z2r/internal/envfile"
	"z2r/internal/fiskaly"
)

func main() {
	baseURL := flag.String("base-url", fiskaly.TestBaseURL, "fiskaly API base URL (TEST or simulator)")
	flag.Parse()

	envfile.Load(".env")
	key, secret := os.Getenv("FISKALY_API_KEY"), os.Getenv("FISKALY_API_SECRET")
	if key == "" || secret == "" {
		log.Fatal("FISKALY_API_KEY and FISKALY_API_SECRET must be set (see .env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	root := fiskaly.NewClient(*baseURL, key, secret)
	root.OnCall = func(c fiskaly.CallRecord) {
		fmt.Printf("  %-5s %-42s %d  trace=%s\n", c.Method, c.Path, c.Status, c.TraceID)
	}

	started := time.Now()
	fmt.Println("== provisioning merchant stack (TEST) ==")
	stack, scoped, err := fiskaly.ProvisionStack(ctx, root, fiskaly.StackSpec{Name: "Trattoria Da Mario"})
	if err != nil {
		log.Fatalf("provisioning failed: %v", err)
	}
	fmt.Printf("  stack: org=%s taxpayer=%s system=%s\n", stack.UnitOrganizationID, stack.TaxpayerID, stack.SystemID)

	fmt.Println("== issuing receipt ==")
	record, err := fiskaly.IssueReceipt(ctx, scoped, stack.SystemID, "1", []fiskaly.ReceiptItem{
		{Text: "Spaghetti alle vongole", Gross: "14.50"},
		{Text: "Acqua frizzante", Gross: "2.50", VatCode: fiskaly.VatCodeReduced1},
		{Text: "Caffè", Gross: "1.20", VatCode: fiskaly.VatCodeReduced1},
	})
	if err != nil {
		log.Fatalf("receipt failed: %v", err)
	}

	fmt.Printf("\nreceipt %s: state=%s mode=%s in %s\n", record.ID, record.State, record.Mode, time.Since(started).Round(time.Millisecond))
	if record.Compliance != nil {
		fmt.Printf("AdE reference: %s\nAdE document:  %s\n", record.Compliance.Data, record.Compliance.URL)
	}
}
