package database

import (
	"fmt"

	"gorm.io/gorm"
)

// FilterOperator represents SQL comparison operators.
type FilterOperator int

// FilterOperator values.
const (
	OpEqual FilterOperator = iota
	OpNotEqual
	OpGreaterThan
	OpGreaterThanOrEqual
	OpLessThan
	OpLessThanOrEqual
	OpLike
	OpILike
	OpIn
	OpNotIn
	OpIsNull
	OpIsNotNull
	OpBetween
)

// String returns the SQL representation of the operator.
func (o FilterOperator) String() string {
	switch o {
	case OpEqual:
		return "="
	case OpNotEqual:
		return "!="
	case OpGreaterThan:
		return ">"
	case OpGreaterThanOrEqual:
		return ">="
	case OpLessThan:
		return "<"
	case OpLessThanOrEqual:
		return "<="
	case OpLike:
		return "LIKE"
	case OpILike:
		return "ILIKE"
	case OpIn:
		return "IN"
	case OpNotIn:
		return "NOT IN"
	case OpIsNull:
		return "IS NULL"
	case OpIsNotNull:
		return "IS NOT NULL"
	case OpBetween:
		return "BETWEEN"
	default:
		return "="
	}
}

// Filter represents a single query filter condition.
type Filter struct {
	field    string
	operator FilterOperator
	value    any
	value2   any // Used for BETWEEN operator
}

// NewFilter creates a new Filter.
func NewFilter(field string, operator FilterOperator, value any) Filter {
	return Filter{
		field:    field,
		operator: operator,
		value:    value,
	}
}

// NewBetweenFilter creates a new BETWEEN Filter.
func NewBetweenFilter(field string, low, high any) Filter {
	return Filter{
		field:    field,
		operator: OpBetween,
		value:    low,
		value2:   high,
	}
}

// Field returns the filter field name.
func (f Filter) Field() string { return f.field }

// Operator returns the filter operator.
func (f Filter) Operator() FilterOperator { return f.operator }

// Value returns the filter value.
func (f Filter) Value() any { return f.value }

// SortDirection represents sort direction.
type SortDirection int

// SortDirection values.
const (
	SortAsc SortDirection = iota
	SortDesc
)

// String returns the SQL representation.
func (s SortDirection) String() string {
	if s == SortDesc {
		return "DESC"
	}
	return "ASC"
}

// OrderBy represents a sort specification.
type OrderBy struct {
	field     string
	direction SortDirection
}

// NewOrderBy creates a new OrderBy.
func NewOrderBy(field string, direction SortDirection) OrderBy {
	return OrderBy{
		field:     field,
		direction: direction,
	}
}

// Field returns the field name.
func (o OrderBy) Field() string { return o.field }

// Direction returns the sort direction.
func (o OrderBy) Direction() SortDirection { return o.direction }

// Query represents a database query with filters, ordering, and pagination.
type Query struct {
	filters []Filter
	orderBy []OrderBy
	limit   int
	offset  int
}

// NewQuery creates a new empty Query.
func NewQuery() Query {
	return Query{}
}

// Where adds a filter condition.
func (q Query) Where(field string, operator FilterOperator, value any) Query {
	q.filters = append(q.filters, NewFilter(field, operator, value))
	return q
}

// WhereBetween adds a BETWEEN filter.
func (q Query) WhereBetween(field string, low, high any) Query {
	q.filters = append(q.filters, NewBetweenFilter(field, low, high))
	return q
}

// Equal adds an equality filter.
func (q Query) Equal(field string, value any) Query {
	return q.Where(field, OpEqual, value)
}

// NotEqual adds a not-equal filter.
func (q Query) NotEqual(field string, value any) Query {
	return q.Where(field, OpNotEqual, value)
}

// GreaterThan adds a greater-than filter.
func (q Query) GreaterThan(field string, value any) Query {
	return q.Where(field, OpGreaterThan, value)
}

// GreaterThanOrEqual adds a greater-than-or-equal filter.
func (q Query) GreaterThanOrEqual(field string, value any) Query {
	return q.Where(field, OpGreaterThanOrEqual, value)
}

// LessThan adds a less-than filter.
func (q Query) LessThan(field string, value any) Query {
	return q.Where(field, OpLessThan, value)
}

// LessThanOrEqual adds a less-than-or-equal filter.
func (q Query) LessThanOrEqual(field string, value any) Query {
	return q.Where(field, OpLessThanOrEqual, value)
}

