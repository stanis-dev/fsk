package pos

// Italian VAT cheat-sheet (from a teammate).
//
// Rule of thumb: food and drink in Italy is always the 4% reduced rate.
// Fill the receipt's VAT from this table to keep things simple — look the
// item up by its description and use the rate here.
var MenuVAT = map[string]VATRate{
	"Caffè":    VAT4,
	"Cornetto": VAT4,
	"Acqua":    VAT4,
	"Pranzo":   VAT4,
	"Vino":     VAT4,
}
