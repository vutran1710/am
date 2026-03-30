package nango

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchRecords(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/records" {
			t.Errorf("path = %s, want /records", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Provider-Config-Key") != "google-mail" {
			t.Errorf("provider = %q", r.Header.Get("Provider-Config-Key"))
		}
		if r.Header.Get("Connection-Id") != "user-1" {
			t.Errorf("connection = %q", r.Header.Get("Connection-Id"))
		}
		if r.URL.Query().Get("model") != "emails" {
			t.Errorf("model = %q", r.URL.Query().Get("model"))
		}

		resp := RecordsResponse{
			Records: []Record{
				{
					ID:        "rec-1",
					Data:      json.RawMessage(`{"from":"alice@example.com","subject":"Hello"}`),
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:        "rec-2",
					Data:      json.RawMessage(`{"from":"bob@example.com","subject":"Meeting"}`),
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
			NextCursor: "cursor-abc",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-secret", WithBaseURL(server.URL))

	result, err := client.FetchRecords(context.Background(), FetchRecordsInput{
		ProviderConfigKey: "google-mail",
		ConnectionID:      "user-1",
		Model:             "emails",
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(result.Records))
	}
	if result.NextCursor != "cursor-abc" {
		t.Errorf("cursor = %q, want cursor-abc", result.NextCursor)
	}
	if result.Records[0].ID != "rec-1" {
		t.Errorf("id = %q, want rec-1", result.Records[0].ID)
	}
}

func TestFetchRecordsWithPagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		cursor := r.URL.Query().Get("cursor")

		var resp RecordsResponse
		if cursor == "" {
			resp = RecordsResponse{
				Records:    []Record{{ID: "r1", Data: json.RawMessage(`{}`)}},
				NextCursor: "page2",
			}
		} else {
			resp = RecordsResponse{
				Records:    []Record{{ID: "r2", Data: json.RawMessage(`{}`)}},
				NextCursor: "",
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))

	// First page
	r1, err := client.FetchRecords(context.Background(), FetchRecordsInput{
		ProviderConfigKey: "slack", ConnectionID: "ws-1", Model: "messages",
	})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if r1.NextCursor != "page2" {
		t.Errorf("cursor = %q, want page2", r1.NextCursor)
	}

	// Second page
	r2, err := client.FetchRecords(context.Background(), FetchRecordsInput{
		ProviderConfigKey: "slack", ConnectionID: "ws-1", Model: "messages",
		Cursor: r1.NextCursor,
	})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if r2.NextCursor != "" {
		t.Errorf("cursor = %q, want empty", r2.NextCursor)
	}
	if callCount != 2 {
		t.Errorf("calls = %d, want 2", callCount)
	}
}

func TestFetchRecordsModifiedAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ma := r.URL.Query().Get("modified_after")
		if ma == "" {
			t.Error("expected modified_after param")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecordsResponse{})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	_, err := client.FetchRecords(context.Background(), FetchRecordsInput{
		ProviderConfigKey: "google-mail",
		ConnectionID:      "user-1",
		Model:             "emails",
		ModifiedAfter:     time.Now().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
}

func TestFetchRecordsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	client := NewClient("bad-key", WithBaseURL(server.URL))
	_, err := client.FetchRecords(context.Background(), FetchRecordsInput{
		ProviderConfigKey: "google-mail", ConnectionID: "u1", Model: "emails",
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestCreateConnectSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/connect/sessions" {
			t.Errorf("path = %s, want /connect/sessions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// Check allowed_integrations was sent
		if ai, ok := body["allowed_integrations"].([]any); !ok || len(ai) != 1 {
			t.Errorf("allowed_integrations = %v", body["allowed_integrations"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"token":        "sess-token-123",
				"connect_link": "https://connect.nango.dev/xyz",
				"expires_at":   "2026-03-30T12:00:00Z",
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	sess, err := client.CreateConnectSession(context.Background(), CreateConnectSessionInput{
		EndUserID:           "user-1",
		AllowedIntegrations: []string{"google-mail"},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Token != "sess-token-123" {
		t.Errorf("token = %q", sess.Token)
	}
	if sess.ConnectLink != "https://connect.nango.dev/xyz" {
		t.Errorf("link = %q", sess.ConnectLink)
	}
}

func TestListConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/connections" {
			t.Errorf("path = %s, want /connections", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"connections": []map[string]any{
				{
					"id":                  1,
					"connection_id":       "conn-gmail-personal",
					"provider_config_key": "google-mail",
					"provider":            "google-mail",
					"created_at":          "2026-03-30T10:00:00Z",
					"tags":                map[string]string{"end_user_id": "user-1"},
				},
				{
					"id":                  2,
					"connection_id":       "conn-slack-work",
					"provider_config_key": "slack",
					"provider":            "slack",
					"created_at":          "2026-03-30T10:00:00Z",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	conns, err := client.ListConnections(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("connections = %d, want 2", len(conns))
	}
	if conns[0].ConnectionID != "conn-gmail-personal" {
		t.Errorf("conn[0].id = %q", conns[0].ConnectionID)
	}
	if conns[0].ProviderConfigKey != "google-mail" {
		t.Errorf("conn[0].provider = %q", conns[0].ProviderConfigKey)
	}
	if conns[1].ConnectionID != "conn-slack-work" {
		t.Errorf("conn[1].id = %q", conns[1].ConnectionID)
	}
}
