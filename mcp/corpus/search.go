package corpus

// bm25 and newBM25 are completed in the search task; this minimal form lets the
// corpus package compile and its loader/lookup tests run independently.
type bm25 struct{}

func newBM25(secs []Section) *bm25 { return &bm25{} }
