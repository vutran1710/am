package nango

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.nango.dev"

// Record is a single record returned by the Nango /records endpoint.
type Record struct {
	ID        string          `json:"id"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

// RecordsResponse is the response from GET /records.
type RecordsResponse struct {
	Records    []Record `json:"records"`
	NextCursor string   `json:"next_cursor"`
}

// Client talks to the Nango REST API.
type Client struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

// ClientOption configures the Nango client.
type ClientOption func(*Client)

// WithBaseURL overrides the default Nango API URL (useful for testing).
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient creates a Nango API client.
func NewClient(secretKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:   defaultBaseURL,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchRecordsInput are the parameters for fetching records.
type FetchRecordsInput struct {
	ProviderConfigKey string    // required: integration ID (e.g. "google-mail")
	ConnectionID      string    // required: connection ID (e.g. "user-1-gmail")
	Model             string    // required: data model (e.g. "emails")
	ModifiedAfter     time.Time // optional: only records modified after this time
	Cursor            string    // optional: pagination cursor
	Limit             int       // optional: max records per page (default 100)
}

// FetchRecords retrieves synced records from Nango.
func (c *Client) FetchRecords(ctx context.Context, input FetchRecordsInput) (*RecordsResponse, error) {
	params := url.Values{}
	params.Set("model", input.Model)
	if !input.ModifiedAfter.IsZero() {
		params.Set("modified_after", input.ModifiedAfter.Format(time.RFC3339))
	}
	if input.Cursor != "" {
		params.Set("cursor", input.Cursor)
	}
	if input.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", input.Limit))
	}

	reqURL := fmt.Sprintf("%s/records?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Provider-Config-Key", input.ProviderConfigKey)
	req.Header.Set("Connection-Id", input.ConnectionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nango API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result RecordsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ConnectSession is the response from POST /connect/sessions.
type ConnectSession struct {
	Token       string    `json:"token"`
	ConnectLink string    `json:"connect_link"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CreateConnectSessionInput configures a new connect session.
type CreateConnectSessionInput struct {
	EndUserID           string   `json:"end_user_id,omitempty"`
	AllowedIntegrations []string `json:"allowed_integrations,omitempty"`
}

// CreateConnectSession creates a Nango connect session and returns a shareable link.
func (c *Client) CreateConnectSession(ctx context.Context, input CreateConnectSessionInput) (*ConnectSession, error) {
	body := map[string]any{}
	if input.EndUserID != "" {
		body["tags"] = map[string]string{"end_user_id": input.EndUserID}
	}
	if len(input.AllowedIntegrations) > 0 {
		body["allowed_integrations"] = input.AllowedIntegrations
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/connect/sessions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nango API error (status %d): %s", resp.StatusCode, string(b))
	}

	var wrapper struct {
		Data ConnectSession `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &wrapper.Data, nil
}

// NangoConnection represents a connection from GET /connections.
type NangoConnection struct {
	ID                int               `json:"id"`
	ConnectionID      string            `json:"connection_id"`
	ProviderConfigKey string            `json:"provider_config_key"`
	Provider          string            `json:"provider"`
	CreatedAt         time.Time         `json:"created_at"`
	Tags              map[string]string `json:"tags"`
}

// ListConnections lists all connections, optionally filtered by search term.
func (c *Client) ListConnections(ctx context.Context, search string) ([]NangoConnection, error) {
	params := url.Values{}
	if search != "" {
		params.Set("search", search)
	}

	reqURL := c.baseURL + "/connections"
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nango API error (status %d): %s", resp.StatusCode, string(b))
	}

	var wrapper struct {
		Connections []NangoConnection `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return wrapper.Connections, nil
}
