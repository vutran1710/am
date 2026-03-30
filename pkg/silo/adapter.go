package silo

import "context"

// AdapterMode describes how an adapter delivers messages.
type AdapterMode int

const (
	ModePoll  AdapterMode = iota // adapter is polled on a schedule
	ModeWatch                    // adapter pushes messages continuously
)

// Adapter is the base interface all message source adapters implement.
type Adapter interface {
	// Name returns the adapter identifier (e.g. "gmail", "slack").
	Name() string

	// Source returns the Source constant for messages from this adapter.
	Source() Source

	// Mode returns whether this adapter polls or watches.
	Mode() AdapterMode
}

// Poller fetches new messages since the last known cursor.
// Returns the fetched messages and an updated cursor for the next poll.
type Poller interface {
	Adapter
	Poll(ctx context.Context, since Cursor) ([]Message, Cursor, error)
}

// Watcher pushes messages to a channel continuously.
// Watch blocks until ctx is cancelled or a fatal error occurs.
type Watcher interface {
	Adapter
	Watch(ctx context.Context, sink chan<- Message) error
}
