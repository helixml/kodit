package search

// BatchProgress is called after each batch completes during indexing.
// completed is the running total of documents processed so far;
// total is the overall number of documents to embed.
type BatchProgress func(completed, total int)

// IndexOption configures the behaviour of an Index call.
type IndexOption func(*IndexConfig)

// IndexConfig holds the resolved configuration for an Index call.
type IndexConfig struct {
	progress BatchProgress
}

// NewIndexConfig applies all options and returns the resolved config.
func NewIndexConfig(opts ...IndexOption) IndexConfig {
	var cfg IndexConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Progress returns the progress callback, or nil if none was set.
func (c IndexConfig) Progress() BatchProgress { return c.progress }

// WithProgress registers a callback that is invoked after each batch
// of embeddings is generated and saved.
func WithProgress(fn BatchProgress) IndexOption {
	return func(c *IndexConfig) { c.progress = fn }
}
