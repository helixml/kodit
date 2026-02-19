package repository

import "context"

// Collection is a read-only view of a Store, exposing only Find and Get.
type Collection[T any] struct {
	store Store[T]
}

// NewCollection wraps a Store in a read-only Collection.
func NewCollection[T any](store Store[T]) Collection[T] {
	return Collection[T]{store: store}
}

// Find returns all entities matching the given options.
func (c Collection[T]) Find(ctx context.Context, options ...Option) ([]T, error) {
	return c.store.Find(ctx, options...)
}

// Get returns a single entity matching the given options.
func (c Collection[T]) Get(ctx context.Context, options ...Option) (T, error) {
	return c.store.FindOne(ctx, options...)
}

// Count returns the total number of entities matching the given options.
func (c Collection[T]) Count(ctx context.Context, options ...Option) (int64, error) {
	return c.store.Count(ctx, options...)
}
