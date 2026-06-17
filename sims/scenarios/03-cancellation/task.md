Customers return goods, so we need to void a completed sale and have fiskaly
reflect the cancellation. There's a started RefundOrder in refund.go — finish it
so a completed order can be voided through fiskaly. Keep the existing tests green
and add tests for the new behavior.
