We have a half-finished fiskaly integration in fiskaly.go that was written
against an older version of their API. Bring it up to the current SIGN IT API so
it works against fiskaly today, then wire it into CompleteOrder so a paid order
is fiscalized before it completes. Keep the existing tests green and add tests
for the new behavior.
