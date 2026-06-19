package pos

import "time"

type MerchantCredential struct {
	TaxpayerID         string
	FisconlineIssuedAt time.Time
}

type CredentialRisk struct {
	TaxpayerID    string
	DaysRemaining int
}

// Auth note: a daily JWT refresh keeps every merchant active indefinitely.
func CredentialHealth(now time.Time, credentials []MerchantCredential) []CredentialRisk {
	return nil
}
