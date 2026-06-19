// Package corpus loads and searches the embedded fiskaly SIGN IT documentation corpus.
package corpus

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed index.json
var indexJSON []byte

// Section is one self-contained unit of documentation (one id = one fetch).
type Section struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Text    string `json:"text"`
}

// Corpus is an in-memory, searchable view of the embedded sections.
type Corpus struct {
	sections []Section
	byID     map[string]Section
	index    *bm25
}

// Load parses the embedded index and builds a searchable Corpus.
func Load() (*Corpus, error) {
	var secs []Section
	if err := json.Unmarshal(indexJSON, &secs); err != nil {
		return nil, fmt.Errorf("corpus: parsing embedded index: %w", err)
	}
	if len(secs) == 0 {
		return nil, fmt.Errorf("corpus: embedded index is empty")
	}
	return New(secs), nil
}

// New builds a searchable Corpus from sections.
func New(secs []Section) *Corpus {
	byID := make(map[string]Section, len(secs))
	for _, s := range secs {
		byID[s.ID] = s
	}
	return &Corpus{sections: secs, byID: byID, index: newBM25(secs)}
}

// Lookup returns the section with the given id.
func (c *Corpus) Lookup(id string) (Section, bool) {
	s, ok := c.byID[id]
	return s, ok
}
