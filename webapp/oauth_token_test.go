//go:build oauthtoken

package webapp

// This is an interactive helper, not a regular test. It starts a local web
// server, prints a Google sign-in URL, and waits for you to open it in a
// browser and approve access. The token Google returns is saved to
// testdata/token.json, which is git-ignored.
//
// The OAuth client comes from secrets/client_secret.json under the
// repository root, the file the Cloud Console offers as "Download JSON" (see
// oauth-client.md). It
// must be a Web application client with exactly one redirect URI, an http URL
// on localhost with a port and a path, because this test serves that URI
// itself. Run, from the repository root:
//
//	go test -tags oauthtoken -count=1 -v ./webapp
//
// -v is needed to see the URL while the test waits, and -count=1 stops go test
// from replaying a cached pass instead of asking again.

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
)

const (
	// Relative to this package's directory, where go test runs.
	clientSecretPath = "../secrets/client_secret.json"
	tokenPath        = "testdata/token.json"
	authTimeout      = 5 * time.Minute
)

// parseClientConfig and localRedirectURL live in oauth.go, shared with the
// real server.

// loadOAuthConfig reads and parses clientSecretPath, failing the test if it can't.
func loadOAuthConfig(t *testing.T) (*oauth2.Config, *url.URL) {
	t.Helper()

	data, err := os.ReadFile(clientSecretPath)
	if err != nil {
		t.Fatalf("read OAuth client file: %v (download it from the Cloud Console, see oauth-client.md)", err)
	}
	cfg, redirect, err := parseClientConfig(data)
	if err != nil {
		t.Fatalf("%s: %v", clientSecretPath, err)
	}
	return cfg, redirect
}

// TestObtainOAuthToken runs the interactive flow described above and saves
// the resulting token to tokenPath.
func TestObtainOAuthToken(t *testing.T) {
	cfg, redirect := loadOAuthConfig(t)

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
