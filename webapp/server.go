package webapp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

var formTemplate = template.Must(template.New("form").Parse(`<!DOCTYPE html>
<html>
<head><title>New Project</title></head>
<body>
<h1>New Google Cloud project</h1>
<form method="post" action="/submit">
<p><label>Project display name<br><input type="text" name="display_name" required></label></p>
<p><label>Your email<br><input type="email" name="email" required></label></p>
<p><label>Google group email<br><input type="email" name="group" required></label></p>
<button type="submit">Submit</button>
</form>
</body>
</html>
`))

var resultTemplate = template.Must(template.New("result").Parse(`<!DOCTYPE html>
<html>
<head><title>Submitted</title></head>
<body>
<h1>Submitted</h1>
<ul>
<li>Request ID: {{.ID}}</li>
<li>Project display name: {{.DisplayName}}</li>
<li>Your email: {{.Email}}</li>
<li>Google group email: {{.Group}}</li>
</ul>
</body>
</html>
`))

// resultData is what resultTemplate renders.
type resultData struct {
	ID          domain.ProjectCreationID
	DisplayName string
	Email       string
	Group       string
}

// Handler returns the HTTP handler for the web application: a form to enter
// a new project's display name, requester email, and developer group email,
// which signs the caller in with Google as part of submitting it, and a page
// showing what was submitted.
//
// cfg comes from LoadOAuthConfig. Its RedirectURL's path is served for
// Google's sign-in redirect, alongside the form's own routes.
func Handler(cfg *oauth2.Config) http.Handler {
	redirect, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		// LoadOAuthConfig already validated this; a hand-built cfg with a bad
		// RedirectURL is a programmer error, not something to recover from.
		panic("webapp.Handler: invalid cfg.RedirectURL: " + err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleForm)
	mux.HandleFunc("POST /submit", handleSubmit(cfg))
	mux.HandleFunc("GET "+redirect.Path, handleOAuthCallback(cfg))
	return mux
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	formTemplate.Execute(w, nil)
}

// handleSubmit starts a sign-in for the submitted request: it stashes a
// fresh CSRF nonce and PKCE verifier in oauthFlowCookie, encodes the same
// nonce and the form data into the OAuth "state" parameter, and redirects to
// Google's consent screen. handleOAuthCallback does the rest once the caller
// comes back — immediately, if they're already signed in to Google in this
// browser.
func handleSubmit(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		flow := oauthFlowState{Nonce: rand.Text(), Verifier: oauth2.GenerateVerifier()}
		flowData, err := json.Marshal(flow)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     oauthFlowCookie,
			Value:    base64.RawURLEncoding.EncodeToString(flowData),
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		state, err := encodeOAuthState(oauthState{
			Nonce:          flow.Nonce,
			RequestID:      domain.ProjectCreationID(rand.Text()),
			DisplayName:    r.FormValue("display_name"),
			Requester:      r.FormValue("email"),
			DeveloperGroup: r.FormValue("group"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Ask for a refresh token: without AccessTypeOffline the access token
		// alone would go stale during a slow, multi-step initial setup.
		authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(flow.Verifier))
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// handleOAuthCallback completes a sign-in started by handleSubmit: it checks
// the CSRF nonce, exchanges the code for a token using the PKCE verifier,
// and calls InitiateProjectCreationAsUser with that token — used once, right
// here, and never stored anywhere.
func handleOAuthCallback(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(oauthFlowCookie)
		if err != nil {
			http.Error(w, "no sign-in in progress", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauthFlowCookie, Path: "/", MaxAge: -1})

		flowData, err := base64.RawURLEncoding.DecodeString(c.Value)
		if err != nil {
			http.Error(w, "invalid sign-in state", http.StatusBadRequest)
			return
		}
		var flow oauthFlowState
		if err := json.Unmarshal(flowData, &flow); err != nil {
			http.Error(w, "invalid sign-in state", http.StatusBadRequest)
			return
		}

		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed: "+e, http.StatusBadRequest)
			return
		}
		state, err := decodeOAuthState(q.Get("state"))
		if err != nil {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}
		if state.Nonce != flow.Nonce {
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

		request := InitiateProjectCreationRequest{
			ID:             state.RequestID,
			DisplayName:    state.DisplayName,
			Requester:      state.Requester,
			DeveloperGroup: state.DeveloperGroup,
		}
		// cfg.TokenSource refreshes token itself if it goes stale during the
		// initial setup, unlike a bare oauth2.StaticTokenSource(token).
		callerCredentials := option.WithTokenSource(cfg.TokenSource(r.Context(), token))
		resp, err := InitiateProjectCreationAsUser(r.Context(), request, callerCredentials)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		resultTemplate.Execute(w, resultData{
			ID:          resp.ID,
			DisplayName: request.DisplayName,
			Email:       request.Requester,
			Group:       request.DeveloperGroup,
		})
	}
}
