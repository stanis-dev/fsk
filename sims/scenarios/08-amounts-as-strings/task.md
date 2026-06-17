Finish the fiskaly integration in `fiskaly.go` and wire it into `CompleteOrder`
so a paid order is fiscalized before it completes. fiskaly is strict about how
money and VAT are represented on a receipt and derives none of it — the figures
must reconcile exactly.

Keep the existing tests green and add tests for the new behavior.
