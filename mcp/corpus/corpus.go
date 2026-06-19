// Package corpus loads and searches the embedded fiskaly SIGN IT documentation corpus.
package corpus

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed index.json
var indexJSON []byte

type Section struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URI     string `json:"uri"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Text    string `json:"text"`
}

type Corpus struct {
	sections []Section
	byID     map[string]Section
	index    *bm25
}

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

func New(secs []Section) *Corpus {
	byID := make(map[string]Section, len(secs))
	for _, s := range secs {
		byID[s.ID] = s
	}
	return &Corpus{sections: secs, byID: byID, index: newBM25(secs)}
}

func (c *Corpus) Lookup(id string) (Section, bool) {
	s, ok := c.byID[id]
	return s, ok
}
