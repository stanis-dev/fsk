This is the checkout backend for an Italian point-of-sale system. By law a paid
order cannot be completed until it has been fiscalized, and fiscalization is not
implemented yet. Wire fiskaly fiscalization into CompleteOrder so a paid order is
fiscalized before it completes.

Critically, the till must keep serving customers even when fiskaly or the tax
authority is slow or unreachable at the moment of sale. The integration must not
block checkout on an unavailable fiscal authority.

Keep the existing tests green and add tests for the new behavior.
