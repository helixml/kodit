package search

// BatchProgress is called after each batch completes during indexing.
// completed is the running total of documents processed so far;
// total is the overall number of documents to embed.
type BatchProgress func(completed, total int)

// BatchError is called when a batch fails during indexing.
// batchStart and batchEnd are the document offsets of the failed batch;
// err is the upstream error (e.g. HTTP 429, timeout, auth failure).
type BatchError func(batchStart, batchEnd int, err error)

// IndexOption configures the behaviour of an Index call.
type IndexOption func(*IndexConfig)

// IndexConfig holds the resolved configuration for an Index call.
type IndexConfig struct {
	progress   BatchProgress
	batchError BatchError
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

// BatchError returns the batch error callback, or nil if none was set.
func (c IndexConfig) BatchError() BatchError { return c.batchError }

// WithProgress registers a callback that is invoked after each batch
// of embeddings is generated and saved.
func WithProgress(fn BatchProgress) IndexOption {
	return func(c *IndexConfig) { c.progress = fn }
}

// WithBatchError registers a callback that is invoked when an individual
// batch fails during indexing. This allows callers to log each upstream
// error (HTTP status, timeout, etc.) as it occurs.
func WithBatchError(fn BatchError) IndexOption {
	return func(c *IndexConfig) { c.batchError = fn }
}
