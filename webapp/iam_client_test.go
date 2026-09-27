package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

// iamRequests records what newFakeIAMPolicyServer received.
type iamRequests struct {
	// auth holds each request's Authorization header.
	auth []string
	// set holds each policy written with setIamPolicy.
	set []*cloudresourcemanager.Policy
}

// newFakeIAMPolicyServer serves my-project's IAM policy, starting from
// bindings, or fails getIamPolicy with getStatus if that isn't 200.
func newFakeIAMPolicyServer(t *testing.T, getStatus int, bindings []*cloudresourcemanager.Binding) (*httptest.Server, *iamRequests) {
	t.Helper()
	got := &iamRequests{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v3/projects/my-project:getIamPolicy", func(w http.ResponseWriter, r *http.Request) {
		got.auth = append(got.auth, r.Header.Get("Authorization"))
		if getStatus != http.StatusOK {
			w.WriteHeader(getStatus)
			return
		}
		json.NewEncoder(w).Encode(&cloudresourcemanager.Policy{Bindings: bindings, Etag: "etag-1"})
	})
	mux.HandleFunc("POST /v3/projects/my-project:setIamPolicy", func(w http.ResponseWriter, r *http.Request) {
		got.auth = append(got.auth, r.Header.Get("Authorization"))
		var req cloudresourcemanager.SetIamPolicyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode setIamPolicy request: %v", err)
		}
		got.set = append(got.set, req.Policy)
		json.NewEncoder(w).Encode(req.Policy)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
}

// hasMember reports whether policy grants role to member unconditionally.
func hasMember(policy *cloudresourcemanager.Policy, role, member string) bool {
	for _, b := range policy.Bindings {
		if b.Role == role && b.Condition == nil && slices.Contains(b.Members, member) {
			return true
		}
	}
	return false
}

func TestResourceManagerIAM(t *testing.T) {
	token := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "user-token"})
	saEmail, err := domain.NewEmail("automation@example-project.iam.gserviceaccount.com")
	if err != nil {
		t.Fatal(err)
	}
	principal := domain.NewServiceAccountPrincipal(saEmail)
	role, err := domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")
	if err != nil {
		t.Fatal(err)
	}
	// The project's creator is its owner, so the policy starts with one.
	owner := &cloudresourcemanager.Binding{Role: "roles/owner", Members: []string{"user:requester@example.com"}}

	t.Run("利用者のtokenで、principalの種類付きのメンバーとしてロールを付与する", func(t *testing.T) {
		srv, requests := newFakeIAMPolicyServer(t, http.StatusOK, []*cloudresourcemanager.Binding{owner})
		iam := &resourceManagerIAM{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if err := iam.AddBinding(context.Background(), token, "my-project", principal, role); err != nil {
			t.Fatalf("AddBinding() = %v", err)
		}
		// IAM identifies the member by its kind, so it must be "serviceAccount:...".
		if len(requests.set) != 1 || !hasMember(requests.set[0], role.String(), "serviceAccount:automation@example-project.iam.gserviceaccount.com") {
			t.Errorf("written policies = %v, want one granting %s to serviceAccount:%s", requests.set, role, saEmail)
		}
		// It must act as the user, not as the app.
		for _, a := range requests.auth {
			if a != "Bearer user-token" {
				t.Errorf("Authorization = %q, want %q", a, "Bearer user-token")
			}
		}
	})

	t.Run("利用者のtokenで、principalのロールを外す", func(t *testing.T) {
		granted := &cloudresourcemanager.Binding{Role: role.String(), Members: []string{principal.String()}}
		srv, requests := newFakeIAMPolicyServer(t, http.StatusOK, []*cloudresourcemanager.Binding{owner, granted})
		iam := &resourceManagerIAM{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if err := iam.RemoveBinding(context.Background(), token, "my-project", principal, role); err != nil {
			t.Fatalf("RemoveBinding() = %v", err)
		}
		if len(requests.set) != 1 || hasMember(requests.set[0], role.String(), principal.String()) {
			t.Errorf("written policies = %v, want one without %s for %s", requests.set, role, principal)
		}
		for _, a := range requests.auth {
			if a != "Bearer user-token" {
				t.Errorf("Authorization = %q, want %q", a, "Bearer user-token")
			}
		}
	})

	t.Run("principalがロールを持っていなければ、何も書かずに成功する", func(t *testing.T) {
		// Removing is done once the principal lacks the role, so a retry after
		// a partial success must not fail.
		srv, requests := newFakeIAMPolicyServer(t, http.StatusOK, []*cloudresourcemanager.Binding{owner})
		iam := &resourceManagerIAM{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if err := iam.RemoveBinding(context.Background(), token, "my-project", principal, role); err != nil {
			t.Errorf("RemoveBinding(missing binding) = %v, want nil error", err)
		}
		if len(requests.set) != 0 {
			t.Errorf("wrote %d policies, want none", len(requests.set))
		}
	})

	t.Run("権限がなければエラーを返す", func(t *testing.T) {
		srv, _ := newFakeIAMPolicyServer(t, http.StatusForbidden, nil)
		iam := &resourceManagerIAM{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if err := iam.AddBinding(context.Background(), token, "my-project", principal, role); err == nil {
			t.Error("AddBinding(permission denied) = nil error, want an error")
		}
	})
}
