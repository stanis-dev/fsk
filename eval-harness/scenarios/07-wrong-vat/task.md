Wire fiskaly fiscalization into CompleteOrder so a paid order is fiscalized
through fiskaly before it can reach the completed state. fiskaly needs the VAT
broken out on the receipt and derives none of it; compute every field, and
it must reconcile to the order total to the cent.

A draft VAT helper lives in `vatrates.go`. Keep the existing tests green
and add tests for the new behavior.
