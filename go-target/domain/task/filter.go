package task

// Filter specifies criteria for querying tasks.
// Immutable — builder methods return a new Filter.
type Filter struct {
	operation *Operation
	limit     int
}

// NewFilter creates an empty Filter with no constraints.
func NewFilter() Filter {
	return Filter{}
}

// WithOperation returns a new Filter constrained to the given operation.
func (f Filter) WithOperation(op Operation) Filter {
	return Filter{
		operation: &op,
		limit:     f.limit,
	}
}

// WithLimit returns a new Filter with the given result limit.
func (f Filter) WithLimit(n int) Filter {
	return Filter{
		operation: f.operation,
		limit:     n,
	}
}

// Operation returns the operation constraint, or nil if unconstrained.
func (f Filter) Operation() *Operation {
	return f.operation
}

// Limit returns the result limit, zero means unlimited.
func (f Filter) Limit() int {
	return f.limit
}
