package database

import (
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// ApplyOptions builds a store.Query from the given options and applies it to a GORM session.
func ApplyOptions(db *gorm.DB, options ...repository.Option) *gorm.DB {
	q := repository.Build(options...)

	for _, cond := range q.Conditions() {
		if cond.In() {
			db = db.Where(fmt.Sprintf("%s IN ?", cond.Field()), cond.Value())
		} else {
			db = db.Where(fmt.Sprintf("%s = ?", cond.Field()), cond.Value())
		}
	}

	for _, ord := range q.Orders() {
		dir := "ASC"
		if !ord.Ascending() {
			dir = "DESC"
		}
		db = db.Order(fmt.Sprintf("%s %s", ord.Field(), dir))
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

	for _, cond := range q.Conditions() {
		if cond.In() {
			db = db.Where(fmt.Sprintf("%s IN ?", cond.Field()), cond.Value())
		} else {
			db = db.Where(fmt.Sprintf("%s = ?", cond.Field()), cond.Value())
		}
	}

	return db
}
