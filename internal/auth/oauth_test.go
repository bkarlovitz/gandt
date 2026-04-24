package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRunLoopbackOAuthSuccess(t *testing.T) {
	tokenServer := oauthTokenServer(t, "code-123")
	defer tokenServer.Close()

	var openedURL string
	token, err := RunLoopbackOAuth(context.Background(), ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, LoopbackOAuthOptions{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.example.test/auth",
			TokenURL: tokenServer.URL,
		},
		Timeout: time.Second,
		OpenBrowser: func(authURL string) error {
			openedURL = authURL
			go requestOAuthCallback(t, authURL, "code-123", "")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run loopback oauth: %v", err)
	}
	if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" {
		t.Fatalf("token = %#v, want test token", token)
	}

	values := parseQuery(t, openedURL)
	if values.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q, want client-id", values.Get("client_id"))
	}
	if values.Get("response_type") != "code" {
		t.Fatalf("response_type = %q, want code", values.Get("response_type"))
	}
	for _, scope := range GmailOAuthScopes {
		if !strings.Contains(values.Get("scope"), scope) {
			t.Fatalf("scope %q missing from %q", scope, values.Get("scope"))
		}
	}
	if values.Get("redirect_uri") == "" || values.Get("state") == "" {
		t.Fatalf("auth URL missing redirect_uri or state: %s", openedURL)
	}
}

func TestRunLoopbackOAuthRejectsBadState(t *testing.T) {
	tokenServer := oauthTokenServer(t, "code-123")
	defer tokenServer.Close()

	_, err := RunLoopbackOAuth(context.Background(), ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, LoopbackOAuthOptions{
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: tokenServer.URL},
		Timeout:  time.Second,
		OpenBrowser: func(authURL string) error {
			go requestOAuthCallback(t, authURL, "code-123", "wrong-state")
			return nil
		},
	})
	if !errors.Is(err, ErrOAuthStateMismatch) {
		t.Fatalf("error = %v, want ErrOAuthStateMismatch", err)
	}
}

func TestRunLoopbackOAuthRejectsMissingCode(t *testing.T) {
	tokenServer := oauthTokenServer(t, "code-123")
	defer tokenServer.Close()

	_, err := RunLoopbackOAuth(context.Background(), ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, LoopbackOAuthOptions{
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: tokenServer.URL},
		Timeout:  time.Second,
		OpenBrowser: func(authURL string) error {
			go requestOAuthCallback(t, authURL, "", "")
			return nil
		},
	})
	if !errors.Is(err, ErrOAuthMissingCode) {
		t.Fatalf("error = %v, want ErrOAuthMissingCode", err)
	}
}

func TestRunLoopbackOAuthTimeout(t *testing.T) {
	tokenServer := oauthTokenServer(t, "code-123")
	defer tokenServer.Close()

	_, err := RunLoopbackOAuth(context.Background(), ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, LoopbackOAuthOptions{
		Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: tokenServer.URL},
		Timeout:     10 * time.Millisecond,
		OpenBrowser: func(string) error { return nil },
	})
	if !errors.Is(err, ErrOAuthTimeout) {
		t.Fatalf("error = %v, want ErrOAuthTimeout", err)
	}
}

func TestRunLoopbackOAuthCancellation(t *testing.T) {
	tokenServer := oauthTokenServer(t, "code-123")
	defer tokenServer.Close()
	ctx, cancel := context.WithCancel(context.Background())

	_, err := RunLoopbackOAuth(ctx, ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, LoopbackOAuthOptions{
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.test/auth", TokenURL: tokenServer.URL},
		Timeout:  time.Second,
		OpenBrowser: func(string) error {
			cancel()
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func oauthTokenServer(t *testing.T, wantCode string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token request: %v", err)
		}
		if got := r.Form.Get("code"); got != wantCode {
			t.Fatalf("token code = %q, want %q", got, wantCode)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
}

func requestOAuthCallback(t *testing.T, authURL string, code string, stateOverride string) {
	t.Helper()

	values := parseQuery(t, authURL)
	callbackURL := values.Get("redirect_uri")
	state := values.Get("state")
	if stateOverride != "" {
		state = stateOverride
	}

	query := url.Values{}
	query.Set("state", state)
	if code != "" {
		query.Set("code", code)
	}

	resp, err := http.Get(callbackURL + "?" + query.Encode())
	if err != nil {
		t.Errorf("request oauth callback: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func parseQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	return parsed.Query()
}
