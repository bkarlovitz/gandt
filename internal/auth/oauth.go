package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	ErrOAuthStateMismatch = errors.New("oauth state mismatch")
	ErrOAuthMissingCode   = errors.New("oauth callback missing code")
	ErrOAuthTimeout       = errors.New("oauth callback timed out")
)

var GmailOAuthScopes = []string{
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/userinfo.email",
}

type BrowserOpener func(string) error

type LoopbackOAuthOptions struct {
	Endpoint      oauth2.Endpoint
	Scopes        []string
	OpenBrowser   BrowserOpener
	ListenAddress string
	CallbackPath  string
	Timeout       time.Duration
}

func RunLoopbackOAuth(ctx context.Context, credentials ClientCredentials, options LoopbackOAuthOptions) (*oauth2.Token, error) {
	credentials = normalizeClientCredentials(credentials)
	if err := ValidateClientCredentials(credentials); err != nil {
		return nil, err
	}

	options = normalizeLoopbackOptions(options)
	listener, err := net.Listen("tcp", options.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen for oauth callback: %w", err)
	}

	state, err := randomState()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	redirectURL := "http://" + listener.Addr().String() + options.CallbackPath
	config := oauth2.Config{
		ClientID:     credentials.ClientID,
		ClientSecret: credentials.ClientSecret,
		Endpoint:     options.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       options.Scopes,
	}

	resultCh := make(chan oauthCallbackResult, 1)
	server := &http.Server{
		Handler: oauthCallbackHandler(options.CallbackPath, state, resultCh),
	}
	defer shutdownOAuthServer(server)

	serveErrCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
		}
	}()

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if err := options.OpenBrowser(authURL); err != nil {
		return nil, fmt.Errorf("open oauth browser: %w", err)
	}

	waitCtx := ctx
	cancel := func() {}
	if options.Timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	}
	defer cancel()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		token, err := config.Exchange(ctx, result.code)
		if err != nil {
			return nil, fmt.Errorf("exchange oauth code: %w", err)
		}
		return token, nil
	case err := <-serveErrCh:
		return nil, fmt.Errorf("serve oauth callback: %w", err)
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrOAuthTimeout
		}
		return nil, waitCtx.Err()
	}
}

func normalizeLoopbackOptions(options LoopbackOAuthOptions) LoopbackOAuthOptions {
	if options.Endpoint.AuthURL == "" || options.Endpoint.TokenURL == "" {
		options.Endpoint = google.Endpoint
	}
	if len(options.Scopes) == 0 {
		options.Scopes = append([]string{}, GmailOAuthScopes...)
	}
	if options.OpenBrowser == nil {
		options.OpenBrowser = browser.OpenURL
	}
	if options.ListenAddress == "" {
		options.ListenAddress = "127.0.0.1:0"
	}
	if options.CallbackPath == "" {
		options.CallbackPath = "/callback"
	}
	if !strings.HasPrefix(options.CallbackPath, "/") {
		options.CallbackPath = "/" + options.CallbackPath
	}
	if options.Timeout == 0 {
		options.Timeout = 2 * time.Minute
	}
	return options
}

func oauthCallbackHandler(path string, state string, resultCh chan<- oauthCallbackResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}

		query := r.URL.Query()
		if query.Get("state") != state {
			sendOAuthCallbackResult(resultCh, oauthCallbackResult{err: ErrOAuthStateMismatch})
			http.Error(w, "OAuth state mismatch.", http.StatusBadRequest)
			return
		}
		if callbackErr := query.Get("error"); callbackErr != "" {
			sendOAuthCallbackResult(resultCh, oauthCallbackResult{err: fmt.Errorf("oauth callback error: %s", callbackErr)})
			http.Error(w, "OAuth failed.", http.StatusBadRequest)
			return
		}
		code := query.Get("code")
		if code == "" {
			sendOAuthCallbackResult(resultCh, oauthCallbackResult{err: ErrOAuthMissingCode})
			http.Error(w, "OAuth code missing.", http.StatusBadRequest)
			return
		}

		sendOAuthCallbackResult(resultCh, oauthCallbackResult{code: code})
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("G&T authentication complete. You can close this browser tab."))
	})
}

type oauthCallbackResult struct {
	code string
	err  error
}

func sendOAuthCallbackResult(resultCh chan<- oauthCallbackResult, result oauthCallbackResult) {
	select {
	case resultCh <- result:
	default:
	}
}

func randomState() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func shutdownOAuthServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
