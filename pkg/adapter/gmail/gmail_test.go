package gmail

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// --- mock client ---

type mockGmailClient struct {
	messages map[string]*RawEmail // id -> email
	listIDs  []string            // IDs returned by ListMessages
}

func (m *mockGmailClient) ListMessages(_ context.Context, query string, pageToken string, maxResults int64) (*MessageList, error) {
	result := &MessageList{}
	for _, id := range m.listIDs {
		result.Messages = append(result.Messages, MessageRef{ID: id})
	}
	return result, nil
}

func (m *mockGmailClient) GetMessage(_ context.Context, id string) (*RawEmail, error) {
	if email, ok := m.messages[id]; ok {
		return email, nil
	}
	return nil, nil
}

func TestPollNoMessages(t *testing.T) {
	client := &mockGmailClient{
		messages: map[string]*RawEmail{},
		listIDs:  nil,
	}

	adapter := NewAdapterWithClient("test", client, slog.Default())

	msgs, cursor, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
	if cursor == nil {
		t.Fatal("expected cursor, got nil")
	}
}

func TestPollWithMessages(t *testing.T) {
	now := time.Now()
	client := &mockGmailClient{
		listIDs: []string{"msg-1", "msg-2"},
		messages: map[string]*RawEmail{
			"msg-1": {
				ID:      "msg-1",
				From:    "alice@example.com",
				Subject: "Hello",
				Snippet: "How are you?",
				Date:    now.Add(-10 * time.Minute),
				Raw:     json.RawMessage(`{"id":"msg-1"}`),
			},
			"msg-2": {
				ID:      "msg-2",
				From:    "bob@example.com",
				Subject: "Meeting",
				Snippet: "Let's meet at 3pm",
				Date:    now.Add(-5 * time.Minute),
				Raw:     json.RawMessage(`{"id":"msg-2"}`),
			},
		},
	}

	adapter := NewAdapterWithClient("personal", client, slog.Default())

	msgs, cursorBytes, err := adapter.Poll(context.Background(), nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// Verify message fields
	if msgs[0].Source != silo.SourceGmail {
		t.Errorf("source = %q, want gmail", msgs[0].Source)
	}
	if msgs[0].Sender != "alice@example.com" {
		t.Errorf("sender = %q, want alice@example.com", msgs[0].Sender)
	}
	if msgs[0].ID != "gmail:personal:msg-1" {
		t.Errorf("id = %q, want gmail:personal:msg-1", msgs[0].ID)
	}

	// Verify cursor advanced
	var cur cursor
	if err := json.Unmarshal(cursorBytes, &cur); err != nil {
		t.Fatalf("unmarshal cursor: %v", err)
	}
	if cur.AfterEpoch < now.Add(-6*time.Minute).Unix() {
		t.Errorf("cursor not advanced, epoch = %d", cur.AfterEpoch)
	}
}

func TestPollWithExistingCursor(t *testing.T) {
	client := &mockGmailClient{
		listIDs:  nil,
		messages: map[string]*RawEmail{},
	}

	adapter := NewAdapterWithClient("work", client, slog.Default())

	// Provide an existing cursor
	existingCursor, _ := json.Marshal(cursor{AfterEpoch: 1700000000})

	msgs, _, err := adapter.Poll(context.Background(), silo.Cursor(existingCursor))
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestAdapterIdentity(t *testing.T) {
	adapter := NewAdapterWithClient("myaccount", nil, slog.Default())

	if adapter.Name() != "gmail:myaccount" {
		t.Errorf("name = %q, want gmail:myaccount", adapter.Name())
	}
	if adapter.Source() != silo.SourceGmail {
		t.Errorf("source = %q, want gmail", adapter.Source())
	}
	if adapter.Mode() != silo.ModePoll {
		t.Errorf("mode = %v, want ModePoll", adapter.Mode())
	}
}
