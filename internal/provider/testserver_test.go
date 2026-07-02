package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

// mockAPI is a stateful in-memory implementation of the MotherDuck REST API,
// used by both unit and acceptance tests via the provider's base_url.
type mockAPI struct {
	mu       sync.Mutex
	users    map[string]bool
	tokens   map[string][]client.Token // username -> tokens
	configs  map[string]client.DucklingConfig
	tokenSeq int
}

func newMockAPI() *mockAPI {
	return &mockAPI{
		users:   map[string]bool{},
		tokens:  map[string][]client.Token{},
		configs: map[string]client.DucklingConfig{},
	}
}

func (m *mockAPI) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	return srv.URL
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"message": msg, "code": code, "issues": []any{}})
}

func (m *mockAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid Credentials")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	// POST /v1/users
	case r.Method == http.MethodPost && r.URL.Path == "/v1/users":
		var body struct {
			Username string `json:"username"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Username == "" {
			writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "username required")
			return
		}
		m.users[body.Username] = true
		json.NewEncoder(w).Encode(map[string]string{"username": body.Username})

	// DELETE /v1/users/{username}
	case r.Method == http.MethodDelete && len(parts) == 3 && parts[1] == "users":
		username := parts[2]
		if !m.users[username] {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		delete(m.users, username)
		delete(m.tokens, username)
		delete(m.configs, username)
		json.NewEncoder(w).Encode(map[string]string{"username": username})

	// POST/GET /v1/users/{username}/tokens
	case len(parts) == 4 && parts[1] == "users" && parts[3] == "tokens":
		username := parts[2]
		if !m.users[username] {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body struct {
				Name      string `json:"name"`
				TTL       *int64 `json:"ttl"`
				TokenType string `json:"token_type"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.TokenType == "" {
				body.TokenType = "read_write"
			}
			m.tokenSeq++
			tok := client.Token{
				ID:        fmt.Sprintf("00000000-0000-0000-0000-%012d", m.tokenSeq),
				Name:      body.Name,
				CreatedTS: "2026-07-01T00:00:00Z",
				ReadOnly:  body.TokenType == "read_scaling",
				TokenType: client.TokenType(body.TokenType),
			}
			if body.TTL != nil {
				tok.ExpireAt = "2026-08-01T00:00:00Z"
			}
			m.tokens[username] = append(m.tokens[username], tok)
			out := tok
			out.Token = "secret-" + tok.ID
			json.NewEncoder(w).Encode(out)
		case http.MethodGet:
			list := m.tokens[username]
			if list == nil {
				list = []client.Token{}
			}
			json.NewEncoder(w).Encode(map[string]any{"tokens": list})
		}

	// DELETE /v1/users/{username}/tokens/{token_id}
	case r.Method == http.MethodDelete && len(parts) == 5 && parts[1] == "users" && parts[3] == "tokens":
		username, tokenID := parts[2], parts[4]
		list := m.tokens[username]
		for i, tok := range list {
			if tok.ID == tokenID {
				m.tokens[username] = append(list[:i], list[i+1:]...)
				w.Write([]byte("{}"))
				return
			}
		}
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "token not found")

	// GET/PUT /v1/users/{username}/instances
	case len(parts) == 4 && parts[1] == "users" && parts[3] == "instances":
		username := parts[2]
		switch r.Method {
		case http.MethodGet:
			cfg, ok := m.configs[username]
			if !ok {
				// Every real user has a default config.
				cfg = client.DucklingConfig{
					ReadWrite:   client.ReadWriteConfig{InstanceSize: "pulse"},
					ReadScaling: client.ReadScalingConfig{InstanceSize: "pulse", FlockSize: 0},
				}
			}
			json.NewEncoder(w).Encode(cfg)
		case http.MethodPut:
			var body struct {
				Config client.DucklingConfig `json:"config"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			m.configs[username] = body.Config
			json.NewEncoder(w).Encode(body.Config)
		}

	// GET /v1/active_accounts
	case r.Method == http.MethodGet && r.URL.Path == "/v1/active_accounts":
		accounts := []map[string]any{}
		for u := range m.users {
			accounts = append(accounts, map[string]any{
				"username":  u,
				"ducklings": []map[string]string{{"id": "d-" + u, "type": "read_write", "status": "running"}},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"accounts": accounts})

	// POST /v1/dives/{dive_id}/embed-session
	case r.Method == http.MethodPost && len(parts) == 4 && parts[1] == "dives" && parts[3] == "embed-session":
		json.NewEncoder(w).Encode(map[string]string{"session": "embed-session-" + parts[2]})

	default:
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no route: "+r.Method+" "+r.URL.Path)
	}
}
