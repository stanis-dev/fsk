package fiskaly

import (
	"encoding/json"
	"time"
)

// The fiskaly Unified API wraps every request and response body in a
// content envelope. Discriminated unions (oneOf + discriminator in the
// OpenAPI spec) are modeled as flat structs whose Type field carries the
// discriminator constant; only the variants used by SIGN IT are included.

type Ref struct {
	ID string `json:"id"`
}

// --- tokens -----------------------------------------------------------------

type TokenAuthentication struct {
	Type      string    `json:"type"`
	Bearer    string    `json:"bearer"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Token struct {
	ID             string              `json:"id"`
	Authentication TokenAuthentication `json:"authentication"`
	Organization   Ref                 `json:"organization"`
	Subject        Ref                 `json:"subject"`
}

// --- organizations ----------------------------------------------------------

const (
	OrganizationTypeGroup = "GROUP"
	OrganizationTypeUnit  = "UNIT"
)

type OrganizationCreate struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type Organization struct {
	ID           string `json:"id"`
	Type         string `json:"type,omitempty"`
	Name         string `json:"name,omitempty"`
	State        string `json:"state,omitempty"`
	Organization *Ref   `json:"organization,omitempty"`
}

// --- subjects ---------------------------------------------------------------

type SubjectCreate struct {
	Type string `json:"type"` // API_KEY
	Name string `json:"name"`
}

type SubjectCredentials struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type Subject struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	Name        string              `json:"name"`
	State       string              `json:"state"`
	Credentials *SubjectCredentials `json:"credentials,omitempty"` // returned once, on creation
}

// --- taxpayers ----------------------------------------------------------------

type CompanyName struct {
	Legal string `json:"legal"`
	Trade string `json:"trade"`
}

type AddressLine struct {
	Type   string `json:"type"` // STREET_NUMBER
	Street string `json:"street"`
	Number string `json:"number"`
}

type Address struct {
	Line    AddressLine `json:"line"`
	Code    string      `json:"code"`
	City    string      `json:"city"`
	Country string      `json:"country"`
}

type FisconlineCredentials struct {
	Type        string `json:"type"` // FISCONLINE
	PIN         string `json:"pin"`
	Password    string `json:"password"`
	TaxIDNumber string `json:"tax_id_number"` // 16-char personal codice fiscale
}

type ItalianFiscalization struct {
	Type        string                `json:"type"` // IT
	TaxIDNumber string                `json:"tax_id_number"`
	VatIDNumber string                `json:"vat_id_number"`
	Credentials FisconlineCredentials `json:"credentials"`
}

type TaxpayerCreate struct {
	Type          string                `json:"type"` // COMPANY
	Name          CompanyName           `json:"name"`
	Address       Address               `json:"address"`
	Fiscalization *ItalianFiscalization `json:"fiscalization,omitempty"`
}

type Taxpayer struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Mode      string `json:"mode"`
	Country   string `json:"country,omitempty"`
	VatNumber string `json:"vat_number,omitempty"`
}

// --- locations -----------------------------------------------------------------

type LocationCreate struct {
	Type     string  `json:"type"` // BRANCH
	Taxpayer Ref     `json:"taxpayer"`
	Name     string  `json:"name"` // max 32 chars
	Address  Address `json:"address"`
}

type Location struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Mode  string `json:"mode"`
}

// --- systems ---------------------------------------------------------------------

type ProducerDetails struct {
	Name string `json:"name"`
}

type Producer struct {
	Type    string          `json:"type"` // MPN
	Number  string          `json:"number"`
	Details ProducerDetails `json:"details"`
}

type Software struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SystemCreate struct {
	Type     string   `json:"type"` // FISCAL_DEVICE
	Location Ref      `json:"location"`
	Producer Producer `json:"producer"`
	Software Software `json:"software"`
}

type System struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Mode  string `json:"mode"`
}

// --- lifecycle ----------------------------------------------------------------------

const (
	StateAcquired       = "ACQUIRED"
	StateCommissioned   = "COMMISSIONED"
	StateDecommissioned = "DECOMMISSIONED"
)

type StateUpdate struct {
	State string `json:"state"`
}

// --- records -------------------------------------------------------------------------

const (
	RecordTypeIntention   = "INTENTION"
	RecordTypeTransaction = "TRANSACTION"

	RecordStateAccepted  = "ACCEPTED"
	RecordStateRejected  = "REJECTED"
	RecordStateCompleted = "COMPLETED"
	RecordStateFailed    = "FAILED"

	RecordModeProcessing = "PROCESSING"
	RecordModeFinished   = "FINISHED"
)

type TransactionIntention struct {
	Type string `json:"type"` // TRANSACTION
}

type TotalVat struct {
	Amount    string `json:"amount"`
	Exclusive string `json:"exclusive"`
	Inclusive string `json:"inclusive"`
}

type Document struct {
	Number   string   `json:"number"`
	TotalVat TotalVat `json:"total_vat"`
	IssuedAt string   `json:"issued_at,omitempty"`
}

type Unit struct {
	Quantity string `json:"quantity"`
	Price    string `json:"price"`
}

type Value struct {
	Base string `json:"base"`
}

const (
	VatCodeStandard = "STANDARD"  // 22%
	VatCodeReduced1 = "REDUCED_1" // 10%
	VatCodeReduced2 = "REDUCED_2" // 5%
	VatCodeReduced3 = "REDUCED_3" // 4%
)

type Vat struct {
	Type       string `json:"type"` // VAT_RATE
	Code       string `json:"code"`
	Percentage string `json:"percentage"`
	Amount     string `json:"amount"`
	Exclusive  string `json:"exclusive"`
	Inclusive  string `json:"inclusive"`
}

type ItemEntry struct {
	Type  string `json:"type"` // ITEM
	Text  string `json:"text"`
	Unit  Unit   `json:"unit"`
	Value Value  `json:"value"`
	Vat   Vat    `json:"vat"`
}

type EntryDetails struct {
	Concept string `json:"concept"` // GOOD | SERVICE
}

type SaleEntry struct {
	Type    string       `json:"type"` // SALE
	Data    ItemEntry    `json:"data"`
	Details EntryDetails `json:"details"`
}

type PaymentDetails struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency,omitempty"`
}

