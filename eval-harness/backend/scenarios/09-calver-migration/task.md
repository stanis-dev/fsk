We have a half-finished fiskaly integration in fiskaly.go that was written
against an older version of their API. Bring it up to SIGN IT API version
2026-02-03, then wire it into CompleteOrder so a paid order is fiscalized before
it completes. Keep the existing tests green and add tests for the new behavior.
