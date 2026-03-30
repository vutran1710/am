package composio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/tools/execute/GMAIL_FETCH_EMAILS" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("api key = %q", r.Header.Get("x-api-key"))
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["connected_account_id"] != "conn-123" {
			t.Errorf("connected_account_id = %v", body["connected_account_id"])
		}
		if body["entity_id"] != "user-1" {
			t.Errorf("entity_id = %v", body["entity_id"])
		}
		if body["arguments"] == nil {
			t.Error("expected arguments")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "msg-1", "from": "alice@test.com", "subject": "Hello"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	result, err := client.ExecuteTool(context.Background(), "GMAIL_FETCH_EMAILS", "conn-123", "user-1", map[string]any{
		"max_results": 10,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected data")
	}
}

func TestCreateConnectLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/connected_accounts/link" {
			t.Errorf("path = %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"redirect_url":        "https://connect.composio.dev/link/lk_abc",
			"link_token":          "lk_abc",
			"connected_account_id": "ca_123",
		})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	link, err := client.CreateConnectLink(context.Background(), CreateConnectLinkInput{
		AuthConfigID: "ac_test",
		UserID:       "user-1",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if link.RedirectURL == "" {
		t.Error("expected redirect_url")
	}
	if link.ConnectedAccountID != "ca_123" {
		t.Errorf("connected_account_id = %q", link.ConnectedAccountID)
	}
}

func TestListConnectedAccounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": "acc-1", "appName": "gmail", "status": "ACTIVE", "entityId": "user-1"},
				{"id": "acc-2", "appName": "slack", "status": "ACTIVE", "entityId": "user-1"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	accs, err := client.ListConnectedAccounts(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accs) != 2 {
		t.Fatalf("accounts = %d, want 2", len(accs))
	}
	if accs[0].AppName != "gmail" {
		t.Errorf("app = %q", accs[0].AppName)
	}
}

func TestGetConnectedAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connected_accounts/ca-123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": "ca-123", "status": "ACTIVE", "appName": "gmail",
		})
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	acc, err := client.GetConnectedAccount(context.Background(), "ca-123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if acc.ID != "ca-123" {
		t.Errorf("id = %q", acc.ID)
	}
	if acc.Status != "ACTIVE" {
		t.Errorf("status = %q", acc.Status)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	client := NewClient("bad", WithBaseURL(server.URL))
	_, err := client.ExecuteTool(context.Background(), "GMAIL_FETCH_EMAILS", "c", "e", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