type Payment struct {
	Type    string         `json:"type"` // CASH | CARD | ONLINE | OTHER | OUTSTANDING | VOUCHER
	Details PaymentDetails `json:"details"`
	Number  string         `json:"number,omitempty"` // CARD, VOUCHER
	Kind    string         `json:"kind,omitempty"`   // CARD, VOUCHER
	Name    string         `json:"name,omitempty"`   // ONLINE, OTHER
}

type ReceiptOperation struct {
	Type     string      `json:"type"` // RECEIPT | CORRECTION | CANCELLATION
	Document Document    `json:"document"`
	Entries  []SaleEntry `json:"entries"`
	Payments []Payment   `json:"payments"`
}

// RecordCreate covers both record creation calls: an INTENTION populates
// System, a TRANSACTION populates Record (the intention it concludes).
type RecordCreate struct {
	Type      string `json:"type"`
	System    *Ref   `json:"system,omitempty"`
	Record    *Ref   `json:"record,omitempty"`
	Operation any    `json:"operation"`
}

type Compliance struct {
	Data string `json:"data,omitempty"` // AdE documento commerciale reference
	URL  string `json:"url,omitempty"`  // AdE print endpoint
}

type TransmissionPayload struct {
	Data string `json:"data,omitempty"` // base64 raw AdE payload
	Type string `json:"type,omitempty"` // MIME type
}

type Transmission struct {
	Request  *TransmissionPayload `json:"request,omitempty"`
	Response *TransmissionPayload `json:"response,omitempty"`
}

type Record struct {
	ID           string        `json:"id"`
	Type         string        `json:"type,omitempty"`
	State        string        `json:"state"`
	Mode         string        `json:"mode"`
	System       *Ref          `json:"system,omitempty"`
	Record       *Ref          `json:"record,omitempty"`
	Compliance   *Compliance   `json:"compliance,omitempty"`
	Transmission *Transmission `json:"transmission,omitempty"`
	// Operation is returned by the API as a JSON-encoded string (a
	// double-encoding quirk), so it is kept raw rather than typed.
	Operation json.RawMessage `json:"operation,omitempty"`
	Journal   json.RawMessage `json:"journal,omitempty"`
	File      json.RawMessage `json:"file,omitempty"`
	CreatedAt *time.Time      `json:"created_at,omitempty"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

func (r *Record) Terminal() bool {
	return r.Mode == RecordModeFinished &&
		(r.State == RecordStateCompleted || r.State == RecordStateFailed || r.State == RecordStateRejected)
}
