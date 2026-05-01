package repository

import "fmt"

// Option applies a modification to a Query.
type Option func(Query) Query

// Query holds conditions, ordering, and pagination for store lookups.
type Query struct {
	conditions []Condition
	clauses    []Clause
	selects    []Select
	joins      []Join
	orders     []Order
	rawOrders  []string
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

// Clauses returns the raw WHERE clauses.
func (q Query) Clauses() []Clause {
	result := make([]Clause, len(q.clauses))
	copy(result, q.clauses)
	return result
}

// Selects returns the SELECT expressions.
func (q Query) Selects() []Select {
	result := make([]Select, len(q.selects))
	copy(result, q.selects)
	return result
}

// Joins returns the JOIN clauses.
func (q Query) Joins() []Join {
	result := make([]Join, len(q.joins))
	copy(result, q.joins)
	return result
}

// RawOrders returns the raw ORDER BY expressions.
func (q Query) RawOrders() []string {
	result := make([]string, len(q.rawOrders))
	copy(result, q.rawOrders)
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

// Clause represents a raw WHERE clause with arguments.
type Clause struct {
	sql  string
	args []any
}

// SQL returns the clause SQL expression.
func (c Clause) SQL() string { return c.sql }

// Args returns the clause bind arguments.
func (c Clause) Args() []any { return c.args }

// Select represents a SELECT expression with bind arguments.
// Used to inject custom column projections (e.g., similarity-score columns
// for vector or BM25 search) into a Find query.
type Select struct {
	expr string
	args []any
}

// Expr returns the SELECT expression.
func (s Select) Expr() string { return s.expr }

// Args returns the SELECT bind arguments.
func (s Select) Args() []any { return s.args }

// Join represents a JOIN clause with optional bind arguments.
type Join struct {
	expr string
	args []any
}

// Expr returns the JOIN expression.
func (j Join) Expr() string { return j.expr }

// Args returns the JOIN bind arguments.
func (j Join) Args() []any { return j.args }

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

// WithWhere adds a raw WHERE clause with bind arguments.
func WithWhere(sql string, args ...any) Option {
	return func(q Query) Query {
		q.clauses = append(q.clauses, Clause{sql: sql, args: args})
		return q
	}
}

// WithSelect adds a SELECT expression with optional bind arguments.
// Multiple WithSelect options are concatenated with commas.
//
// Used for custom projections — e.g. injecting a similarity-score column
// for vector or BM25 search:
//
//	repository.WithSelect("snippet_id, embedding <=> ? AS score", queryVec)
func WithSelect(expr string, args ...any) Option {
	return func(q Query) Query {
		q.selects = append(q.selects, Select{expr: expr, args: args})
		return q
	}
}

// WithJoin adds a JOIN clause with optional bind arguments.
func WithJoin(expr string, args ...any) Option {
	return func(q Query) Query {
		q.joins = append(q.joins, Join{expr: expr, args: args})
		return q
	}
}

// WithRawOrder adds a raw ORDER BY expression (e.g. "score DESC").
// Use this when the ordering target is a SELECT-injected column or expression
// that does not have a stable column name.
func WithRawOrder(expr string) Option {
	return func(q Query) Query {
		q.rawOrders = append(q.rawOrders, expr)
		return q
	}
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
