package enrichment

// Filter specifies criteria for querying enrichments.
// Immutable — builder methods return a new Filter.
type Filter struct {
	types    []Type
	subtypes []Subtype
	limit    int
	offset   int
}

// NewFilter creates an empty Filter with no constraints.
func NewFilter() Filter {
	return Filter{}
}

// WithType returns a new Filter that also matches the given type.
func (f Filter) WithType(t Type) Filter {
	types := make([]Type, len(f.types), len(f.types)+1)
	copy(types, f.types)
	return Filter{
		types:    append(types, t),
		subtypes: f.subtypes,
		limit:    f.limit,
		offset:   f.offset,
	}
}

// WithSubtype returns a new Filter that also matches the given subtype.
func (f Filter) WithSubtype(s Subtype) Filter {
	subtypes := make([]Subtype, len(f.subtypes), len(f.subtypes)+1)
	copy(subtypes, f.subtypes)
	return Filter{
		types:    f.types,
		subtypes: append(subtypes, s),
		limit:    f.limit,
		offset:   f.offset,
	}
}

// WithLimit returns a new Filter with the given result limit.
func (f Filter) WithLimit(n int) Filter {
	return Filter{
		types:    f.types,
		subtypes: f.subtypes,
		limit:    n,
		offset:   f.offset,
	}
}

// WithOffset returns a new Filter with the given result offset.
func (f Filter) WithOffset(n int) Filter {
	return Filter{
		types:    f.types,
		subtypes: f.subtypes,
		limit:    f.limit,
		offset:   n,
	}
}

// Types returns the type constraints, empty if unconstrained.
func (f Filter) Types() []Type {
	return f.types
}

// Subtypes returns the subtype constraints, empty if unconstrained.
func (f Filter) Subtypes() []Subtype {
	return f.subtypes
}

// Limit returns the result limit, zero means unlimited.
func (f Filter) Limit() int {
	return f.limit
}

// Offset returns the result offset.
func (f Filter) Offset() int {
	return f.offset
}

// FirstType returns a pointer to the first type constraint, or nil if none.
func (f Filter) FirstType() *Type {
	if len(f.types) == 0 {
		return nil
	}
	t := f.types[0]
	return &t
}

// FirstSubtype returns a pointer to the first subtype constraint, or nil if none.
func (f Filter) FirstSubtype() *Subtype {
	if len(f.subtypes) == 0 {
		return nil
	}
	s := f.subtypes[0]
	return &s
}
