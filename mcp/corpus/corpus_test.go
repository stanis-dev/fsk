package corpus

import "testing"

func TestLoadEmbeddedCorpusIsNonEmpty(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := c.Lookup("probe:records-flow"); !ok {
		t.Fatal("expected seed section probe:records-flow to be present")
	}
}

func TestLookupUnknownID(t *testing.T) {
	c := New([]Section{{ID: "a", Title: "A", Text: "x"}})
	if _, ok := c.Lookup("missing"); ok {
		t.Fatal("expected Lookup of unknown id to return ok=false")
	}
}
