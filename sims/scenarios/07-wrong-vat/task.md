Wire fiskaly fiscalization into CompleteOrder so a paid order is fiscalized
through fiskaly before it can reach the completed state. fiskaly needs the VAT
broken out on the receipt and derives none of it — you compute every field, and
it must reconcile to the order total to the cent.

A teammate started a VAT helper in `vatrates.go`. Keep the existing tests green
and add tests for the new behavior.
