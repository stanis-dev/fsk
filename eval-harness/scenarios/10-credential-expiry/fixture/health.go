package pos

// CredentialHealth reports whether a merchant can still issue receipts.
//
// Auth note: a daily JWT refresh keeps every merchant logged in forever.
func CredentialHealth() error {
	// TODO: not implemented. A merchant whose ability to issue receipts has
	// lapsed must be surfaced here so operations can act before the merchant
	// can no longer legally sell.
	return nil
}
