package silo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AdapterConfig holds per-adapter scheduling settings.
type AdapterConfig struct {
	Interval   time.Duration // polling interval (ignored for watchers)
	BackoffMax time.Duration // max backoff on consecutive errors
}

// Scheduler runs adapters and feeds messages into a store.
type Scheduler struct {
	store  Store
	clock  Clock
	logger *slog.Logger

	mu       sync.Mutex
	adapters []adapterEntry
}

type adapterEntry struct {
	adapter Adapter
	config  AdapterConfig
}

// NewScheduler creates a scheduler that persists messages to store.
func NewScheduler(store Store, clock Clock, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:  store,
		clock:  clock,
		logger: logger,
	}
}

// Register adds an adapter with its scheduling config.
func (s *Scheduler) Register(a Adapter, cfg AdapterConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adapters = append(s.adapters, adapterEntry{adapter: a, config: cfg})
}

// Run starts all registered adapters and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	entries := make([]adapterEntry, len(s.adapters))
	copy(entries, s.adapters)
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func(e adapterEntry) {
			defer wg.Done()
			s.runAdapter(ctx, e)
		}(e)
	}

	wg.Wait()
	return nil
}

func (s *Scheduler) runAdapter(ctx context.Context, e adapterEntry) {
	name := e.adapter.Name()
	log := s.logger.With("adapter", name)

	switch e.adapter.Mode() {
	case ModePoll:
		p, ok := e.adapter.(Poller)
		if !ok {
			log.Error("adapter registered as poller but does not implement Poller")
			return
		}
		s.runPoller(ctx, p, e.config, log)

	case ModeWatch:
		w, ok := e.adapter.(Watcher)
		if !ok {
			log.Error("adapter registered as watcher but does not implement Watcher")
			return
		}
		s.runWatcher(ctx, w, log)
	}
}

func (s *Scheduler) runPoller(ctx context.Context, p Poller, cfg AdapterConfig, log *slog.Logger) {
	name := p.Name()
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	backoffMax := cfg.BackoffMax
	if backoffMax <= 0 {
		backoffMax = 5 * time.Minute
	}

	cursor, err := s.store.LoadCursor(ctx, name)
	if err != nil {
		log.Error("failed to load cursor", "err", err)
	}

	consecutiveErrors := 0
	ticker := s.clock.NewTicker(interval)
	defer ticker.Stop()

	// poll once immediately, then on ticker
	s.pollOnce(ctx, p, name, &cursor, &consecutiveErrors, log)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			if consecutiveErrors > 0 {
				backoff := interval * time.Duration(1<<min(consecutiveErrors, 10))
				if backoff > backoffMax {
					backoff = backoffMax
				}
				log.Debug("backing off", "duration", backoff)
			}
			s.pollOnce(ctx, p, name, &cursor, &consecutiveErrors, log)
		}
	}
}

func (s *Scheduler) pollOnce(ctx context.Context, p Poller, name string, cursor *Cursor, errCount *int, log *slog.Logger) {
	msgs, newCursor, err := p.Poll(ctx, *cursor)
	if err != nil {
		*errCount++
		log.Error("poll failed", "err", err, "consecutive_errors", *errCount)
		return
	}
	*errCount = 0

	if len(msgs) > 0 {
		if err := s.store.Put(ctx, msgs...); err != nil {
			log.Error("failed to store messages", "err", err, "count", len(msgs))
			return
		}
		log.Info("stored messages", "count", len(msgs))
	}

	if newCursor != nil {
		*cursor = newCursor
		if err := s.store.SaveCursor(ctx, name, newCursor); err != nil {
			log.Error("failed to save cursor", "err", err)
		}
	}
}

func (s *Scheduler) runWatcher(ctx context.Context, w Watcher, log *slog.Logger) {
	sink := make(chan Message, 64)

	go func() {
		if err := w.Watch(ctx, sink); err != nil && ctx.Err() == nil {
			log.Error("watcher exited with error", "err", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sink:
			if err := s.store.Put(ctx, msg); err != nil {
				log.Error("failed to store watched message", "err", err, "id", msg.ID)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Silo is the top-level orchestrator wiring a scheduler with a store.
type Silo struct {
	Store     Store
	Scheduler *Scheduler
}

// NewSilo creates a Silo with the given store, clock, and logger.
func NewSilo(store Store, clock Clock, logger *slog.Logger) *Silo {
	return &Silo{
		Store:     store,
		Scheduler: NewScheduler(store, clock, logger),
	}
}

// Register adds an adapter to the silo.
func (s *Silo) Register(a Adapter, cfg AdapterConfig) {
	s.Scheduler.Register(a, cfg)
}

// Run starts the scheduler and blocks until ctx is cancelled.
func (s *Silo) Run(ctx context.Context) error {
	return s.Scheduler.Run(ctx)
}

// Close shuts down the store.
func (s *Silo) Close() error {
	return s.Store.Close()
}

func init() {
	// Ensure min doesn't collide with builtin in older Go.
	_ = fmt.Sprintf
}
