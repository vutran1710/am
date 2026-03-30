package sqlite

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPutAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	msg := silo.Message{
		ID:         "msg-1",
		Source:     silo.SourceSlack,
		Sender:     "alice",
		Subject:    "hello",
		Preview:    "hello world",
		Raw:        json.RawMessage(`{"text":"hello world","channel":"general"}`),
		CapturedAt: time.Now().Truncate(time.Second),
		SourceTS:   time.Now().Add(-time.Minute).Truncate(time.Second),
	}

	if err := s.Put(ctx, msg); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := s.Get(ctx, "msg-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected message, got nil")
	}
	if got.Source != silo.SourceSlack {
		t.Errorf("source = %q, want %q", got.Source, silo.SourceSlack)
	}
	if got.Sender != "alice" {
		t.Errorf("sender = %q, want %q", got.Sender, "alice")
	}
}

func TestPutDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	msg := silo.Message{
		ID:         "dup-1",
		Source:     silo.SourceGmail,
		Sender:     "bob",
		Raw:        json.RawMessage(`{}`),
		CapturedAt: time.Now(),
	}

	if err := s.Put(ctx, msg, msg); err != nil {
		t.Fatalf("put duplicates: %v", err)
	}

	msgs, err := s.List(ctx, silo.Query{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1", len(msgs))
	}
}

func TestListBySource(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	msgs := []silo.Message{
		{ID: "s1", Source: silo.SourceSlack, Raw: json.RawMessage(`{}`), CapturedAt: now, SourceTS: now},
		{ID: "s2", Source: silo.SourceSlack, Raw: json.RawMessage(`{}`), CapturedAt: now, SourceTS: now.Add(time.Second)},
		{ID: "d1", Source: silo.SourceDiscord, Raw: json.RawMessage(`{}`), CapturedAt: now, SourceTS: now},
	}
	if err := s.Put(ctx, msgs...); err != nil {
		t.Fatalf("put: %v", err)
	}

	src := silo.SourceSlack
	got, err := s.List(ctx, silo.Query{Source: &src})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d messages, want 2", len(got))
	}
}

func TestFTSSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	msgs := []silo.Message{
		{ID: "f1", Source: silo.SourceGmail, Sender: "alice", Subject: "project update", Preview: "the deployment is ready", Raw: json.RawMessage(`{}`), CapturedAt: now, SourceTS: now},
		{ID: "f2", Source: silo.SourceSlack, Sender: "bob", Subject: "", Preview: "lunch at noon?", Raw: json.RawMessage(`{}`), CapturedAt: now, SourceTS: now},
	}
	if err := s.Put(ctx, msgs...); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := s.List(ctx, silo.Query{Search: "deployment"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d results, want 1", len(got))
	}
	if len(got) > 0 && got[0].ID != "f1" {
		t.Errorf("got id %q, want f1", got[0].ID)
	}
}

func TestCursors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c, err := s.LoadCursor(ctx, "slack")
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil cursor, got %v", c)
	}

	cursor := silo.Cursor(`{"ts":"1234567890.123456"}`)
	if err := s.SaveCursor(ctx, "slack", cursor); err != nil {
		t.Fatalf("save: %v", err)
	}

	c, err = s.LoadCursor(ctx, "slack")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if string(c) != string(cursor) {
		t.Errorf("cursor = %q, want %q", c, cursor)
	}

	// update existing
	cursor2 := silo.Cursor(`{"ts":"9999999999.000000"}`)
	if err := s.SaveCursor(ctx, "slack", cursor2); err != nil {
		t.Fatalf("save update: %v", err)
	}
	c, err = s.LoadCursor(ctx, "slack")
	if err != nil {
		t.Fatalf("load after update: %v", err)
	}
	if string(c) != string(cursor2) {
		t.Errorf("cursor = %q, want %q", c, cursor2)
	}
}
