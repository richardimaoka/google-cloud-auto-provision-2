package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

// resourceManagerRequests records what newFakeResourceManagerServer received.
type resourceManagerRequests struct {
	// auth holds each request's Authorization header.
	auth []string
	// displayNames holds each create request's display name.
	displayNames []string
}

// newFakeResourceManagerServer answers project creation with an operation
// that is already done and created projectID, or fails it with createStatus
// if that isn't 200.
func newFakeResourceManagerServer(t *testing.T, createStatus int, projectID string) (*httptest.Server, *resourceManagerRequests) {
	t.Helper()
	got := &resourceManagerRequests{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v3/projects", func(w http.ResponseWriter, r *http.Request) {
		got.auth = append(got.auth, r.Header.Get("Authorization"))
		var p cloudresourcemanager.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode create request: %v", err)
		}
		got.displayNames = append(got.displayNames, p.DisplayName)
		if createStatus != http.StatusOK {
			w.WriteHeader(createStatus)
			return
		}
		created, err := json.Marshal(&cloudresourcemanager.Project{ProjectId: projectID, DisplayName: p.DisplayName})
		if err != nil {
			t.Fatal(err)
		}
		json.NewEncoder(w).Encode(&cloudresourcemanager.Operation{Name: "operations/1", Done: true, Response: created})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
}

func TestResourceManagerProjects_Create(t *testing.T) {
	token := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "user-token"})
	requester, err := domain.NewEmail("requester@example.com")
	if err != nil {
		t.Fatal(err)
	}
	groupEmail, err := domain.NewEmail("developer-group@example.com")
	if err != nil {
		t.Fatal(err)
	}
	// A display name that is also a valid project ID, which the first attempt uses.
	request, err := domain.NewProjectCreationRequest("example-project", requester, domain.NewGoogleGroup(groupEmail, nil))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("利用者のtokenで、要求された表示名のプロジェクトを作成し、そのIDを返す", func(t *testing.T) {
		// The ID differs from the display name, as after a retry, so the
		// returned ID must come from the API.
		srv, requests := newFakeResourceManagerServer(t, http.StatusOK, "example-project-123456")
		projects := &resourceManagerProjects{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		got, err := projects.Create(context.Background(), token, request)
		if err != nil {
			t.Fatalf("Create() = %v", err)
		}
		if got != "example-project-123456" {
			t.Errorf("Create() = %q, want %q", got, "example-project-123456")
		}
		// It must create the requested project, once.
		if len(requests.displayNames) != 1 || requests.displayNames[0] != request.DisplayName {
			t.Errorf("created display names = %q, want [%q]", requests.displayNames, request.DisplayName)
		}
		// It must act as the user, not as the app.
		for _, a := range requests.auth {
			if a != "Bearer user-token" {
				t.Errorf("Authorization = %q, want %q", a, "Bearer user-token")
			}
		}
	})

	t.Run("プロジェクト作成が失敗したらエラーを返す", func(t *testing.T) {
		srv, _ := newFakeResourceManagerServer(t, http.StatusForbidden, "")
		projects := &resourceManagerProjects{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if _, err := projects.Create(context.Background(), token, request); err == nil {
			t.Error("Create(permission denied) = nil error, want an error")
		}
	})
}
