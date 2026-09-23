package webapp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// scopes are the OAuth scopes this app requests when signing a user in. Keep
// this in sync with oauth-client.md's Data access section.
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
// port and path, so a local server can listen on it. The port and path the
// server uses come from the same URL, so they can't differ from what Google
// is told.
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

// LoadOAuthConfig reads and parses the OAuth client file at path — the file
// the Cloud Console offers as "Download JSON" (see oauth-client.md) — into
// the config Handler needs for sign-in.
func LoadOAuthConfig(path string) (*oauth2.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OAuth client file: %w (download it from the Cloud Console, see oauth-client.md)", err)
	}
	cfg, _, err := parseClientConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// sessions holds signed-in users' tokens, keyed by a random ID stored in the
// sessionCookie. In-memory only: it doesn't survive a restart, and doesn't
// work past a single instance. That's fine to see the sign-in flow work; a
// real deployment needs a shared store instead.
var sessions = struct {
	mu sync.Mutex
	m  map[string]*oauth2.Token
}{m: map[string]*oauth2.Token{}}

const sessionCookie = "session"

func putSession(token *oauth2.Token) string {
	id := rand.Text()
	sessions.mu.Lock()
	sessions.m[id] = token
	sessions.mu.Unlock()
	return id
}

func getSession(r *http.Request) (*oauth2.Token, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, false
	}
	sessions.mu.Lock()
	token, ok := sessions.m[c.Value]
	sessions.mu.Unlock()
	return token, ok
}

// oauthFlowCookie holds oauthFlowState between handleLogin's redirect and
// handleOAuthCallback, for the one sign-in attempt currently in flight.
const oauthFlowCookie = "oauth_flow"

// oauthFlowState is what handleLogin stashes in oauthFlowCookie: the CSRF
// state and PKCE verifier handleOAuthCallback must see again to accept the
// redirect back from Google.
type oauthFlowState struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
}

// handleLogin starts a sign-in: it stashes a fresh CSRF state and PKCE
// verifier in oauthFlowCookie, then redirects to Google's consent screen.
func handleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flow := oauthFlowState{State: rand.Text(), Verifier: oauth2.GenerateVerifier()}
		data, err := json.Marshal(flow)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     oauthFlowCookie,
			Value:    base64.RawURLEncoding.EncodeToString(data),
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		// Ask for a refresh token: without AccessTypeOffline the access token
		// alone would go stale during a slow, multi-step initial setup.
		authURL := cfg.AuthCodeURL(flow.State, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(flow.Verifier))
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// handleOAuthCallback completes a sign-in: it checks state, exchanges the
// code for a token, and stores the token under a new sessionCookie.
func handleOAuthCallback(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(oauthFlowCookie)
		if err != nil {
			http.Error(w, "no sign-in in progress", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauthFlowCookie, Path: "/", MaxAge: -1})

		raw, err := base64.RawURLEncoding.DecodeString(c.Value)
		if err != nil {
			http.Error(w, "invalid sign-in state", http.StatusBadRequest)
			return
		}
		var flow oauthFlowState
		if err := json.Unmarshal(raw, &flow); err != nil {
			http.Error(w, "invalid sign-in state", http.StatusBadRequest)
			return
		}

		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed: "+e, http.StatusBadRequest)
			return
		}
		if q.Get("state") != flow.State {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "no code in redirect", http.StatusBadRequest)
			return
		}

		token, err := cfg.Exchange(r.Context(), code, oauth2.VerifierOption(flow.Verifier))
		if err != nil {
			http.Error(w, "exchange code: "+err.Error(), http.StatusBadGateway)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    putSession(token),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
