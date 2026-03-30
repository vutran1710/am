package nango

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

func TestAdapterPollNoRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecordsResponse{})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	adapter := NewAdapter(client, GmailConfig, "conn-1", "personal", slog.Default())

	msgs, cursor, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("msgs = %d, want 0", len(msgs))
	}
	if cursor == nil {
		t.Fatal("expected cursor")
	}
}

func TestAdapterPollWithRecords(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Provider-Config-Key") != "google-mail" {
			t.Errorf("provider = %q", r.Header.Get("Provider-Config-Key"))
		}
		if r.Header.Get("Connection-Id") != "conn-personal" {
			t.Errorf("connection = %q", r.Header.Get("Connection-Id"))
		}

		resp := RecordsResponse{
			Records: []Record{
				{
					ID:        "email-1",
					Data:      json.RawMessage(`{"from":"alice@test.com","subject":"Hello","snippet":"How are you doing today?"}`),
					CreatedAt: now.Add(-10 * time.Minute),
					UpdatedAt: now.Add(-10 * time.Minute),
				},
				{
					ID:        "email-2",
					Data:      json.RawMessage(`{"from":"bob@test.com","subject":"Lunch","snippet":"Want to grab lunch?"}`),
					CreatedAt: now.Add(-5 * time.Minute),
					UpdatedAt: now.Add(-5 * time.Minute),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	adapter := NewAdapter(client, GmailConfig, "conn-personal", "personal", slog.Default())

	msgs, cursorBytes, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("msgs = %d, want 2", len(msgs))
	}

	// Check first message mapping
	m := msgs[0]
	if m.Source != silo.SourceGmail {
		t.Errorf("source = %q", m.Source)
	}
	if m.Sender != "alice@test.com" {
		t.Errorf("sender = %q", m.Sender)
	}
	if m.Subject != "Hello" {
		t.Errorf("subject = %q", m.Subject)
	}
	if m.Preview != "How are you doing today?" {
		t.Errorf("preview = %q", m.Preview)
	}
	if m.ID != "gmail:conn-personal:email-1" {
		t.Errorf("id = %q", m.ID)
	}

	// Cursor should advance
	var cur cursor
	json.Unmarshal(cursorBytes, &cur)
	if cur.ModifiedAfter.Before(now.Add(-6 * time.Minute)) {
		t.Errorf("cursor not advanced: %v", cur.ModifiedAfter)
	}
}

func TestAdapterPollSlack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Provider-Config-Key") != "slack" {
			t.Errorf("provider = %q", r.Header.Get("Provider-Config-Key"))
		}
		resp := RecordsResponse{
			Records: []Record{
				{
					ID:        "msg-1",
					Data:      json.RawMessage(`{"user":"alice","channel":"general","text":"hello everyone"}`),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	adapter := NewAdapter(client, SlackConfig, "ws-1", "myworkspace", slog.Default())

	msgs, _, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs = %d, want 1", len(msgs))
	}
	if msgs[0].Source != silo.SourceSlack {
		t.Errorf("source = %q", msgs[0].Source)
	}
	if msgs[0].Sender != "alice" {
		t.Errorf("sender = %q", msgs[0].Sender)
	}
	if msgs[0].Preview != "hello everyone" {
		t.Errorf("preview = %q", msgs[0].Preview)
	}
}

func TestAdapterPollPagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var resp RecordsResponse
		if r.URL.Query().Get("cursor") == "" {
			resp = RecordsResponse{
				Records:    []Record{{ID: "r1", Data: json.RawMessage(`{"from":"a"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}},
				NextCursor: "page2",
			}
		} else {
			resp = RecordsResponse{
				Records: []Record{{ID: "r2", Data: json.RawMessage(`{"from":"b"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	adapter := NewAdapter(client, GmailConfig, "c1", "test", slog.Default())

	msgs, _, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("msgs = %d, want 2 (across 2 pages)", len(msgs))
	}
	if callCount != 2 {
		t.Errorf("api calls = %d, want 2", callCount)
	}
}

func TestAdapterSkipsDeleted(t *testing.T) {
	deletedAt := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := RecordsResponse{
			Records: []Record{
				{ID: "live", Data: json.RawMessage(`{"from":"a"}`), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "dead", Data: json.RawMessage(`{"from":"b"}`), CreatedAt: time.Now(), UpdatedAt: time.Now(), DeletedAt: &deletedAt},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	adapter := NewAdapter(client, GmailConfig, "c1", "test", slog.Default())

	msgs, _, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs = %d, want 1 (deleted should be skipped)", len(msgs))
	}
}

func TestAdapterIdentity(t *testing.T) {
	adapter := NewAdapter(NewClient("k"), GmailConfig, "c1", "personal", slog.Default())

	if adapter.Name() != "nango:google-mail:personal" {
		t.Errorf("name = %q", adapter.Name())
	}
	if adapter.Source() != silo.SourceGmail {
		t.Errorf("source = %q", adapter.Source())
	}
	if adapter.Mode() != silo.ModePoll {
		t.Errorf("mode = %v", adapter.Mode())
	}
}
