//go:build oauthtoken

package webapp

// This is an interactive helper, not a regular test. It starts a local web
// server, prints a Google sign-in URL, and waits for you to open it in a
// browser and approve access. The token Google returns is saved to
// testdata/token.json, which is git-ignored.
//
// Put OAUTH_CLIENT_ID, OAUTH_CLIENT_SECRET and OAUTH_REDIRECT_URL (see
// .env.example) in .env and run, from the repository root:
//
//	set -a; . ./.env; set +a; go test -tags oauthtoken -count=1 -v ./webapp
//
// -v is needed to see the URL while the test waits, and -count=1 stops go test
// from replaying a cached pass instead of asking again. OAUTH_REDIRECT_URL
// must be a localhost URL that is also registered as an authorized redirect URI
// on the OAuth client.

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	tokenPath   = "testdata/token.json"
	authTimeout = 5 * time.Minute
)

var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("%s is not set", key)
	}
	return v
}

func TestObtainOAuthToken(t *testing.T) {
	cfg := &oauth2.Config{
		ClientID:     requireEnv(t, "OAUTH_CLIENT_ID"),
		ClientSecret: requireEnv(t, "OAUTH_CLIENT_SECRET"),
		RedirectURL:  requireEnv(t, "OAUTH_REDIRECT_URL"),
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	redirect, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		t.Fatalf("parse OAUTH_REDIRECT_URL: %v", err)
	}
	if h := redirect.Hostname(); h != "localhost" && h != "127.0.0.1" {
		t.Fatalf("OAUTH_REDIRECT_URL host is %q, want localhost or 127.0.0.1", h)
	}
	ln, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		t.Fatalf("listen on %s: %v", redirect.Host, err)
	}

	state := rand.Text()
	verifier := oauth2.GenerateVerifier()
	done := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		err := saveToken(r, cfg, state, verifier)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			fmt.Fprintf(w, "Token saved to %s. You can close this tab.\n", tokenPath)
		}
		select {
		case done <- err:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})

	// Ask for a refresh token, and force the consent screen so Google issues
	// one even if this client was authorized before.
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier))
	t.Logf("Open this URL in your browser to authorize:\n\n%s\n", authURL)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("obtain token: %v", err)
		}
	case <-time.After(authTimeout):
		t.Fatalf("no authorization within %s", authTimeout)
	}
	t.Logf("saved token to %s", tokenPath)
}

// saveToken handles the OAuth redirect: it checks state, exchanges the code for
// a token, and writes the token to tokenPath.
func saveToken(r *http.Request, cfg *oauth2.Config, state, verifier string) error {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return fmt.Errorf("authorization failed: %s", e)
	}
	if q.Get("state") != state {
		return errors.New("state mismatch")
	}
	code := q.Get("code")
	if code == "" {
		return errors.New("no code in redirect")
	}

	token, err := cfg.Exchange(r.Context(), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(tokenPath, data, 0o600)
}
