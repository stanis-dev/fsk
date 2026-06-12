package fiskaly

import "testing"

func TestParseCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		err  bool
	}{
		{"12.20", 1220, false},
		{"2.5", 250, false},
		{"3", 300, false},
		{"0.01", 1, false},
		{"-1.50", -150, false},
		{"1.234", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		got, err := parseCents(c.in)
		if c.err != (err != nil) {
			t.Errorf("parseCents(%q): err=%v, want err=%v", c.in, err, c.err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("parseCents(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	for in, want := range map[int64]string{1220: "12.20", 1: "0.01", -150: "-1.50", 0: "0.00"} {
		if got := formatCents(in); got != want {
			t.Errorf("formatCents(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestNetFromGross(t *testing.T) {
	cases := []struct {
		gross, bp, want int64
	}{
		{1220, 2200, 1000}, // 12.20 gross at 22% -> 10.00 net exactly
		{250, 1000, 227},   // 2.50 at 10% -> 2.27 (2.2727... rounds down)
		{120, 1000, 109},   // 1.20 at 10% -> 1.09
		{100, 2200, 82},    // 1.00 at 22% -> 0.8196 rounds to 0.82
	}
	for _, c := range cases {
		if got := netFromGross(c.gross, c.bp); got != c.want {
			t.Errorf("netFromGross(%d, %d) = %d, want %d", c.gross, c.bp, got, c.want)
		}
	}
}

func TestBuildReceiptTotals(t *testing.T) {
	op, err := BuildReceipt("42", []ReceiptItem{
		{Text: "Spaghetti alle vongole", Gross: "14.50"},
		{Text: "Acqua frizzante", Gross: "2.50", VatCode: VatCodeReduced1},
		{Text: "Caffè", Gross: "1.20", VatCode: VatCodeReduced1},
	})
	if err != nil {
		t.Fatal(err)
	}
	tv := op.Document.TotalVat
	if tv.Inclusive != "18.20" || tv.Exclusive != "15.25" || tv.Amount != "2.95" {
		t.Errorf("totals = %+v, want inclusive=18.20 exclusive=15.25 amount=2.95", tv)
	}
	// Per-line breakdowns must sum exactly to the document totals.
	var net, vat int64
	for _, e := range op.Entries {
		n, _ := parseCents(e.Data.Vat.Exclusive)
		v, _ := parseCents(e.Data.Vat.Amount)
		net, vat = net+n, vat+v
	}
	if formatCents(net) != tv.Exclusive || formatCents(vat) != tv.Amount {
		t.Errorf("line sums (net=%s vat=%s) != document totals (%s, %s)",
			formatCents(net), formatCents(vat), tv.Exclusive, tv.Amount)
	}
	if op.Payments[0].Details.Amount != "18.20" {
		t.Errorf("payment = %s, want 18.20", op.Payments[0].Details.Amount)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Trattoria Da Mario":                     "trattoria-da-mario",
		"Café Über! GmbH":                        "caf-ber-gmbh",
		"ab":                                     "ab0",
		"A Very Long Merchant Name That Exceeds": "a-very-long-merchant-name-that",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
