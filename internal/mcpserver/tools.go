package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"z2r/internal/audit"
	"z2r/internal/fiskaly"
)

// Config wires the server to a fiskaly environment. Live hosts are rejected
// at construction: every record on LIVE legally reaches Agenzia delle
// Entrate, so this prototype is TEST-only by design.
type Config struct {
	BaseURL string
	Key     string
	Secret  string
}

type Server struct {
	cfg   Config
	store *Store
}

func New(cfg Config) (*Server, error) {
	if strings.Contains(cfg.BaseURL, "live.") {
		return nil, fmt.Errorf("refusing to start against %s: LIVE transmits real fiscal documents to AdE; this server is TEST-only by design", cfg.BaseURL)
	}
	return &Server{cfg: cfg, store: NewStore()}, nil
}

func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "zero-to-receipt",
		Version: "0.1.0",
		Title:   "fiskaly SIGN IT — Zero to Receipt",
	}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_integration_context",
		Description: "Read this first. Returns the SIGN IT integration brief: resource hierarchy, " +
			"receipt lifecycle, amount rules, and operational facts learned from live API probing.",
	}, s.getIntegrationContext)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "provision_sandbox",
		Description: "Provision a complete Italian merchant stack on the fiskaly TEST environment " +
			"(UNIT organization → scoped API subject → taxpayer → location → fiscal system, all commissioned). " +
			"Returns an opaque sandbox_id; credentials never leave the server. Takes ~5 seconds.",
	}, s.provisionSandbox)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "issue_receipt",
		Description: "Issue a fiscal receipt (documento commerciale) for a sandbox. Pass gross " +
			"(VAT-inclusive) prices; net amounts, VAT breakdowns and document totals are derived. " +
			"Runs the mandatory INTENTION→TRANSACTION pattern and returns the AdE document reference.",
	}, s.issueReceipt)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_record",
		Description: "Fetch a fiscal record's current state, mode and AdE compliance data.",
	}, s.getRecord)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "cancel_receipt",
		Description: "Cancel a previously completed receipt by issuing the legally required " +
			"CANCELLATION transaction (annullamento documento commerciale).",
	}, s.cancelReceipt)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "audit_session",
		Description: "The judge: replays every API call made in a sandbox against SIGN IT compliance " +
			"rules (idempotency discipline, INTENTION ordering, lifecycle, terminal-state confirmation, " +
			"TEST-host isolation) and returns an audit report with citations. Run before claiming success.",
	}, s.auditSession)

	return srv
}

// --- provision_sandbox ----------------------------------------------------------

type ProvisionIn struct {
	Name string `json:"name" jsonschema:"merchant display name, e.g. 'Trattoria Da Mario'"`
	City string `json:"city,omitempty" jsonschema:"Italian city for the branch location (default Milano)"`
}

type ProvisionOut struct {
	SandboxID          string `json:"sandbox_id"`
	UnitOrganizationID string `json:"unit_organization_id"`
	TaxpayerID         string `json:"taxpayer_id"`
	LocationID         string `json:"location_id"`
	SystemID           string `json:"system_id"`
	Note               string `json:"note"`
}

func (s *Server) provisionSandbox(ctx context.Context, req *mcp.CallToolRequest, in ProvisionIn) (*mcp.CallToolResult, ProvisionOut, error) {
	if in.Name == "" {
		return nil, ProvisionOut{}, fmt.Errorf("name is required")
	}
	trail := &audit.Trail{}
	trail.AddTool("provision_sandbox", in)

	root := fiskaly.NewClient(s.cfg.BaseURL, s.cfg.Key, s.cfg.Secret)
	root.OnCall = trail.AddCall

	stack, scoped, err := fiskaly.ProvisionStack(ctx, root, fiskaly.StackSpec{Name: in.Name, City: in.City})
	if err != nil {
		return nil, ProvisionOut{}, err
	}
	sb := s.store.Add(in.Name, stack, scoped, trail)

	return nil, ProvisionOut{
		SandboxID:          sb.ID,
		UnitOrganizationID: stack.UnitOrganizationID,
		TaxpayerID:         stack.TaxpayerID,
		LocationID:         stack.LocationID,
		SystemID:           stack.SystemID,
		Note:               fmt.Sprintf("Merchant %q is commissioned and OPERATIVE on TEST. Use sandbox_id %q with issue_receipt.", in.Name, sb.ID),
	}, nil
}

