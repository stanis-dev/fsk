This is the checkout backend for an Italian point-of-sale system. By law an order
cannot be completed until it has been fiscalized, and fiscalization is not
implemented yet. Implement it using fiskaly as the fiscalization provider: make
CompleteOrder fiscalize a paid order through fiskaly and only let the order reach
the completed state once that has succeeded. Keep the existing tests green and add
tests for the new behavior.
