package silo

import (
	"context"
	"time"
)

// Query filters messages from the store.
type Query struct {
	Source *Source    // filter by source, nil means all
	Since  *time.Time // messages after this time
	Until  *time.Time // messages before this time
	Search string     // full-text search term
	Limit  int        // max results, 0 means default (100)
	Offset int        // pagination offset
}

// Store persists and retrieves messages.
type Store interface {
	// Put saves one or more messages. Existing IDs are skipped (upsert).
	Put(ctx context.Context, msgs ...Message) error

	// Get retrieves a single message by ID.
	Get(ctx context.Context, id string) (*Message, error)

	// List queries messages matching the filter.
	List(ctx context.Context, q Query) ([]Message, error)

	// LoadCursor returns the last saved cursor for an adapter.
	LoadCursor(ctx context.Context, adapterName string) (Cursor, error)

	// SaveCursor persists the cursor for an adapter.
	SaveCursor(ctx context.Context, adapterName string, c Cursor) error

	// Close releases any resources held by the store.
	Close() error
}