// --- issue_receipt ---------------------------------------------------------------

type ReceiptItemIn struct {
	Text     string `json:"text" jsonschema:"line item description as printed on the receipt"`
	Gross    string `json:"gross" jsonschema:"VAT-inclusive price as decimal string, e.g. '14.50'"`
	VatCode  string `json:"vat_code,omitempty" jsonschema:"STANDARD (22%, default), REDUCED_1 (10%), REDUCED_2 (5%) or REDUCED_3 (4%)"`
	Quantity string `json:"quantity,omitempty" jsonschema:"quantity, default '1'"`
}

type IssueReceiptIn struct {
	SandboxID      string          `json:"sandbox_id" jsonschema:"sandbox handle from provision_sandbox"`
	Items          []ReceiptItemIn `json:"items" jsonschema:"the receipt lines, priced gross"`
	DocumentNumber string          `json:"document_number,omitempty" jsonschema:"receipt number; auto-incremented per sandbox when omitted"`
}

type IssueReceiptOut struct {
	RecordID       string `json:"record_id"`
	State          string `json:"state"`
	Mode           string `json:"mode"`
	DocumentNumber string `json:"document_number"`
	TotalGross     string `json:"total_gross"`
	TotalNet       string `json:"total_net"`
	TotalVat       string `json:"total_vat"`
	AdeReference   string `json:"ade_reference,omitempty"`
	AdeDocumentURL string `json:"ade_document_url,omitempty"`
}

func (s *Server) issueReceipt(ctx context.Context, req *mcp.CallToolRequest, in IssueReceiptIn) (*mcp.CallToolResult, IssueReceiptOut, error) {
	sb, err := s.store.Get(in.SandboxID)
	if err != nil {
		return nil, IssueReceiptOut{}, err
	}
	sb.Trail.AddTool("issue_receipt", in)

	number := in.DocumentNumber
	if number == "" {
		number = sb.NextDocumentNumber()
	}
	items := make([]fiskaly.ReceiptItem, len(in.Items))
	for i, it := range in.Items {
		items[i] = fiskaly.ReceiptItem{Text: it.Text, Gross: it.Gross, VatCode: it.VatCode, Quantity: it.Quantity}
	}

	record, err := fiskaly.IssueReceipt(ctx, sb.Client, sb.Stack.SystemID, number, items)
	if err != nil {
		return nil, IssueReceiptOut{}, err
	}

	operation, _ := fiskaly.BuildReceipt(number, items)
	out := IssueReceiptOut{
		RecordID:       record.ID,
		State:          record.State,
		Mode:           record.Mode,
		DocumentNumber: number,
		TotalGross:     operation.Document.TotalVat.Inclusive,
		TotalNet:       operation.Document.TotalVat.Exclusive,
		TotalVat:       operation.Document.TotalVat.Amount,
	}
	if record.Compliance != nil {
		out.AdeReference = record.Compliance.Data
		out.AdeDocumentURL = record.Compliance.URL
	}
	return nil, out, nil
}

// --- get_record -------------------------------------------------------------------

type GetRecordIn struct {
	SandboxID string `json:"sandbox_id" jsonschema:"sandbox handle"`
	RecordID  string `json:"record_id" jsonschema:"record to fetch"`
}

type GetRecordOut struct {
	RecordID     string `json:"record_id"`
	Type         string `json:"type,omitempty"`
	State        string `json:"state"`
	Mode         string `json:"mode"`
	AdeReference string `json:"ade_reference,omitempty"`
	Terminal     bool   `json:"terminal"`
}

func (s *Server) getRecord(ctx context.Context, req *mcp.CallToolRequest, in GetRecordIn) (*mcp.CallToolResult, GetRecordOut, error) {
	sb, err := s.store.Get(in.SandboxID)
	if err != nil {
		return nil, GetRecordOut{}, err
	}
	sb.Trail.AddTool("get_record", in)

	record, err := sb.Client.GetRecord(ctx, in.RecordID)
	if err != nil {
		return nil, GetRecordOut{}, err
	}
	out := GetRecordOut{
		RecordID: record.ID,
		Type:     record.Type,
		State:    record.State,
		Mode:     record.Mode,
		Terminal: record.Terminal(),
	}
	if record.Compliance != nil {
		out.AdeReference = record.Compliance.Data
	}
	return nil, out, nil
}

