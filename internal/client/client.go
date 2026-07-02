// Package client is a minimal Go client for the MotherDuck REST API
// (https://api.motherduck.com, spec at https://api.motherduck.com/docs/specs).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://api.motherduck.com"

// Client talks to the MotherDuck REST API using an admin read/write token.
type Client struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

func New(baseURL, token, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		UserAgent:  userAgent,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError is the error envelope every endpoint returns: {message, code, issues}.
type APIError struct {
	StatusCode int
	Message    string `json:"message"`
	Code       string `json:"code"`
	Issues     []struct {
		Message string `json:"message"`
	} `json:"issues"`
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("motherduck api: %s (HTTP %d, code %s)", e.Message, e.StatusCode, e.Code)
	for _, i := range e.Issues {
		msg += "; " + i.Message
	}
	return msg
}

// IsNotFound reports whether err is an APIError with HTTP 404.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if jsonErr := json.Unmarshal(respBody, apiErr); jsonErr != nil || apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
			if len(respBody) > 0 {
				apiErr.Message = string(respBody)
			}
		}
		return apiErr
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}
	}
	return nil
}

// --- Users (service accounts) ---

type User struct {
	Username string `json:"username"`
}

// CreateServiceAccount creates a new service-account user (Member role).
// POST /v1/users
func (c *Client) CreateServiceAccount(ctx context.Context, username string) (*User, error) {
	var out User
	err := c.do(ctx, http.MethodPost, "/v1/users", map[string]string{"username": username}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteUser permanently deletes a user and all of their data. THIS CANNOT BE UNDONE.
// DELETE /v1/users/{username}
func (c *Client) DeleteUser(ctx context.Context, username string) error {
	return c.do(ctx, http.MethodDelete, "/v1/users/"+username, nil, nil)
}

// --- Access tokens ---

type TokenType string

const (
	TokenTypeReadWrite   TokenType = "read_write"
	TokenTypeReadScaling TokenType = "read_scaling"
)

type CreateTokenRequest struct {
	Name      string    `json:"name"`
	TTL       *int64    `json:"ttl,omitempty"` // seconds, 300..31536000
	TokenType TokenType `json:"token_type,omitempty"`
}

type Token struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ExpireAt  string    `json:"expire_at"`
	CreatedTS string    `json:"created_ts"`
	ReadOnly  bool      `json:"read_only"`
	TokenType TokenType `json:"token_type"`
	// Token is the secret value; only returned by CreateToken.
	Token string `json:"token,omitempty"`
}

// CreateToken creates an access token for a user. The secret is only returned here.
// POST /v1/users/{username}/tokens
func (c *Client) CreateToken(ctx context.Context, username string, req CreateTokenRequest) (*Token, error) {
	var out Token
	err := c.do(ctx, http.MethodPost, "/v1/users/"+username+"/tokens", req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTokens lists a user's access tokens (without secret values).
// GET /v1/users/{username}/tokens
func (c *Client) ListTokens(ctx context.Context, username string) ([]Token, error) {
	var out struct {
		Tokens []Token `json:"tokens"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/users/"+username+"/tokens", nil, &out)
	if err != nil {
		return nil, err
	}
	return out.Tokens, nil
}

// DeleteToken invalidates a user access token.
// DELETE /v1/users/{username}/tokens/{token_id}
func (c *Client) DeleteToken(ctx context.Context, username, tokenID string) error {
	return c.do(ctx, http.MethodDelete, "/v1/users/"+username+"/tokens/"+tokenID, nil, nil)
}

// --- Ducklings (instance configuration) ---

type ReadWriteConfig struct {
	InstanceSize    string `json:"instance_size"` // pulse|standard|jumbo|mega|giga
	CooldownSeconds *int64 `json:"cooldown_seconds,omitempty"`
}

type ReadScalingConfig struct {
	InstanceSize    string  `json:"instance_size"` // pulse|standard|jumbo|mega|giga
	FlockSize       float64 `json:"flock_size"`    // 0..64
	CooldownSeconds *int64  `json:"cooldown_seconds,omitempty"`
}

type DucklingConfig struct {
	ReadWrite   ReadWriteConfig   `json:"read_write"`
	ReadScaling ReadScalingConfig `json:"read_scaling"`
}

// GetDucklingConfig gets the Duckling (instance) configuration for a user. Requires Admin.
// GET /v1/users/{username}/instances
func (c *Client) GetDucklingConfig(ctx context.Context, username string) (*DucklingConfig, error) {
	var out DucklingConfig
	err := c.do(ctx, http.MethodGet, "/v1/users/"+username+"/instances", nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SetDucklingConfig sets the Duckling (instance) configuration for a user. Requires Admin.
// PUT /v1/users/{username}/instances
func (c *Client) SetDucklingConfig(ctx context.Context, username string, cfg DucklingConfig) (*DucklingConfig, error) {
	var out DucklingConfig
	err := c.do(ctx, http.MethodPut, "/v1/users/"+username+"/instances", map[string]DucklingConfig{"config": cfg}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Active accounts ---

type Duckling struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type ActiveAccount struct {
	Username  string     `json:"username"`
	Ducklings []Duckling `json:"ducklings"`
}

// GetActiveAccounts lists active accounts and their active Ducklings. Requires Admin. [Preview]
// GET /v1/active_accounts
func (c *Client) GetActiveAccounts(ctx context.Context) ([]ActiveAccount, error) {
	var out struct {
		Accounts []ActiveAccount `json:"accounts"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/active_accounts", nil, &out)
	if err != nil {
		return nil, err
	}
	return out.Accounts, nil
}

// --- Dive embed sessions ---

type EmbedResource struct {
	URL   string `json:"url"`
	Alias string `json:"alias,omitempty"`
}

type CreateEmbedSessionRequest struct {
	Username          string          `json:"username"`
	SessionHint       string          `json:"session_hint,omitempty"`
	RequiredResources []EmbedResource `json:"required_resources,omitempty"`
	InitialState      json.RawMessage `json:"initial_state,omitempty"`
	Version           *int64          `json:"version,omitempty"`
}

// CreateEmbedSession creates an embed session for a Dive on behalf of a service account.
// POST /v1/dives/{dive_id}/embed-session
func (c *Client) CreateEmbedSession(ctx context.Context, diveID string, req CreateEmbedSessionRequest) (string, error) {
	var out struct {
		Session string `json:"session"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/dives/"+diveID+"/embed-session", req, &out)
	if err != nil {
		return "", err
	}
	return out.Session, nil
}
