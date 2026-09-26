package webapp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
)

// testOAuthConfig is a config Handler can route with. AuthURL isn't reached
// by these tests (they inspect the redirect instead of following it);
// TokenURL is overridden per test that needs a real exchange.
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

// TestHandleForm checks that the form page renders.
func TestHandleForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handler(testOAuthConfig(), NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `name="display_name"`) {
		t.Error("form doesn't have the display_name field")
	}
}

// TestHandleSubmit_RedirectsToGoogle checks that submitting starts sign-in directly.
func TestHandleSubmit_RedirectsToGoogle(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("display_name=Test&email=a@example.com&group=g@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	Handler(testOAuthConfig(), NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != "https://example.com/auth" {
		t.Errorf("redirected to %q, want the configured AuthURL", got)
	}
	// Presence only; contents are checked in TestHandleOAuthCallback_StateMismatch.
	if loc.Query().Get("state") == "" {
		t.Error("redirect has no state parameter")
	}
	if loc.Query().Get("code_challenge") == "" {
		t.Error("redirect has no PKCE code_challenge")
	}

	var flowCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthFlowCookie {
			flowCookie = c
		}
	}
	if flowCookie == nil {
		t.Fatal("submit didn't set the oauth flow cookie")
	}
}

// TestHandleSubmit drives the whole round trip: submit, then simulate
// Google's callback, against a fake token endpoint.
func TestHandleSubmit(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fake-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	cfg := testOAuthConfig()
	cfg.Endpoint.TokenURL = tokenServer.URL

	submitReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("display_name=Test+Project&email=a@example.com&group=developer-group@example.com"))
	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	submitW := httptest.NewRecorder()
	Handler(cfg, NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(submitW, submitReq)

	if submitW.Code != http.StatusFound {
		t.Fatalf("submit status = %d, want %d", submitW.Code, http.StatusFound)
	}
	authURL, err := url.Parse(submitW.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	// Opaque here; forwarded below, not decoded.
	state := authURL.Query().Get("state")

	var flowCookie *http.Cookie
	for _, c := range submitW.Result().Cookies() {
		if c.Name == oauthFlowCookie {
			flowCookie = c
		}
	}
	if flowCookie == nil {
		t.Fatal("submit didn't set the oauth flow cookie")
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state="+url.QueryEscape(state)+"&code=fake-code", nil)
	callbackReq.AddCookie(flowCookie)
	callbackW := httptest.NewRecorder()
	// The callback runs InitiateProjectCreationAsUser, so it needs working fakes.
	projects := NewProjectCreationService(newFakeProgressStore(), &fakeProjectCreator{projectID: "example-project"}, newFakeGoogleGroups(t), &fakeJobStarter{}, testOrganization(t), testBreakGlassRequirement(t))
	Handler(cfg, projects).ServeHTTP(callbackW, callbackReq)

	if callbackW.Code != http.StatusOK {
		t.Fatalf("callback status = %d, body = %s", callbackW.Code, callbackW.Body.String())
	}
	body := callbackW.Body.String()
	for _, want := range []string{"Test Project", "a@example.com", "developer-group@example.com"} {
		if !strings.Contains(body, want) {
			t.Errorf("body doesn't contain %q:\n%s", want, body)
		}
	}
}

// TestHandleOAuthCallback_NoFlowCookie checks that the callback rejects a
// request with no prior /submit (no oauthFlowCookie).
func TestHandleOAuthCallback_NoFlowCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=x&code=y", nil)
	w := httptest.NewRecorder()
	Handler(testOAuthConfig(), NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestHandleOAuthCallback_StateMismatch checks the CSRF check: a state whose
// nonce doesn't match the flow cookie's is rejected.
func TestHandleOAuthCallback_StateMismatch(t *testing.T) {
	submitReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("display_name=Test&email=a@example.com&group=g@example.com"))
	submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	submitW := httptest.NewRecorder()
	Handler(testOAuthConfig(), NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(submitW, submitReq)

	var flowCookie *http.Cookie
	for _, c := range submitW.Result().Cookies() {
		if c.Name == oauthFlowCookie {
			flowCookie = c
		}
	}
	if flowCookie == nil {
		t.Fatal("submit didn't set the oauth flow cookie")
	}

	badState, err := encodeOAuthState(oauthState{Nonce: "wrong"})
	if err != nil {
		t.Fatal(err)
	}
	callbackReq := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state="+url.QueryEscape(badState)+"&code=y", nil)
	callbackReq.AddCookie(flowCookie)
	w := httptest.NewRecorder()
	Handler(testOAuthConfig(), NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})).ServeHTTP(w, callbackReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
