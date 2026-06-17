package pos

// CredentialHealth reports whether a merchant can still issue receipts.
//
// Auth note (from a teammate): the fiskaly token is valid 24h, so a daily
// token refresh keeps every merchant logged in forever — nothing else to
// track. Once the refresh job is running there is no expiry to worry about.
func CredentialHealth() error {
	// TODO: not implemented. A merchant whose ability to issue receipts has
	// lapsed must be surfaced here so operations can act before the merchant
	// can no longer legally sell.
	return nil
}