// Like adds a LIKE filter.
func (q Query) Like(field string, pattern string) Query {
	return q.Where(field, OpLike, pattern)
}

// ILike adds a case-insensitive LIKE filter.
func (q Query) ILike(field string, pattern string) Query {
	return q.Where(field, OpILike, pattern)
}

// In adds an IN filter.
func (q Query) In(field string, values any) Query {
	return q.Where(field, OpIn, values)
}

// NotIn adds a NOT IN filter.
func (q Query) NotIn(field string, values any) Query {
	return q.Where(field, OpNotIn, values)
}

// IsNull adds an IS NULL filter.
func (q Query) IsNull(field string) Query {
	return q.Where(field, OpIsNull, nil)
}

// IsNotNull adds an IS NOT NULL filter.
func (q Query) IsNotNull(field string) Query {
	return q.Where(field, OpIsNotNull, nil)
}

// Order adds an ordering specification.
func (q Query) Order(field string, direction SortDirection) Query {
	q.orderBy = append(q.orderBy, NewOrderBy(field, direction))
	return q
}

// OrderAsc adds ascending ordering.
func (q Query) OrderAsc(field string) Query {
	return q.Order(field, SortAsc)
}

// OrderDesc adds descending ordering.
func (q Query) OrderDesc(field string) Query {
	return q.Order(field, SortDesc)
}

// Limit sets the result limit.
func (q Query) Limit(limit int) Query {
	q.limit = limit
	return q
}

// Offset sets the result offset.
func (q Query) Offset(offset int) Query {
	q.offset = offset
	return q
}

// Paginate sets both limit and offset for pagination.
func (q Query) Paginate(page, pageSize int) Query {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	q.limit = pageSize
	q.offset = (page - 1) * pageSize
	return q
}

// Filters returns all filter conditions.
func (q Query) Filters() []Filter {
	result := make([]Filter, len(q.filters))
	copy(result, q.filters)
	return result
}

// Orders returns all ordering specifications.
func (q Query) Orders() []OrderBy {
	result := make([]OrderBy, len(q.orderBy))
	copy(result, q.orderBy)
	return result
}

// LimitValue returns the limit value (0 means no limit).
func (q Query) LimitValue() int {
	return q.limit
}

// OffsetValue returns the offset value.
func (q Query) OffsetValue() int {
	return q.offset
}

// Apply applies the query to a GORM database session.
func (q Query) Apply(db *gorm.DB) *gorm.DB {
	result := db

	for _, filter := range q.filters {
		result = applyFilter(result, filter)
	}

	for _, order := range q.orderBy {
		result = result.Order(fmt.Sprintf("%s %s", order.field, order.direction.String()))
	}

	if q.limit > 0 {
		result = result.Limit(q.limit)
	}

	if q.offset > 0 {
		result = result.Offset(q.offset)
	}

	return result
}

func applyFilter(db *gorm.DB, filter Filter) *gorm.DB {
	switch filter.operator {
	case OpEqual:
		return db.Where(fmt.Sprintf("%s = ?", filter.field), filter.value)
	case OpNotEqual:
		return db.Where(fmt.Sprintf("%s != ?", filter.field), filter.value)
	case OpGreaterThan:
		return db.Where(fmt.Sprintf("%s > ?", filter.field), filter.value)
	case OpGreaterThanOrEqual:
		return db.Where(fmt.Sprintf("%s >= ?", filter.field), filter.value)
	case OpLessThan:
		return db.Where(fmt.Sprintf("%s < ?", filter.field), filter.value)
	case OpLessThanOrEqual:
		return db.Where(fmt.Sprintf("%s <= ?", filter.field), filter.value)
	case OpLike:
		return db.Where(fmt.Sprintf("%s LIKE ?", filter.field), filter.value)
	case OpILike:
		return db.Where(fmt.Sprintf("%s ILIKE ?", filter.field), filter.value)
	case OpIn:
		return db.Where(fmt.Sprintf("%s IN ?", filter.field), filter.value)
	case OpNotIn:
		return db.Where(fmt.Sprintf("%s NOT IN ?", filter.field), filter.value)
	case OpIsNull:
		return db.Where(fmt.Sprintf("%s IS NULL", filter.field))
	case OpIsNotNull:
		return db.Where(fmt.Sprintf("%s IS NOT NULL", filter.field))
	case OpBetween:
		return db.Where(fmt.Sprintf("%s BETWEEN ? AND ?", filter.field), filter.value, filter.value2)
	default:
		return db.Where(fmt.Sprintf("%s = ?", filter.field), filter.value)
	}
}
