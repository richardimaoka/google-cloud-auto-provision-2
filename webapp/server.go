package webapp

import (
	"html/template"
	"net/http"
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
<li>Project display name: {{.DisplayName}}</li>
<li>Your email: {{.Email}}</li>
<li>Google group email: {{.Group}}</li>
</ul>
</body>
</html>
`))

// formData holds the values entered on the form.
type formData struct {
	DisplayName string
	Email       string
	Group       string
}

// Handler returns the HTTP handler for the web application: a form to enter a
// new project's display name, requester email, and developer group email, and
// a page showing what was entered.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleForm)
	mux.HandleFunc("POST /submit", handleSubmit)
	return mux
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	formTemplate.Execute(w, nil)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data := formData{
		DisplayName: r.FormValue("display_name"),
		Email:       r.FormValue("email"),
		Group:       r.FormValue("group"),
	}
	resultTemplate.Execute(w, data)
}
