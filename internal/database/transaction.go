package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Transaction wraps a GORM transaction with commit/rollback semantics.
type Transaction struct {
	tx       *gorm.DB
	finished bool
}

// NewTransaction starts a new database transaction.
func NewTransaction(ctx context.Context, db Database) (Transaction, error) {
	tx := db.Session(ctx).Begin()
	if tx.Error != nil {
		return Transaction{}, fmt.Errorf("begin transaction: %w", tx.Error)
	}
	return Transaction{tx: tx}, nil
}

// Session returns the transaction session for executing queries.
func (t Transaction) Session() *gorm.DB {
	return t.tx
}

// Commit commits the transaction.
func (t *Transaction) Commit() error {
	if t.finished {
		return nil
	}
	if err := t.tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	t.finished = true
	return nil
}

// Rollback rolls back the transaction if not already finished.
func (t *Transaction) Rollback() error {
	if t.finished {
		return nil
	}
	if err := t.tx.Rollback().Error; err != nil {
		return fmt.Errorf("rollback transaction: %w", err)
	}
	t.finished = true
	return nil
}

// WithTransaction executes fn within a transaction, committing on success or rolling back on error.
func WithTransaction(ctx context.Context, db Database, fn func(tx *gorm.DB) error) error {
	txn, err := NewTransaction(ctx, db)
	if err != nil {
		return err
	}

	defer func() {
		if !txn.finished {
			_ = txn.Rollback()
		}
	}()

	if err := fn(txn.Session()); err != nil {
		return err
	}

	return txn.Commit()
}

// WithTransactionResult executes fn within a transaction, returning the result on success.
func WithTransactionResult[T any](ctx context.Context, db Database, fn func(tx *gorm.DB) (T, error)) (T, error) {
	var result T

	txn, err := NewTransaction(ctx, db)
	if err != nil {
		return result, err
	}

	defer func() {
		if !txn.finished {
			_ = txn.Rollback()
		}
	}()

	result, err = fn(txn.Session())
	if err != nil {
		return result, err
	}

	if err := txn.Commit(); err != nil {
		return result, err
	}

	return result, nil
}
