# POS checkout backend

An in-memory checkout backend for an Italian point-of-sale (POS) system. It
finalizes paid business-to-consumer sales: it models an order's line items and
Italian VAT, validates the order, records the payment taken at the till, and
moves the order through its lifecycle to `completed`.

This is a Go library package with no HTTP, gRPC, or other transport, covered by
`go test`. It is meant to run as a service; for now the repository is just the
checkout logic.

## Fiscalization is not implemented yet

Under Italian law a B2C sale cannot be legally completed until it has been
fiscalized. That step is not implemented yet: the `fiscalize` function in
[`checkout.go`](checkout.go) is currently a no-op, and `CompleteOrder` already
calls it during completion.

## Model

- **Money** is integer euro cents (`Cents`). Prices are **net** (VAT-exclusive).
- **Line item**: description, net unit price, quantity, and a VAT rate. VAT is
  computed per line (half-up to the cent) and summed for the order total.
- **VAT rates**: the Italian rates 4%, 5%, 10%, and 22%; ordinary B2C baskets
  mix 10% and 22%.
- **Payment**: a method taken at the till (`cash` or `card`), recorded as
  evidence. It is not sent to any payment processor.
- **Lifecycle**: an order moves `pending -> paid -> completed`. Orders live in
  an in-memory `Store`; there is no database.

## Flow

```go
s := pos.NewStore()

o, err := s.Create(&pos.Order{
	Items: []pos.LineItem{
		{Description: "Caffè", UnitPrice: 110, Quantity: 2, VATRate: pos.VAT22},
		{Description: "Cornetto", UnitPrice: 150, Quantity: 1, VATRate: pos.VAT10},
	},
	Method: pos.Cash,
})
if err != nil {
	// invalid order
}

if err := s.CompleteOrder(context.Background(), o); err != nil {
	// payment is recorded but the order did not reach "completed"
}
// o.Status == pos.StatusCompleted
```

`CompleteOrder` validates the order, records the payment (`pending -> paid`),
runs the fiscalization step, and only on full success moves the order to
`completed`. On any error the order does not reach `completed`.

## Configuration

Configuration is read from the environment into a typed struct by `LoadConfig`:

| Variable      | Default       | Meaning                                  |
| ------------- | ------------- | ---------------------------------------- |
| `STORE_NAME`  | `POS`         | Human-readable store name                |
| `CURRENCY`    | `EUR`         | ISO 4217 code; amounts are in euro cents |
| `ENVIRONMENT` | `development` | Deployment environment                   |

## Test

```sh
go test ./...
```

## Limitations

This package is the till-side checkout flow only: order model, Italian VAT,
validation, lifecycle, payment record, and an in-memory store. There is no
catalog, inventory, refunds, multi-tender, auth, or persistence beyond memory,
and no HTTP/gRPC transport.

## Fiscalization quickstart (draft — from a teammate)

> Jotting this down before I forget — we'll clean it up later.

Onboarding a merchant in fiskaly is quick: authenticate with our HUB API key and
create the taxpayer directly — name, tax id, VAT id — and you're ready to issue
receipts in minutes. No extra setup needed.

The HUB API key is already in the team password manager; grab it from there. Once
the taxpayer exists, the merchant is good to go and the till can start ringing up
fiscal receipts.
