package fiskaly

import (
	"fmt"
	"strconv"
	"strings"
)

// Italian VAT rates by spec code, in basis points.
var vatBasisPoints = map[string]int64{
	VatCodeStandard: 2200,
	VatCodeReduced1: 1000,
	VatCodeReduced2: 500,
	VatCodeReduced3: 400,
}

func VatPercentage(code string) (string, error) {
	bp, ok := vatBasisPoints[code]
	if !ok {
		return "", fmt.Errorf("unknown VAT code %q (want STANDARD, REDUCED_1, REDUCED_2 or REDUCED_3)", code)
	}
	return fmt.Sprintf("%d.%02d", bp/100, bp%100), nil
}

// parseCents parses a decimal string like "12.20" into cents, rejecting
// more than two decimal places (receipt amounts are euro cents).
func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	switch len(frac) {
	case 0:
		frac = "00"
	case 1:
		frac += "0"
	case 2:
	default:
		return 0, fmt.Errorf("amount %q has more than two decimal places", s)
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	cents := w*100 + f
	if neg {
		cents = -cents
	}
	return cents, nil
}

func formatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// netFromGross derives the net amount from a gross amount at the given VAT
// rate, rounding half-up to the cent: net = gross / (1 + rate).
func netFromGross(grossCents, bp int64) int64 {
	num := grossCents * 10000
	den := 10000 + bp
	return (num + den/2) / den
}
