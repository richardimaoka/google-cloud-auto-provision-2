package webapp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// TestFormTemplate checks that form.html executes without error and has the
// fields handleSubmit's r.FormValue calls depend on.
func TestFormTemplate(t *testing.T) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "form.html", nil); err != nil {
		t.Fatalf("ExecuteTemplate(form.html): %v", err)
	}
	body := buf.String()
	for _, want := range []string{`name="display_name"`, `name="email"`, `name="group"`} {
		if !strings.Contains(body, want) {
			t.Errorf("form.html doesn't have a field %s", want)
		}
	}
}

// TestResultTemplate checks that result.html executes without error against
// the real resultData type, and that each value lands next to its own
// label — not just anywhere in the output, which would miss two values
// swapped between fields. If result.html is typo'd to reference a field
// resultData doesn't have, or a future change to resultData makes some
// field's value fail to execute, ExecuteTemplate returns an error and this
// test fails; that's the safety net replacing what compile-time checking
// gave the old inline string-literal templates.
func TestResultTemplate(t *testing.T) {
	data := resultData{
		ID:          domain.ProjectCreationID("request-123"),
		DisplayName: "Example Project",
		Email:       "requester@example.com",
		Group:       "developer-group@example.com",
	}

	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "result.html", data); err != nil {
		t.Fatalf("ExecuteTemplate(result.html): %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		"Request ID: " + string(data.ID),
		"Project display name: " + data.DisplayName,
		"Your email: " + data.Email,
		"Google group email: " + data.Group,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("result.html output doesn't contain %q:\n%s", want, body)
		}
	}
}

// TestRenderTemplate_Error checks that renderTemplate turns a template
// execution failure into a clean 500, instead of writing a partial,
// silently-truncated 200 response — the failure mode this whole design is
// meant to avoid (see renderTemplate's doc comment).
func TestRenderTemplate_Error(t *testing.T) {
	w := httptest.NewRecorder()
	renderTemplate(w, "does-not-exist.html", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if w.Body.Len() == 0 {
		t.Error("no error body written")
	}
}
