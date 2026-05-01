package database

import (
	"fmt"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// ApplyOptions builds a store.Query from the given options and applies it to a GORM session.
func ApplyOptions(db *gorm.DB, options ...repository.Option) *gorm.DB {
	q := repository.Build(options...)

	if sels := q.Selects(); len(sels) > 0 {
		exprs := make([]string, len(sels))
		var args []any
		for i, s := range sels {
			exprs[i] = s.Expr()
			args = append(args, s.Args()...)
		}
		db = db.Select(strings.Join(exprs, ", "), args...)
	}

	for _, j := range q.Joins() {
		db = db.Joins(j.Expr(), j.Args()...)
	}

	for _, cond := range q.Conditions() {
		if cond.In() {
			db = db.Where(fmt.Sprintf("%s IN ?", cond.Field()), cond.Value())
		} else {
			db = db.Where(fmt.Sprintf("%s = ?", cond.Field()), cond.Value())
		}
	}

	for _, clause := range q.Clauses() {
		db = db.Where(clause.SQL(), clause.Args()...)
	}

	for _, ord := range q.Orders() {
		dir := "ASC"
		if !ord.Ascending() {
			dir = "DESC"
		}
		db = db.Order(fmt.Sprintf("%s %s", ord.Field(), dir))
	}

	for _, raw := range q.RawOrders() {
		db = db.Order(raw)
	}

	if q.LimitValue() > 0 {
		db = db.Limit(q.LimitValue())
	}

	if q.OffsetValue() > 0 {
		db = db.Offset(q.OffsetValue())
	}

	return db
}

// ApplyConditions applies only WHERE conditions (no limit/offset/order) for COUNT queries.
func ApplyConditions(db *gorm.DB, options ...repository.Option) *gorm.DB {
	q := repository.Build(options...)

	for _, j := range q.Joins() {
		db = db.Joins(j.Expr(), j.Args()...)
	}

	for _, cond := range q.Conditions() {
		if cond.In() {
			db = db.Where(fmt.Sprintf("%s IN ?", cond.Field()), cond.Value())
		} else {
			db = db.Where(fmt.Sprintf("%s = ?", cond.Field()), cond.Value())
		}
	}

	for _, clause := range q.Clauses() {
		db = db.Where(clause.SQL(), clause.Args()...)
	}

	return db
}
