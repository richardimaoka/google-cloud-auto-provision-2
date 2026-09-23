package webapp

import (
	"crypto/rand"
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
{{if .SignedIn}}
<p>Signed in.</p>
{{else}}
<p><a href="/login">Sign in with Google</a> to submit a request.</p>
{{end}}
<form method="post" action="/submit">
<p><label>Project display name<br><input type="text" name="display_name" required></label></p>
<p><label>Your email<br><input type="email" name="email" required></label></p>
<p><label>Google group email<br><input type="email" name="group" required></label></p>
<button type="submit"{{if not .SignedIn}} disabled{{end}}>Submit</button>
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

// formPageData is what formTemplate renders.
type formPageData struct {
	SignedIn bool
}

// resultData is what resultTemplate renders.
type resultData struct {
	ID          domain.ProjectCreationID
	DisplayName string
	Email       string
	Group       string
}

// Handler returns the HTTP handler for the web application: sign-in with
// Google, a form to enter a new project's display name, requester email, and
// developer group email, and a page showing what was submitted.
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
	mux.HandleFunc("GET /login", handleLogin(cfg))
	mux.HandleFunc("GET "+redirect.Path, handleOAuthCallback(cfg))
	mux.HandleFunc("POST /submit", handleSubmit(cfg))
	return mux
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	_, signedIn := getSession(r)
	formTemplate.Execute(w, formPageData{SignedIn: signedIn})
}

// handleSubmit initiates project creation under the signed-in caller's own
// OAuth-delegated credentials (see InitiateProjectCreationAsUser). It
// doesn't create anything yet: InitiateProjectCreationAsUser's own steps are
// still unimplemented, so this only exercises the HTTP-to-webapp plumbing.
func handleSubmit(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, signedIn := getSession(r)
		if !signedIn {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		request := InitiateProjectCreationRequest{
			ID:             domain.ProjectCreationID(rand.Text()),
			DisplayName:    r.FormValue("display_name"),
			Requester:      r.FormValue("email"),
			DeveloperGroup: r.FormValue("group"),
		}
		// cfg.TokenSource refreshes token itself if it's expired, unlike a bare
		// oauth2.StaticTokenSource(token).
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
