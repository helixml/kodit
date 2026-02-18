package repository

import "fmt"

// Option applies a modification to a Query.
type Option func(Query) Query

// Query holds conditions, ordering, and pagination for store lookups.
type Query struct {
	conditions []Condition
	orders     []Order
	limit      int
	offset     int
	params     map[string]any
}

// Build creates a Query from a set of options.
func Build(options ...Option) Query {
	q := Query{}
	for _, opt := range options {
		q = opt(q)
	}
	return q
}

// Conditions returns the query conditions.
func (q Query) Conditions() []Condition {
	result := make([]Condition, len(q.conditions))
	copy(result, q.conditions)
	return result
}

// Orders returns the query ordering specifications.
func (q Query) Orders() []Order {
	result := make([]Order, len(q.orders))
	copy(result, q.orders)
	return result
}

// LimitValue returns the limit (0 means no limit).
func (q Query) LimitValue() int {
	return q.limit
}

// OffsetValue returns the offset.
func (q Query) OffsetValue() int {
	return q.offset
}

// Condition represents a single query condition (equality or IN).
type Condition struct {
	field string
	value any
	in    bool
}

// Field returns the condition field name.
func (c Condition) Field() string { return c.field }

// Value returns the condition value.
func (c Condition) Value() any { return c.value }

// In returns true if this is an IN condition (value is a slice).
func (c Condition) In() bool { return c.in }

// String returns a readable representation.
func (c Condition) String() string {
	if c.in {
		return fmt.Sprintf("%s IN %v", c.field, c.value)
	}
	return fmt.Sprintf("%s = %v", c.field, c.value)
}

// Order represents a sort specification.
type Order struct {
	field     string
	ascending bool
}

// Field returns the order field name.
func (o Order) Field() string { return o.field }

// Ascending returns true for ASC, false for DESC.
func (o Order) Ascending() bool { return o.ascending }

// --- Generic options reused across all stores ---

// WithCondition adds a field = value equality condition.
// Domain packages use this to define their own typed options.
func WithCondition(field string, value any) Option {
	return func(q Query) Query {
		q.conditions = append(q.conditions, Condition{field: field, value: value})
		return q
	}
}

// WithConditionIn adds a field IN (values) condition.
func WithConditionIn(field string, values any) Option {
	return func(q Query) Query {
		q.conditions = append(q.conditions, Condition{field: field, value: values, in: true})
		return q
	}
}

// WithID filters by the "id" column.
func WithID(id int64) Option {
	return WithCondition("id", id)
}

// WithIDIn filters by the "id" column using IN.
func WithIDIn(ids []int64) Option {
	return WithConditionIn("id", ids)
}

// WithRepoID filters by the "repo_id" column.
func WithRepoID(id int64) Option {
	return WithCondition("repo_id", id)
}

// WithLimit sets the maximum number of results.
func WithLimit(n int) Option {
	return func(q Query) Query {
		q.limit = n
		return q
	}
}

// WithOffset sets the result offset.
func WithOffset(n int) Option {
	return func(q Query) Query {
		q.offset = n
		return q
	}
}

// WithOrderAsc adds ascending ordering on a field.
func WithOrderAsc(field string) Option {
	return func(q Query) Query {
		q.orders = append(q.orders, Order{field: field, ascending: true})
		return q
	}
}

// WithOrderDesc adds descending ordering on a field.
func WithOrderDesc(field string) Option {
	return func(q Query) Query {
		q.orders = append(q.orders, Order{field: field, ascending: false})
		return q
	}
}

// WithPagination returns limit and offset options for a page.
func WithPagination(limit, offset int) []Option {
	return []Option{WithLimit(limit), WithOffset(offset)}
}

// WithParam stores an arbitrary key-value pair on the query.
// Domain packages define typed option builders on top of this.
func WithParam(key string, value any) Option {
	return func(q Query) Query {
		if q.params == nil {
			q.params = make(map[string]any)
		}
		q.params[key] = value
		return q
	}
}

// Param retrieves a parameter by key.
func (q Query) Param(key string) (any, bool) {
	if q.params == nil {
		return nil, false
	}
	v, ok := q.params[key]
	return v, ok
}
