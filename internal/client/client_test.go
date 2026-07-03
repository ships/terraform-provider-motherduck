package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-token", "test-agent")
}

func TestAuthHeaderAndUserAgent(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("User-Agent = %q", got)
		}
		json.NewEncoder(w).Encode(map[string]string{"username": "svc"})
	})

	if _, err := c.CreateServiceAccount(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateServiceAccount(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/users" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "svc_etl" {
			t.Errorf("username = %q", body["username"])
		}
		json.NewEncoder(w).Encode(map[string]string{"username": "svc_etl"})
	})

	user, err := c.CreateServiceAccount(context.Background(), "svc_etl")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "svc_etl" {
		t.Errorf("Username = %q", user.Username)
	}
}

func TestCreateToken(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/users/svc/tokens" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "ci" || body["token_type"] != "read_write" || body["ttl"] != float64(3600) {
			t.Errorf("body = %v", body)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"token": "secret", "id": "tok-1", "name": "ci",
			"created_ts": "2026-07-01T00:00:00Z", "expire_at": "2026-07-01T01:00:00Z",
			"read_only": false, "token_type": "read_write",
		})
	})

	ttl := int64(3600)
	tok, err := c.CreateToken(context.Background(), "svc", CreateTokenRequest{Name: "ci", TTL: &ttl, TokenType: TokenTypeReadWrite})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "secret" || tok.ID != "tok-1" {
		t.Errorf("token = %+v", tok)
	}
}

func TestListTokens(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tokens": []map[string]any{
				{"id": "a", "name": "one", "created_ts": "x", "read_only": false, "token_type": "read_write"},
				{"id": "b", "name": "two", "created_ts": "y", "read_only": true, "token_type": "read_scaling"},
			},
		})
	})

	tokens, err := c.ListTokens(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 || tokens[1].TokenType != TokenTypeReadScaling {
		t.Errorf("tokens = %+v", tokens)
	}
}

func TestDucklingConfigRoundTrip(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var body struct {
				Config DucklingConfig `json:"config"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.Config.ReadWrite.InstanceSize != "jumbo" || body.Config.ReadScaling.FlockSize != 2 {
				t.Errorf("config = %+v", body.Config)
			}
			json.NewEncoder(w).Encode(body.Config)
			return
		}
		json.NewEncoder(w).Encode(DucklingConfig{
			ReadWrite:   ReadWriteConfig{InstanceSize: "standard"},
			ReadScaling: ReadScalingConfig{InstanceSize: "pulse", FlockSize: 1},
		})
	})

	got, err := c.GetDucklingConfig(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	if got.ReadWrite.InstanceSize != "standard" {
		t.Errorf("got = %+v", got)
	}

	cooldown := int64(300)
	set, err := c.SetDucklingConfig(context.Background(), "svc", DucklingConfig{
		ReadWrite:   ReadWriteConfig{InstanceSize: "jumbo", CooldownSeconds: &cooldown},
		ReadScaling: ReadScalingConfig{InstanceSize: "jumbo", FlockSize: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if set.ReadWrite.InstanceSize != "jumbo" {
		t.Errorf("set = %+v", set)
	}
}

func TestGetActiveAccounts(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/active_accounts" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"accounts": []map[string]any{
				{"username": "u1", "ducklings": []map[string]string{{"id": "d1", "type": "read_write", "status": "running"}}},
			},
		})
	})

	accounts, err := c.GetActiveAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Ducklings[0].ID != "d1" {
		t.Errorf("accounts = %+v", accounts)
	}
}

func TestCreateEmbedSession(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dives/dive-1/embed-session" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "svc" {
			t.Errorf("body = %v", body)
		}
		if _, hasState := body["initial_state"]; !hasState {
			t.Error("initial_state missing")
		}
		json.NewEncoder(w).Encode(map[string]string{"session": "sess-token"})
	})

	session, err := c.CreateEmbedSession(context.Background(), "dive-1", CreateEmbedSessionRequest{
		Username:     "svc",
		InitialState: json.RawMessage(`{"filters":{}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if session != "sess-token" {
		t.Errorf("session = %q", session)
	}
}

func TestAPIErrorParsing(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Not Found", "code": "NOT_FOUND",
			"issues": []map[string]string{{"message": "no such user"}},
		})
	})

	_, err := c.ListTokens(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound = false for %v", err)
	}
	apiErr := err.(*APIError)
	if apiErr.Code != "NOT_FOUND" || len(apiErr.Issues) != 1 {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestNonJSONError(t *testing.T) {
	c := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("upstream broke"))
	})

	err := c.DeleteUser(context.Background(), "svc")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := err.(*APIError)
	if apiErr.StatusCode != http.StatusBadGateway || apiErr.Message != "upstream broke" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}
