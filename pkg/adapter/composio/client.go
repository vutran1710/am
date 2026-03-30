package composio

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

const defaultBaseURL = "https://backend.composio.dev/api/v3"

// Client talks to the Composio REST API.
type Client struct {
	baseURL    string // REST API base (management endpoints)
	mcpURL     string // MCP tool router URL (tool execution, optional)
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures the client.
type ClientOption func(*Client)

func WithBaseURL(u string) ClientOption    { return func(c *Client) { c.baseURL = u } }
func WithMCPURL(u string) ClientOption     { return func(c *Client) { c.mcpURL = u } }
func WithHTTPClient(hc *http.Client) ClientOption { return func(c *Client) { c.httpClient = hc } }

func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// toolBaseURL returns the URL to use for tool execution.
// Uses MCP URL if configured, otherwise falls back to REST API.
func (c *Client) toolBaseURL() string {
	if c.mcpURL != "" {
		return c.mcpURL
	}
	return c.baseURL
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// --- Connected Accounts ---

// ConnectLink is the response from POST /connected_accounts/link.
// ConnectLink is the response from POST /connected_accounts/link.
type ConnectLink struct {
	LinkToken          string `json:"link_token"`
	RedirectURL        string `json:"redirect_url"`
	ExpiresAt          string `json:"expires_at"`
	ConnectedAccountID string `json:"connected_account_id"`
}

// CreateConnectLinkInput configures a new auth link.
type CreateConnectLinkInput struct {
	AuthConfigID string `json:"auth_config_id"`
	UserID       string `json:"user_id"`
}

// CreateConnectLink creates an OAuth link for the user.
func (c *Client) CreateConnectLink(ctx context.Context, input CreateConnectLinkInput) (*ConnectLink, error) {
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/connected_accounts/link", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("composio API error (status %d): %s", resp.StatusCode, string(b))
	}

	var link ConnectLink
	if err := json.NewDecoder(resp.Body).Decode(&link); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &link, nil
}

// ConnectedAccount represents a connected account.
type ConnectedAccount struct {
	ID        string       `json:"id"`
	AppName   string       `json:"appName"`
	Status    string       `json:"status"`
	EntityID  string       `json:"entityId"`
	CreatedAt string       `json:"createdAt"`
	Params    AccountParams `json:"params"`
}

// AccountParams holds the credentials from a connected account.
type AccountParams struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// AccessToken returns the OAuth access token.
func (a *ConnectedAccount) AccessToken() string {
	return a.Params.AccessToken
}

// ListConnectedAccounts lists connected accounts, optionally filtered.
func (c *Client) ListConnectedAccounts(ctx context.Context, entityID string) ([]ConnectedAccount, error) {
	params := url.Values{}
	if entityID != "" {
		params.Set("entity_id", entityID)
	}

	reqURL := c.baseURL + "/connected_accounts"
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("composio API error (status %d): %s", resp.StatusCode, string(b))
	}

	var wrapper struct {
		Items []ConnectedAccount `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return wrapper.Items, nil
}

// GetConnectedAccount fetches a single connected account by ID.
func (c *Client) GetConnectedAccount(ctx context.Context, id string) (*ConnectedAccount, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/connected_accounts/"+id, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("composio API error (status %d): %s", resp.StatusCode, string(b))
	}

	var acc ConnectedAccount
	if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &acc, nil
}

// GetAccessToken retrieves the OAuth token for a connected account.
func (c *Client) GetAccessToken(ctx context.Context, connectionID string) (string, error) {
	acc, err := c.GetConnectedAccount(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if acc.Params.AccessToken == "" {
		return "", fmt.Errorf("no access token for connection %s", connectionID)
	}
	return acc.Params.AccessToken, nil
}

// WaitForActive polls a specific connected account until it becomes ACTIVE.
func (c *Client) WaitForActive(ctx context.Context, connectionID string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timed out waiting for connection %s to become active", connectionID)
		case <-ticker.C:
			acc, err := c.GetConnectedAccount(ctx, connectionID)
			if err != nil {
				continue
			}
			if acc.Status == "ACTIVE" {
				return acc.ID, nil
			}
		}
	}
}

// --- Auth Configs ---

// AuthConfig represents a Composio auth configuration for a toolkit.
type AuthConfig struct {
	ID          string `json:"id"`
	ToolkitSlug string `json:"toolkit_slug,omitempty"`
	Toolkit     struct {
		Slug string `json:"slug"`
	} `json:"toolkit"`
}

// GetOrCreateAuthConfigID looks up or creates an auth config for a toolkit.
func (c *Client) GetOrCreateAuthConfigID(ctx context.Context, toolkitSlug string) (string, error) {
	// Try to find existing
	params := url.Values{"toolkit_slug": {toolkitSlug}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/auth_configs?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var wrapper struct {
			Items []AuthConfig `json:"items"`
		}
		if json.NewDecoder(resp.Body).Decode(&wrapper) == nil && len(wrapper.Items) > 0 {
			return wrapper.Items[0].ID, nil
		}
	} else {
		io.ReadAll(resp.Body) // drain
	}

	// Not found — create one with Composio-managed credentials
	return c.createAuthConfig(ctx, toolkitSlug)
}

func (c *Client) createAuthConfig(ctx context.Context, toolkitSlug string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"toolkit":                        map[string]string{"slug": toolkitSlug},
		"auth_scheme":                    "OAUTH2",
		"use_composio_managed_credentials": true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/auth_configs", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create auth config failed (status %d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		AuthConfig struct {
			ID string `json:"id"`
		} `json:"auth_config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	return result.AuthConfig.ID, nil
}

// --- Tool Execution ---

// ToolResult is the response from executing a tool.
type ToolResult struct {
	Data json.RawMessage `json:"data"`
}

// ExecuteTool runs a Composio tool.
func (c *Client) ExecuteTool(ctx context.Context, toolSlug string, connectedAccountID string, entityID string, args map[string]any) (*ToolResult, error) {
	body, _ := json.Marshal(map[string]any{
		"connected_account_id": connectedAccountID,
		"entity_id":            entityID,
		"arguments":            args,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/tools/execute/"+toolSlug, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("composio API error (status %d): %s", resp.StatusCode, string(b))
	}

	var result ToolResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &result, nil
}
