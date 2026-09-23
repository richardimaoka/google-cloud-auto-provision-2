package webapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// testOAuthConfig is a config Handler can route with, but that never reaches
// the network: every test below either never calls Google, or uses a token
// with no expiry, which oauth2.Config.TokenSource returns without a refresh.
func testOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/oauth2/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}
}

func TestHandleForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handler(testOAuthConfig()).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Sign in with Google") {
		t.Error("form doesn't show a sign-in link when not signed in")
	}
	if !strings.Contains(body, "disabled") {
		t.Error("submit button isn't disabled when not signed in")
	}
}

func TestHandleSubmit_RequiresSignIn(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("display_name=Test&email=a@example.com&group=g@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	Handler(testOAuthConfig()).ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

func TestHandleSubmit(t *testing.T) {
	sessionID := putSession(&oauth2.Token{AccessToken: "fake-token"})

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("display_name=Test+Project&email=a@example.com&group=g@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sessionID})
	w := httptest.NewRecorder()
	Handler(testOAuthConfig()).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"Test Project", "a@example.com", "g@example.com"} {
		if !strings.Contains(body, want) {
			t.Errorf("body doesn't contain %q:\n%s", want, body)
		}
	}
}
