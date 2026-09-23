package webapp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
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

// oauthFlowCookie holds oauthFlowState between handleSubmit's redirect and
// handleOAuthCallback, for the one sign-in attempt currently in flight.
// Nothing here is ever stored past that: no session, no token, in memory or
// otherwise.
const oauthFlowCookie = "oauth_flow"

// oauthFlowState is what handleSubmit stashes in oauthFlowCookie: the CSRF
// nonce (checked against the matching field of oauthState, which travels
// separately through the OAuth "state" parameter) and the PKCE verifier.
// Both must stay off any URL — unlike oauthState, which Google and the
// browser both see — so they live only here.
type oauthFlowState struct {
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
}

// oauthState is encoded into the OAuth "state" parameter handleSubmit sends
// to Google, and decoded back out of it by handleOAuthCallback: the CSRF
// nonce (see oauthFlowState) and everything needed to call
// InitiateProjectCreationAsUser once the token comes back. Nothing here is
// secret — state travels through the browser's address bar and Google's own
// systems — so nothing sensitive belongs here; in particular, never a token
// or the PKCE verifier.
type oauthState struct {
	Nonce          string                   `json:"nonce"`
	RequestID      domain.ProjectCreationID `json:"requestID"`
	DisplayName    string                   `json:"displayName"`
	Requester      string                   `json:"requester"`
	DeveloperGroup string                   `json:"developerGroup"`
}

func encodeOAuthState(s oauthState) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeOAuthState(raw string) (oauthState, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return oauthState{}, err
	}
	var s oauthState
	if err := json.Unmarshal(data, &s); err != nil {
		return oauthState{}, err
	}
	return s, nil
}
