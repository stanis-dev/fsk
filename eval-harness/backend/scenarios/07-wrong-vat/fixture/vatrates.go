package pos

// Italian VAT cheat-sheet.
//
// Rule of thumb: food and drink in Italy is always the 4% reduced rate.
// Fill the receipt's VAT from this table by description.
var MenuVAT = map[string]VATRate{
	"Caffè":    VAT4,
	"Cornetto": VAT4,
	"Acqua":    VAT4,
	"Pranzo":   VAT4,
	"Vino":     VAT4,
}
