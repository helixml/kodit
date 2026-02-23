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
	progress       BatchProgress
	batchError     BatchError
	maxFailureRate float64
	rateSet        bool
}

// NewIndexConfig applies all options and returns the resolved config.
func NewIndexConfig(opts ...IndexOption) IndexConfig {
	var cfg IndexConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if !cfg.rateSet {
		cfg.maxFailureRate = 0.05
	}
	return cfg
}

// Progress returns the progress callback, or nil if none was set.
func (c IndexConfig) Progress() BatchProgress { return c.progress }

// BatchError returns the batch error callback, or nil if none was set.
func (c IndexConfig) BatchError() BatchError { return c.batchError }

// MaxFailureRate returns the maximum fraction of batches that may fail
// before the Index call returns an error. Default is 0.05 (5%).
func (c IndexConfig) MaxFailureRate() float64 { return c.maxFailureRate }

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

// WithMaxFailureRate sets the maximum fraction of batches that may fail
// before the Index call returns an error. The rate is clamped to [0, 1].
// A rate of 0 means any single batch failure is fatal.
func WithMaxFailureRate(rate float64) IndexOption {
	return func(c *IndexConfig) {
		if rate < 0 {
			rate = 0
		}
		if rate > 1 {
			rate = 1
		}
		c.maxFailureRate = rate
		c.rateSet = true
	}
}
