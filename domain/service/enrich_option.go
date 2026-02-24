package service

// EnrichProgress is called after each request completes during enrichment.
// completed is the running total of requests processed so far;
// total is the overall number of requests to enrich.
type EnrichProgress func(completed, total int)

// RequestError is called when an individual request fails during enrichment.
// requestID is the identifier of the failed request; err is the upstream error.
type RequestError func(requestID string, err error)

// EnrichOption configures the behaviour of an Enrich call.
type EnrichOption func(*EnrichConfig)

// EnrichConfig holds the resolved configuration for an Enrich call.
type EnrichConfig struct {
	progress       EnrichProgress
	requestError   RequestError
	maxFailureRate float64
	rateSet        bool
}

// NewEnrichConfig applies all options and returns the resolved config.
func NewEnrichConfig(opts ...EnrichOption) EnrichConfig {
	var cfg EnrichConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if !cfg.rateSet {
		cfg.maxFailureRate = 0.05
	}
	return cfg
}

// Progress returns the progress callback, or nil if none was set.
func (c EnrichConfig) Progress() EnrichProgress { return c.progress }

// RequestError returns the request error callback, or nil if none was set.
func (c EnrichConfig) RequestError() RequestError { return c.requestError }

// MaxFailureRate returns the maximum fraction of requests that may fail
// before the Enrich call returns an error. Default is 0.05 (5%).
func (c EnrichConfig) MaxFailureRate() float64 { return c.maxFailureRate }

// WithEnrichProgress registers a callback that is invoked after each
// enrichment request completes successfully.
func WithEnrichProgress(fn EnrichProgress) EnrichOption {
	return func(c *EnrichConfig) { c.progress = fn }
}

// WithRequestError registers a callback that is invoked when an individual
// request fails during enrichment. This allows callers to log each upstream
// error as it occurs.
func WithRequestError(fn RequestError) EnrichOption {
	return func(c *EnrichConfig) { c.requestError = fn }
}

// WithMaxFailureRate sets the maximum fraction of requests that may fail
// before the Enrich call returns an error. The rate is clamped to [0, 1].
// A rate of 0 means any single request failure is fatal.
func WithMaxFailureRate(rate float64) EnrichOption {
	return func(c *EnrichConfig) {
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
