package corpus

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Hit struct {
	Section Section
	score   float64
	Snippet string
}

const (
	boostTitle = 3
	boostPath  = 2
	boostText  = 1
	k1         = 1.5
	b          = 0.75
	snippetLen = 200
)

type docStats struct {
	tf     map[string]int
	length int
}

type bm25 struct {
	docs  []docStats
	df    map[string]int
	n     int
	avgdl float64
}

func newBM25(secs []Section) *bm25 {
	idx := &bm25{df: map[string]int{}, n: len(secs)}
	var total int
	for _, s := range secs {
		tf := map[string]int{}
		addTokens(tf, s.Title, boostTitle)
		addTokens(tf, s.Path, boostPath)
		addTokens(tf, s.Text, boostText)
		length := 0
		for _, c := range tf {
			length += c
		}
		for term := range tf {
			idx.df[term]++
		}
		idx.docs = append(idx.docs, docStats{tf: tf, length: length})
		total += length
	}
	if idx.n > 0 {
		idx.avgdl = float64(total) / float64(idx.n)
	}
	return idx
}

// Search returns up to limit sections ranked by field-weighted BM25-lite.
// A whitespace-only query returns nil; no matches returns an empty slice.
func (c *Corpus) Search(query string, limit int) []Hit {
	if limit <= 0 {
		limit = 8
	}
	qterms := tokenize(query)
	if len(qterms) == 0 {
		return nil
	}
	hits := []Hit{}
	for i, ds := range c.index.docs {
		score := 0.0
		for _, t := range qterms {
			tf := ds.tf[t]
			if tf == 0 {
				continue
			}
			df := float64(c.index.df[t])
			idf := math.Log(1 + (float64(c.index.n)-df+0.5)/(df+0.5))
			denom := float64(tf) + k1*(1-b+b*float64(ds.length)/c.index.avgdl)
			score += idf * (float64(tf) * (k1 + 1)) / denom
		}
		if score > 0 {
			sec := c.sections[i]
			hits = append(hits, Hit{Section: sec, score: score, Snippet: snippet(sec.Text, qterms)})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func addTokens(tf map[string]int, s string, boost int) {
	for _, t := range tokenize(s) {
		tf[t] += boost
	}
}

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func snippet(text string, qterms []string) string {
	runes := []rune(text)
	lower := strings.ToLower(text)
	firstMatchByte := -1
	for _, t := range qterms {
		if i := strings.Index(lower, t); i >= 0 && (firstMatchByte < 0 || i < firstMatchByte) {
			firstMatchByte = i
		}
	}
	start := 0
	if firstMatchByte > 0 {
		start = min(utf8.RuneCountInString(lower[:firstMatchByte]), len(runes))
	}
	end := min(start+snippetLen, len(runes))
	out := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}
