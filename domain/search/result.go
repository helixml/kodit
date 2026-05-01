package search

// Result represents a generic search result.
type Result struct {
	snippetID string
	score     float64
}

// NewResult creates a new Result.
func NewResult(snippetID string, score float64) Result {
	return Result{
		snippetID: snippetID,
		score:     score,
	}
}

// SnippetID returns the snippet ID.
func (r Result) SnippetID() string { return r.snippetID }

// Score returns the search score.
func (r Result) Score() float64 { return r.score }

// FusionRequest represents a fusion request input.
type FusionRequest struct {
	id    string
	score float64
}

// NewFusionRequest creates a new FusionRequest.
func NewFusionRequest(id string, score float64) FusionRequest {
	return FusionRequest{
		id:    id,
		score: score,
	}
}

// ID returns the document ID.
func (f FusionRequest) ID() string { return f.id }

// Score returns the score.
func (f FusionRequest) Score() float64 { return f.score }

// FusionResult represents a fusion result.
type FusionResult struct {
	id             string
	score          float64
	originalScores []float64
}

// NewFusionResult creates a new FusionResult.
func NewFusionResult(id string, score float64, originalScores []float64) FusionResult {
	scores := make([]float64, len(originalScores))
	copy(scores, originalScores)
	return FusionResult{
		id:             id,
		score:          score,
		originalScores: scores,
	}
}

// ID returns the document ID.
func (f FusionResult) ID() string { return f.id }

// Score returns the fused score.
func (f FusionResult) Score() float64 { return f.score }

// OriginalScores returns the original scores from each search method.
func (f FusionResult) OriginalScores() []float64 {
	scores := make([]float64, len(f.originalScores))
	copy(scores, f.originalScores)
	return scores
}

// Document is a unit of search content. text is read by BM25 stores;
// vector is read by embedding stores. A given Document carries one or
// the other; the unused field is ignored by the implementation that
// does not need it.
type Document struct {
	snippetID string
	text      string
	vector    []float64
}

// NewDocument creates a Document carrying text — used for BM25 indexing.
func NewDocument(snippetID, text string) Document {
	return Document{
		snippetID: snippetID,
		text:      text,
	}
}

// NewVectorDocument creates a Document carrying a precomputed vector —
// used for embedding stores.
func NewVectorDocument(snippetID string, vector []float64) Document {
	cp := make([]float64, len(vector))
	copy(cp, vector)
	return Document{
		snippetID: snippetID,
		vector:    cp,
	}
}

// SnippetID returns the snippet ID.
func (d Document) SnippetID() string { return d.snippetID }

// Text returns the document text (empty for vector documents).
func (d Document) Text() string { return d.text }

// Vector returns a defensive copy of the embedding vector
// (nil for text documents).
func (d Document) Vector() []float64 {
	cp := make([]float64, len(d.vector))
	copy(cp, d.vector)
	return cp
}
