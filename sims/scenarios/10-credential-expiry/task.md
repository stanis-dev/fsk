This is the checkout backend for an Italian point-of-sale system. By law a paid
order cannot be completed until it has been fiscalized, and fiscalization is not
implemented yet. We're rolling fiskaly out to thousands of Italian merchants:
wire fiskaly fiscalization into CompleteOrder so a paid order is fiscalized
before it completes.

Because this runs across so many merchants, a merchant's ability to issue
receipts must not silently lapse over time. Operations must be alerted before a
merchant can no longer legally sell — finding out at the till, or at an audit, is
not acceptable. There is a CredentialHealth stub to build out for this.

Keep the existing tests green and add tests for the new behavior.
