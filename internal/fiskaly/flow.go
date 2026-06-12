package fiskaly

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Stack is one fully provisioned SIGN IT merchant hierarchy, ready to issue
// receipts: UNIT organization -> scoped subject -> taxpayer -> location ->
// system, all COMMISSIONED.
type Stack struct {
	UnitOrganizationID string `json:"unit_organization_id"`
	SubjectID          string `json:"subject_id"`
	SubjectKey         string `json:"subject_key"`
	SubjectSecret      string `json:"subject_secret"`
	TaxpayerID         string `json:"taxpayer_id"`
	LocationID         string `json:"location_id"`
	SystemID           string `json:"system_id"`
}

type StackSpec struct {
	Name      string // merchant display name, e.g. "Trattoria Da Mario"
	LegalName string // defaults to Name + " S.r.l."
	Street    string
	Number    string
	ZipCode   string
	City      string
}

func (s *StackSpec) defaults() {
	if s.LegalName == "" {
		s.LegalName = s.Name + " S.r.l."
	}
	if s.Street == "" {
		s.Street, s.Number = "Via Roma", "1"
	}
	if s.ZipCode == "" {
		s.ZipCode = "20121"
	}
	if s.City == "" {
		s.City = "Milano"
	}
}

// slug squeezes a display name into the subject-name shape the API enforces
// (^[a-z0-9-]{3,30}$ — undocumented outside the error response itself).
func slug(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ' || r == '-' || r == '_':
			if n := len(out); n > 0 && out[n-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	s := strings.Trim(string(out), "-")
	if len(s) > 30 {
		s = strings.Trim(s[:30], "-")
	}
	for len(s) < 3 {
		s += "0"
	}
	return s
}

// testFisconline are placeholder Fisconline credentials accepted by the TEST
// environment. LIVE requires the merchant's real credentials (90-day expiry).
var testFisconline = FisconlineCredentials{
	Type:        "FISCONLINE",
	PIN:         "1234567890",
	Password:    "TestPassword1!",
	TaxIDNumber: "RSSMRA85M01H501Z",
}

// ProvisionStack builds a complete merchant stack using the root (GROUP)
// client, returning the stack and a client authenticated as the UNIT-scoped
// subject. Resource creation does not honor X-Scope-Identifier redirection,
// so the only path into a UNIT org is a subject bound to it at creation.
func ProvisionStack(ctx context.Context, root *Client, spec StackSpec) (Stack, *Client, error) {
	spec.defaults()
	var stack Stack

	unit, err := root.CreateOrganization(ctx, OrganizationCreate{Type: OrganizationTypeUnit, Name: spec.Name})
	if err != nil {
		return stack, nil, fmt.Errorf("creating UNIT organization: %w", err)
	}
	stack.UnitOrganizationID = unit.ID

	subject, err := root.CreateSubject(ctx, SubjectCreate{Type: "API_KEY", Name: slug(spec.Name)}, unit.ID)
	if err != nil {
		return stack, nil, fmt.Errorf("creating scoped subject: %w", err)
	}
	if subject.Credentials == nil {
		return stack, nil, fmt.Errorf("subject %s came back without credentials", subject.ID)
	}
	stack.SubjectID = subject.ID
	stack.SubjectKey = subject.Credentials.Key
	stack.SubjectSecret = subject.Credentials.Secret

	scoped := NewClient(root.BaseURL, subject.Credentials.Key, subject.Credentials.Secret)
	scoped.OnCall = root.OnCall

	address := Address{
		Line:    AddressLine{Type: "STREET_NUMBER", Street: spec.Street, Number: spec.Number},
		Code:    spec.ZipCode,
		City:    spec.City,
		Country: "IT",
	}

	taxpayer, err := scoped.CreateTaxpayer(ctx, TaxpayerCreate{
		Type:    "COMPANY",
		Name:    CompanyName{Legal: spec.LegalName, Trade: spec.Name},
		Address: address,
		Fiscalization: &ItalianFiscalization{
			Type:        "IT",
			TaxIDNumber: "12345678903",
			VatIDNumber: "12345678903",
			Credentials: testFisconline,
		},
	})
	if err != nil {
		return stack, nil, fmt.Errorf("creating taxpayer: %w", err)
	}
	stack.TaxpayerID = taxpayer.ID

	name := spec.City
	if len(name) > 32 {
		name = name[:32]
	}
	location, err := scoped.CreateLocation(ctx, LocationCreate{
		Type:     "BRANCH",
		Taxpayer: Ref{ID: taxpayer.ID},
		Name:     name,
		Address:  address,
	})
	if err != nil {
		return stack, nil, fmt.Errorf("creating location: %w", err)
	}
	stack.LocationID = location.ID

	system, err := scoped.CreateSystem(ctx, SystemCreate{
		Type:     "FISCAL_DEVICE",
		Location: Ref{ID: location.ID},
		Producer: Producer{Type: "MPN", Number: "Z2R-POS-001", Details: ProducerDetails{Name: "Zero-to-Receipt POS"}},
		Software: Software{Name: "zero-to-receipt", Version: "0.1.0"},
	})
	if err != nil {
		return stack, nil, fmt.Errorf("creating system: %w", err)
	}
	stack.SystemID = system.ID

	for _, step := range []struct{ resource, id string }{
		{"taxpayers", taxpayer.ID},
		{"locations", location.ID},
		{"systems", system.ID},
	} {
		if err := scoped.SetState(ctx, step.resource, step.id, StateCommissioned); err != nil {
			return stack, nil, fmt.Errorf("commissioning %s: %w", step.resource, err)
		}
	}

	return stack, scoped, nil
}

// ReceiptItem is one line of a receipt, priced gross (VAT inclusive) the way
// POS systems think about it; net and VAT amounts are derived.
type ReceiptItem struct {
	Text     string
	Gross    string // VAT-inclusive amount, e.g. "12.20"
	VatCode  string // defaults to STANDARD (22%)
	Concept  string // GOOD (default) or SERVICE
	Quantity string // defaults to "1"
}

// BuildReceipt derives the full fiscal payload (per-line VAT breakdown and
// document totals) from gross-priced items. The API derives nothing: every
// percentage, amount, exclusive and inclusive field is mandatory.
func BuildReceipt(documentNumber string, items []ReceiptItem) (ReceiptOperation, error) {
	if len(items) == 0 {
		return ReceiptOperation{}, fmt.Errorf("a receipt needs at least one item")
	}
	var entries []SaleEntry
	var totalGross, totalNet int64
	for i, item := range items {
		if item.VatCode == "" {
			item.VatCode = VatCodeStandard
		}
		if item.Concept == "" {
			item.Concept = "GOOD"
		}
		if item.Quantity == "" {
			item.Quantity = "1"
		}
		bp, ok := vatBasisPoints[item.VatCode]
		if !ok {
			return ReceiptOperation{}, fmt.Errorf("item %d: unknown VAT code %q", i, item.VatCode)
		}
		gross, err := parseCents(item.Gross)
		if err != nil {
			return ReceiptOperation{}, fmt.Errorf("item %d: %w", i, err)
		}
		net := netFromGross(gross, bp)
		percentage, _ := VatPercentage(item.VatCode)

		entries = append(entries, SaleEntry{
			Type: "SALE",
			Data: ItemEntry{
				Type:  "ITEM",
				Text:  item.Text,
				Unit:  Unit{Quantity: item.Quantity, Price: formatCents(gross)},
				Value: Value{Base: formatCents(gross)},
				Vat: Vat{
					Type:       "VAT_RATE",
					Code:       item.VatCode,
					Percentage: percentage,
					Amount:     formatCents(gross - net),
					Exclusive:  formatCents(net),
					Inclusive:  formatCents(gross),
				},
			},
			Details: EntryDetails{Concept: item.Concept},
		})
		totalGross += gross
		totalNet += net
	}

	return ReceiptOperation{
		Type: "RECEIPT",
		Document: Document{
			Number: documentNumber,
			TotalVat: TotalVat{
				Amount:    formatCents(totalGross - totalNet),
				Exclusive: formatCents(totalNet),
				Inclusive: formatCents(totalGross),
			},
		},
		Entries: entries,
		Payments: []Payment{
			{Type: "CASH", Details: PaymentDetails{Amount: formatCents(totalGross), Currency: "EUR"}},
		},
	}, nil
}

// IssueReceipt runs the documented two-call pattern: INTENTION first, then
// the TRANSACTION concluding it, then waits for the terminal state (TEST
// completes synchronously; LIVE transmits to AdE asynchronously).
func IssueReceipt(ctx context.Context, c *Client, systemID, documentNumber string, items []ReceiptItem) (Record, error) {
	intention, err := c.CreateRecord(ctx, RecordCreate{
		Type:      RecordTypeIntention,
		System:    &Ref{ID: systemID},
		Operation: TransactionIntention{Type: "TRANSACTION"},
	})
	if err != nil {
		return Record{}, fmt.Errorf("creating intention: %w", err)
	}

	operation, err := BuildReceipt(documentNumber, items)
	if err != nil {
		return Record{}, err
	}
	txn, err := c.CreateRecord(ctx, RecordCreate{
		Type:      RecordTypeTransaction,
		Record:    &Ref{ID: intention.ID},
		Operation: operation,
	})
	if err != nil {
		return Record{}, fmt.Errorf("creating transaction: %w", err)
	}
	if txn.Terminal() {
		return txn, nil
	}
	return c.WaitRecord(ctx, txn.ID, 90*time.Second)
}
