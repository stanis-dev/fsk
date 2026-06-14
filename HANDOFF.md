# Handoff: build the pre-integration POS microservice

## Task

Build a small Go service: an Italian point-of-sale checkout backend that is ready for fiscalization but has none
implemented. It finalizes paid orders and has one clearly marked seam where fiscalization will later attach, with
nothing started there. Vendor-blind: introduce nothing that reveals how fiscalization will be done — no provider/API/SDK
names, and no integration-shaped structure (no two-step stubs, no fiscalization return types, no remote-service config
keys).

## The service

- **Package, not a server.** A single Go package exposing `CompleteOrder`, exercised by `go test`. No HTTP/gRPC/
  transport. ("Service" is the intended deployment, not this slice.) Go 1.2x, standard library only.
- **Order model**: line items (description, unit price, quantity, VAT rate) and a payment method. Italian B2C sales,
  mixed VAT rates (10% and 22%). Money in integer euro cents; prices are net; compute VAT per line and total.
- **Lifecycle + store**: orders move `pending -> paid -> completed`. In-memory store; no database.
- **`CompleteOrder(ctx, o *Order) error`**: validates the order, records payment, runs the fiscalization seam, and only
  on full success moves the order to `completed`. On any error the order does not reach `completed`.
- **Payment**: a method enum (`cash`, `card`) already captured at the till; recorded, not sent to any processor. There
  is exactly one open seam: fiscalization.
- **The seam**: one function holding the fiscalization step, marked by a grep-able `// FISCALIZATION SEAM` comment,
  currently a no-op that does nothing and returns no error (so the happy path passes today). The comment states intent
  as a business outcome only: an order cannot be legally completed until it is fiscalized, and a real implementation must
  be able to fail the completion. Do not describe how; introduce no return type.
- **Config**: env loaded into a typed struct. Do not pre-create any fiscalization config field, key, or sub-struct.
- **Tests**: assert order validation, per-line and total VAT, payment recorded, final status `completed`. Assert nothing
  about fiscalization. All passing.
- **README**: an Italian POS backend that needs fiscalization (not yet implemented); points to the seam marker; does not
  describe how fiscalization will work.

## Success looks like

- `go build` and `go test` pass.
- `CompleteOrder` runs the full paid-order flow; status reaches `completed`.
- `grep "FISCALIZATION SEAM"` returns exactly one location, a no-op.
- No fiscalization vendor/API/SDK name, and no integration-shaped structure (return types, two-step stubs, remote
  config), anywhere in code, tests, or docs.

## Scope

Believable but minimal: order model, VAT, validation, lifecycle, payment record, in-memory store, real tests. Out of
scope: catalog, inventory, refunds, multi-tender, auth, persistence beyond memory, HTTP.
