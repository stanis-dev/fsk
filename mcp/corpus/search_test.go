package corpus

import "testing"

func testCorpus() *Corpus {
	return New([]Section{
		{ID: "records", Title: "issuing a receipt: the records flow", Path: "POST /records",
			Text: "a receipt is two POST /records calls, INTENTION then TRANSACTION RECEIPT"},
		{ID: "auth", Title: "authentication and headers", Path: "POST /tokens",
			Text: "POST /tokens returns a JWT; X-Idempotency-Key required on every post"},
		{ID: "money", Title: "VAT and amounts", Path: "",
			Text: "amounts are decimal strings; VatRateCategory requires all fields"},
	})
}

func TestSearchRanksRecordsFirst(t *testing.T) {
	c := testCorpus()
	hits := c.Search("records flow", 8)
	if len(hits) == 0 {
		t.Fatal("expected hits for 'records flow'")
	}
	if hits[0].Section.ID != "records" {
		t.Fatalf("expected 'records' first, got %q", hits[0].Section.ID)
	}
	if hits[0].Snippet == "" {
		t.Fatal("expected a non-empty snippet")
	}
}

func TestSearchEmptyQueryReturnsNil(t *testing.T) {
	if c := testCorpus(); c.Search("   ", 8) != nil {
		t.Fatal("expected nil for whitespace query")
	}
}

func TestSearchNoMatchReturnsEmpty(t *testing.T) {
	if hits := testCorpus().Search("kubernetes helm chart", 8); len(hits) != 0 {
		t.Fatalf("expected no hits, got %d", len(hits))
	}
}

func TestSearchHonorsLimit(t *testing.T) {
	c := testCorpus()
	if hits := c.Search("post", 1); len(hits) > 1 {
		t.Fatalf("expected at most 1 hit, got %d", len(hits))
	}
}