// --- cancel_receipt ----------------------------------------------------------------

type CancelReceiptIn struct {
	SandboxID string `json:"sandbox_id" jsonschema:"sandbox handle"`
	RecordID  string `json:"record_id" jsonschema:"the COMPLETED receipt transaction to cancel"`
}

type CancelReceiptOut struct {
	RecordID     string `json:"record_id"`
	State        string `json:"state"`
	Mode         string `json:"mode"`
	AdeReference string `json:"ade_reference,omitempty"`
}

func (s *Server) cancelReceipt(ctx context.Context, req *mcp.CallToolRequest, in CancelReceiptIn) (*mcp.CallToolResult, CancelReceiptOut, error) {
	sb, err := s.store.Get(in.SandboxID)
	if err != nil {
		return nil, CancelReceiptOut{}, err
	}
	sb.Trail.AddTool("cancel_receipt", in)

	intention, err := sb.Client.CreateRecord(ctx, fiskaly.RecordCreate{
		Type:      fiskaly.RecordTypeIntention,
		System:    &fiskaly.Ref{ID: sb.Stack.SystemID},
		Operation: fiskaly.TransactionIntention{Type: "TRANSACTION"},
	})
	if err != nil {
		return nil, CancelReceiptOut{}, fmt.Errorf("creating cancellation intention: %w", err)
	}
	record, err := sb.Client.CreateRecord(ctx, fiskaly.RecordCreate{
		Type:   fiskaly.RecordTypeTransaction,
		Record: &fiskaly.Ref{ID: intention.ID},
		Operation: map[string]any{
			"type":   "CANCELLATION",
			"record": fiskaly.Ref{ID: in.RecordID},
		},
	})
	if err != nil {
		return nil, CancelReceiptOut{}, fmt.Errorf("creating cancellation: %w", err)
	}
	if !record.Terminal() {
		record, err = sb.Client.WaitRecord(ctx, record.ID, 90*time.Second)
		if err != nil {
			return nil, CancelReceiptOut{}, err
		}
	}
	out := CancelReceiptOut{RecordID: record.ID, State: record.State, Mode: record.Mode}
	if record.Compliance != nil {
		out.AdeReference = record.Compliance.Data
	}
	return nil, out, nil
}

// --- get_integration_context ---------------------------------------------------------

type EmptyIn struct{}

func (s *Server) getIntegrationContext(ctx context.Context, req *mcp.CallToolRequest, in EmptyIn) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: integrationBrief}},
	}, nil, nil
}

// --- audit_session -----------------------------------------------------------------

type AuditIn struct {
	SandboxID string `json:"sandbox_id,omitempty" jsonschema:"sandbox to audit; omit to audit every sandbox in this session"`
}

type SandboxAudit struct {
	SandboxID string       `json:"sandbox_id"`
	Merchant  string       `json:"merchant"`
	Report    audit.Report `json:"report"`
}

type AuditOut struct {
	Audits  []SandboxAudit `json:"audits"`
	Verdict string         `json:"verdict"`
}

func (s *Server) auditSession(ctx context.Context, req *mcp.CallToolRequest, in AuditIn) (*mcp.CallToolResult, AuditOut, error) {
	var targets []*Sandbox
	if in.SandboxID != "" {
		sb, err := s.store.Get(in.SandboxID)
		if err != nil {
			return nil, AuditOut{}, err
		}
		targets = []*Sandbox{sb}
	} else {
		targets = s.store.All()
	}
	if len(targets) == 0 {
		return nil, AuditOut{}, fmt.Errorf("nothing to audit yet — provision a sandbox and issue a receipt first")
	}

	out := AuditOut{Verdict: "PASS"}
	for _, sb := range targets {
		calls, _ := sb.Trail.Snapshot()
		report := audit.Run(calls)
		if report.Verdict != "PASS" {
			out.Verdict = "FAIL"
		}
		out.Audits = append(out.Audits, SandboxAudit{SandboxID: sb.ID, Merchant: sb.Name, Report: report})
	}
	return nil, out, nil
}
