//go:build oauthtoken

package webapp

// This is an interactive helper, not a regular test. It starts a local web
// server, prints a Google sign-in URL, and waits for you to open it in a
// browser and approve access. The token Google returns is saved to
// testdata/token.json, which is git-ignored.
//
// The OAuth client comes from client_secret.json in the repository root, the
// file the Cloud Console offers as "Download JSON" (see oauth-client.md). It
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
	"golang.org/x/oauth2/google"
)

const (
	// Relative to this package's directory, where go test runs.
	clientSecretPath = "../client_secret.json"
	tokenPath        = "testdata/token.json"
	authTimeout      = 5 * time.Minute
)

var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

// parseClientConfig builds the OAuth config from the contents of a downloaded
// client JSON, and returns its redirect URI. google.ConfigFromJSON silently
// uses only the first redirect URI, so this insists there is exactly one.
func parseClientConfig(data []byte) (*oauth2.Config, *url.URL, error) {
	var f struct {
		Web struct {
			RedirectURIs []string `json:"redirect_uris"`
		} `json:"web"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, err
	}
	if n := len(f.Web.RedirectURIs); n != 1 {
		return nil, nil, fmt.Errorf("has %d redirect URIs in its \"web\" section, want exactly 1", n)
	}

	redirect, err := localRedirectURL(f.Web.RedirectURIs[0])
	if err != nil {
		return nil, nil, fmt.Errorf("redirect URI: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, nil, err
	}
	return cfg, redirect, nil
}

// localRedirectURL checks that raw is an http URL on localhost with an explicit
// port and path, so this test can listen on it. The port and path the server
// uses come from the same URL, so they can't differ from what Google is told.
func localRedirectURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("scheme is %q, want http", u.Scheme)
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" {
		return nil, fmt.Errorf("host is %q, want localhost or 127.0.0.1", h)
	}
	if u.Port() == "" {
		return nil, errors.New("no port")
	}
	if u.Path == "" {
		return nil, errors.New("no path")
	}
	return u, nil
}

func loadConfig(t *testing.T) (*oauth2.Config, *url.URL) {
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

func TestObtainOAuthToken(t *testing.T) {
	cfg, redirect := loadConfig(t)

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

func TestParseClientConfig(t *testing.T) {
	client := func(uris ...string) []byte {
		b, _ := json.Marshal(map[string]any{"web": map[string]any{
			"client_id":     "id.apps.googleusercontent.com",
			"client_secret": "not-a-real-secret",
			"redirect_uris": uris,
		}})
		return b
	}

	cases := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "localhostのURIが1つ", data: client("http://localhost:8080/oauth2/callback")},
		{name: "127.0.0.1のURIが1つ", data: client("http://127.0.0.1:8080/oauth2/callback")},
		{name: "URIが複数", data: client("http://localhost:8080/a", "http://localhost:8080/b"), wantErr: true},
		{name: "URIがない", data: client(), wantErr: true},
		{name: "httpsのURI", data: client("https://localhost:8080/cb"), wantErr: true},
		{name: "localhost以外のホスト", data: client("http://example.com:8080/cb"), wantErr: true},
		{name: "ポートがない", data: client("http://localhost/cb"), wantErr: true},
		{name: "パスがない", data: client("http://localhost:8080"), wantErr: true},
		{name: "webではなくinstalled", data: []byte(`{"installed":{"client_id":"x","client_secret":"y","redirect_uris":["http://localhost:8080/cb"]}}`), wantErr: true},
		{name: "JSONではない", data: []byte("not json"), wantErr: true},
	}
	for _, c := range cases {
		cfg, redirect, err := parseClientConfig(c.data)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if err == nil && cfg.RedirectURL != redirect.String() {
			t.Errorf("%s: cfg.RedirectURL = %q, want %q", c.name, cfg.RedirectURL, redirect.String())
		}
	}
}
