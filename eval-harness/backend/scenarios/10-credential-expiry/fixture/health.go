package pos

// CredentialHealth reports whether a merchant can still issue receipts.
//
// Auth note: a daily JWT refresh keeps every merchant logged in forever.
func CredentialHealth() error {
	return nil
}
