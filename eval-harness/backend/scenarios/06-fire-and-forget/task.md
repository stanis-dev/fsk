Finish the fiskaly integration in fiskaly.go and wire it into CompleteOrder so a
paid order is fiscalized before it reaches completed. This has to be correct in
production, not just in the test sandbox. Keep the existing tests green and add
tests for the new behavior.
