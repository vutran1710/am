package silo

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// --- mock clock ---

type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time                  { return c.now }
func (c *mockClock) NewTicker(d time.Duration) Ticker { return newMockTicker() }

type mockTicker struct {
	ch   chan time.Time
	done chan struct{}
}

func newMockTicker() *mockTicker {
	return &mockTicker{
		ch:   make(chan time.Time, 10),
		done: make(chan struct{}),
	}
}

func (t *mockTicker) C() <-chan time.Time { return t.ch }
func (t *mockTicker) Stop()              { close(t.done) }
func (t *mockTicker) Tick()              { t.ch <- time.Now() }

// --- mock store ---

type mockStore struct {
	mu       sync.Mutex
	messages []Message
	cursors  map[string]Cursor
}

func newMockStore() *mockStore {
	return &mockStore{cursors: make(map[string]Cursor)}
}

func (s *mockStore) Put(_ context.Context, msgs ...Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msgs...)
	return nil
}

func (s *mockStore) Get(_ context.Context, id string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.messages {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, nil
}

func (s *mockStore) List(_ context.Context, _ Query) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.messages, nil
}

func (s *mockStore) LoadCursor(_ context.Context, name string) (Cursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursors[name], nil
}

func (s *mockStore) SaveCursor(_ context.Context, name string, c Cursor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursors[name] = c
	return nil
}

func (s *mockStore) Close() error { return nil }

func (s *mockStore) messageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

// --- mock poller ---

type mockPoller struct {
	name     string
	source   Source
	pollFunc func(ctx context.Context, since Cursor) ([]Message, Cursor, error)
}

func (p *mockPoller) Name() string      { return p.name }
func (p *mockPoller) Source() Source     { return p.source }
func (p *mockPoller) Mode() AdapterMode { return ModePoll }
func (p *mockPoller) Poll(ctx context.Context, since Cursor) ([]Message, Cursor, error) {
	return p.pollFunc(ctx, since)
}

// --- mock watcher ---

type mockWatcher struct {
	name      string
	source    Source
	watchFunc func(ctx context.Context, sink chan<- Message) error
}

func (w *mockWatcher) Name() string      { return w.name }
func (w *mockWatcher) Source() Source     { return w.source }
func (w *mockWatcher) Mode() AdapterMode { return ModeWatch }
func (w *mockWatcher) Watch(ctx context.Context, sink chan<- Message) error {
	return w.watchFunc(ctx, sink)
}

// --- tests ---

func TestSchedulerPoller(t *testing.T) {
	store := newMockStore()
	clock := &mockClock{now: time.Now()}
	logger := slog.Default()

	sched := NewScheduler(store, clock, logger)

	callCount := 0
	poller := &mockPoller{
		name:   "test-poller",
		source: SourceSlack,
		pollFunc: func(ctx context.Context, since Cursor) ([]Message, Cursor, error) {
			callCount++
			return []Message{
				{
					ID:         "poll-" + time.Now().String(),
					Source:     SourceSlack,
					Raw:        json.RawMessage(`{}`),
					CapturedAt: time.Now(),
				},
			}, Cursor("next"), nil
		},
	}

	sched.Register(poller, AdapterConfig{Interval: time.Hour}) // long interval, we'll cancel quickly

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	// Should have polled at least once (the immediate poll)
	if store.messageCount() < 1 {
		t.Errorf("expected at least 1 message, got %d", store.messageCount())
	}
}

func TestSchedulerWatcher(t *testing.T) {
	store := newMockStore()
	clock := &mockClock{now: time.Now()}
	logger := slog.Default()

	sched := NewScheduler(store, clock, logger)

	watcher := &mockWatcher{
		name:   "test-watcher",
		source: SourceDiscord,
		watchFunc: func(ctx context.Context, sink chan<- Message) error {
			sink <- Message{
				ID:         "watch-1",
				Source:     SourceDiscord,
				Raw:        json.RawMessage(`{"content":"hello"}`),
				CapturedAt: time.Now(),
			}
			<-ctx.Done()
			return nil
		},
	}

	sched.Register(watcher, AdapterConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	sched.Run(ctx)

	if store.messageCount() < 1 {
		t.Errorf("expected at least 1 message from watcher, got %d", store.messageCount())
	}
}

func TestSchedulerCursorPersistence(t *testing.T) {
	store := newMockStore()
	clock := &mockClock{now: time.Now()}
	logger := slog.Default()

	sched := NewScheduler(store, clock, logger)

	poller := &mockPoller{
		name:   "cursor-test",
		source: SourceGmail,
		pollFunc: func(ctx context.Context, since Cursor) ([]Message, Cursor, error) {
			return nil, Cursor("page-token-123"), nil
		},
	}

	sched.Register(poller, AdapterConfig{Interval: time.Hour})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sched.Run(ctx)

	c, err := store.LoadCursor(context.Background(), "cursor-test")
	if err != nil {
		t.Fatalf("load cursor: %v", err)
	}
	if string(c) != "page-token-123" {
		t.Errorf("cursor = %q, want %q", c, "page-token-123")
	}
}
