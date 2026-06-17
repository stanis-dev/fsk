Finish the fiskaly integration in fiskaly.go and wire it into CompleteOrder so a
paid order is fiscalized before it completes. Checkout may retry on transient
network failures, and that must be safe — we must never issue a receipt twice.
Keep the existing tests green and add tests for the new behavior.
