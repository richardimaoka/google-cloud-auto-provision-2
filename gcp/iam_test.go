package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Unit tests for IAMClient.
//
// They cover failures we can't cause on demand (a conflict on the etag), our own
// logic (the read-modify-write loop and the last-owner guard), and what the client
// sends (policy version, etag, conditions).
//
// Every case below is skipped until it is written.

// fakeIAMResponse is one scripted answer of the fake server: an error status with
// an optional body, or status 200 with policy as the body. The policies follow
// the shapes in samples/data/projects.getIamPolicy.*.json.
type fakeIAMResponse struct {
	status int
	body   string
	policy *cloudresourcemanager.Policy
}

func (r fakeIAMResponse) write(w http.ResponseWriter) {
	if r.status != http.StatusOK {
		w.WriteHeader(r.status)
		io.WriteString(w, r.body)
		return
	}
	json.NewEncoder(w).Encode(r.policy)
}

// ok is a response with status 200 and the policy p.
func ok(p *cloudresourcemanager.Policy) fakeIAMResponse {
	return fakeIAMResponse{status: http.StatusOK, policy: p}
}

// setOK is a set response that worked. Its content is ignored.
var setOK = ok(&cloudresourcemanager.Policy{})

// checkSetRequestBindings checks the bindings of the one set request the fake got.
func checkSetRequestBindings(t *testing.T, srv *fakeIAMServer, want []*cloudresourcemanager.Binding) {
	t.Helper()

	reqs := srv.SetRequests()
	if len(reqs) != 1 {
		t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
	}
	if got := reqs[0].Policy.Bindings; !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Errorf("set request bindings = %s, want %s", gotJSON, wantJSON)
	}
}

// fakeIAMServer fakes POST /v3/projects/my-project:getIamPolicy and
// :setIamPolicy, and records what it receives. Any other method or path is not
// served, so a client that uses one fails.
type fakeIAMServer struct {
	*httptest.Server

	mu      sync.Mutex
	getReqs []cloudresourcemanager.GetIamPolicyRequest
	setReqs []cloudresourcemanager.SetIamPolicyRequest
}

// newFakeIAMServer starts a fakeIAMServer. The responses are given in order: the
// first getIamPolicy request gets getResponses[0], the second getResponses[1], and
// so on, and setIamPolicy requests get setResponses the same way. A request past
// the last response fails the test.
func newFakeIAMServer(t *testing.T, getResponses, setResponses []fakeIAMResponse) *fakeIAMServer {
	t.Helper()

	s := &fakeIAMServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v3/projects/my-project:getIamPolicy", newIAMHandler(t, &s.mu, "getIamPolicy", getResponses, &s.getReqs))
	mux.HandleFunc("POST /v3/projects/my-project:setIamPolicy", newIAMHandler(t, &s.mu, "setIamPolicy", setResponses, &s.setReqs))

	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// GetRequests returns the getIamPolicy requests received so far, oldest first.
func (s *fakeIAMServer) GetRequests() []cloudresourcemanager.GetIamPolicyRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.getReqs)
}

// SetRequests returns the setIamPolicy requests received so far, oldest first.
func (s *fakeIAMServer) SetRequests() []cloudresourcemanager.SetIamPolicyRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.setReqs)
}

// newIAMHandler records the body of each request in requests, and answers it with
// the next entry of responses. mu guards requests.
func newIAMHandler[T any](t *testing.T, mu *sync.Mutex, name string, responses []fakeIAMResponse, requests *[]T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode %s request: %v", name, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		*requests = append(*requests, req)
		i := len(*requests) - 1
		mu.Unlock()

		if i >= len(responses) {
			t.Errorf("unexpected %s request %d, only %d scripted", name, i+1, len(responses))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		responses[i].write(w)
	}
}

func newTestIAMClient(t *testing.T, endpoint string) *IAMClient {
	t.Helper()

	client, err := NewIAMClient(context.Background(), option.WithEndpoint(endpoint), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewIAMClient: %v", err)
	}
	client.retryWait = time.Millisecond
	return client
}

func TestIAMClient_GetPolicy(t *testing.T) {
	t.Run("バージョン3を要求する", func(t *testing.T) {
		// Without version 3, the API leaves the conditions out of the policy.
		srv := newFakeIAMServer(t, []fakeIAMResponse{
			{status: http.StatusOK, policy: &cloudresourcemanager.Policy{}},
		}, nil)
		client := newTestIAMClient(t, srv.URL)

		if _, err := client.GetPolicy(context.Background(), "my-project"); err != nil {
			t.Fatalf("GetPolicy() = %v, want nil error", err)
		}

		reqs := srv.GetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d getIamPolicy requests, want 1", len(reqs))
		}
		if opts := reqs[0].Options; opts == nil || opts.RequestedPolicyVersion != 3 {
			t.Errorf("request options = %+v, want requestedPolicyVersion 3", opts)
		}
	})
}

func TestIAMClient_AddBinding(t *testing.T) {
	condition := &cloudresourcemanager.Expr{
		Title:       "condition title",
		Description: "condition description",
		Expression:  `request.time < timestamp("2030-01-01T00:00:00Z")`,
	}

	t.Run("ロールがなければ、set request に新しいバインディングが入る", func(t *testing.T) {
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.AddBinding(context.Background(), "my-project", "roles/editor", "user:bob@example.com"); err != nil {
			t.Fatalf("AddBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			{Role: "roles/editor", Members: []string{"user:bob@example.com"}},
		})
	})
	t.Run("ロールに条件なしのバインディングがあれば、そこにメンバーを足す", func(t *testing.T) {
		// The role has a conditional binding too, listed first, which stays as it was.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:dave@example.com"}, Condition: condition},
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.AddBinding(context.Background(), "my-project", "roles/viewer", "user:bob@example.com"); err != nil {
			t.Fatalf("AddBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:dave@example.com"}, Condition: condition},
			{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
		})
	})
	t.Run("ロールに条件付きのバインディングしかなければ、条件なしを新しく作る", func(t *testing.T) {
		// Putting the member into the conditional binding would grant the role only
		// while the condition holds. That is so even for a member that is in it.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}, Condition: condition},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.AddBinding(context.Background(), "my-project", "roles/viewer", "user:alice@example.com"); err != nil {
			t.Fatalf("AddBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}, Condition: condition},
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
		})
	})
	t.Run("すでにあれば、set request を送らずに成功する", func(t *testing.T) {
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})}, nil)
		client := newTestIAMClient(t, srv.URL)

		if err := client.AddBinding(context.Background(), "my-project", "roles/viewer", "user:alice@example.com"); err != nil {
			t.Fatalf("AddBinding() = %v, want nil error", err)
		}

		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
}

func TestIAMClient_RemoveBinding(t *testing.T) {
	condition := &cloudresourcemanager.Expr{
		Title:       "condition title",
		Description: "condition description",
		Expression:  `request.time < timestamp("2030-01-01T00:00:00Z")`,
	}

	t.Run("メンバーを外した set request を送る", func(t *testing.T) {
		// The other members of the binding, and the other bindings, stay.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
				{Role: "roles/editor", Members: []string{"user:carol@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.RemoveBinding(context.Background(), "my-project", "roles/viewer", "user:bob@example.com"); err != nil {
			t.Fatalf("RemoveBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			{Role: "roles/editor", Members: []string{"user:carol@example.com"}},
		})
	})
	t.Run("最後のメンバーを外すと、バインディングごと消える", func(t *testing.T) {
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
				{Role: "roles/editor", Members: []string{"user:carol@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.RemoveBinding(context.Background(), "my-project", "roles/viewer", "user:alice@example.com"); err != nil {
			t.Fatalf("RemoveBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/editor", Members: []string{"user:carol@example.com"}},
		})
	})
	t.Run("条件付きバインディングのメンバーは外さない", func(t *testing.T) {
		// The member is in a conditional binding, listed first, and in the
		// unconditional one. Only the unconditional one loses it.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}, Condition: condition},
				{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.RemoveBinding(context.Background(), "my-project", "roles/viewer", "user:alice@example.com"); err != nil {
			t.Fatalf("RemoveBinding() = %v, want nil error", err)
		}

		checkSetRequestBindings(t, srv, []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}, Condition: condition},
			{Role: "roles/viewer", Members: []string{"user:bob@example.com"}},
		})
	})
	t.Run("見つからなければ ErrBindingNotFound で、set request を送らない", func(t *testing.T) {
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.RemoveBinding(context.Background(), "my-project", "roles/viewer", "user:bob@example.com")

		if !errors.Is(err, ErrBindingNotFound) {
			t.Errorf("RemoveBinding() = %v, want ErrBindingNotFound", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
	t.Run("条件付きバインディングにしかいないメンバーは、見つからない扱い", func(t *testing.T) {
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}, Condition: condition},
			}})}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.RemoveBinding(context.Background(), "my-project", "roles/viewer", "user:alice@example.com")

		if !errors.Is(err, ErrBindingNotFound) {
			t.Errorf("RemoveBinding() = %v, want ErrBindingNotFound", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
	t.Run("最後の owner を外すと ErrLastOwner", func(t *testing.T) {
		// The guard of updatePolicy reaches the caller.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/owner", Members: []string{"user:alice@example.com"}},
				{Role: "roles/viewer", Members: []string{"user:bob@example.com"}},
			}})}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.RemoveBinding(context.Background(), "my-project", "roles/owner", "user:alice@example.com")

		if !errors.Is(err, ErrLastOwner) {
			t.Errorf("RemoveBinding() = %v, want ErrLastOwner", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
}

func TestIAMClient_updatePolicy(t *testing.T) {
	// addViewer changes the policy, so that updatePolicy has a change to set.
	addViewer := func(p *Policy) error {
		p.Bindings = append(p.Bindings, Binding{Role: "roles/viewer", Members: []string{"user:alice@example.com"}})
		return nil
	}

	t.Run("set request には、get response の etag が付く", func(t *testing.T) {
		// Without it, the API can't tell that someone else changed the policy after
		// our get request, and the set request would overwrite that change.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		reqs := srv.SetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
		}
		if p := reqs[0].Policy; p == nil || p.Etag != "etag-1" {
			t.Errorf("request policy = %+v, want etag %q", p, "etag-1")
		}
	})
	t.Run("set request の version は、get response の version に関わらず 3 になる", func(t *testing.T) {
		// Google answers version 1 for a policy without conditions, even if we ask
		// for 3. So always set version 3, or conditions get dropped.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		reqs := srv.SetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
		}
		if p := reqs[0].Policy; p == nil || p.Version != 3 {
			t.Errorf("request policy = %+v, want version 3", p)
		}
	})
	t.Run("set request の updateMask は bindings と etag だけ", func(t *testing.T) {
		// Policy has no audit-log settings (audit configs). The mask keeps the set
		// request from touching them.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		reqs := srv.SetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
		}
		if reqs[0].UpdateMask != "bindings,etag" {
			t.Errorf("request updateMask = %q, want %q", reqs[0].UpdateMask, "bindings,etag")
		}
	})
	t.Run("update が触れなかった条件付きバインディングは、条件ごと set request に入る", func(t *testing.T) {
		// Without its condition, the role would be granted all the time.
		condition := &cloudresourcemanager.Expr{
			Title:       "condition title",
			Description: "condition description",
			Expression:  `request.time < timestamp("2030-01-01T00:00:00Z")`,
		}
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
				{Role: "roles/bigquery.jobUser", Members: []string{"user:dave@example.com"}, Condition: condition},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members = append(p.Bindings[0].Members, "user:bob@example.com")
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		want := []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
			{Role: "roles/bigquery.jobUser", Members: []string{"user:dave@example.com"}, Condition: condition},
		}
		checkSetRequestBindings(t, srv, want)
	})
	t.Run("変更がなければ set request を送らない", func(t *testing.T) {
		// update leaves the policy as the get response had it, so no set request is
		// sent, and the etag is not spent on a set that changes nothing.
		srv := newFakeIAMServer(t, []fakeIAMResponse{ok(&cloudresourcemanager.Policy{
			Version: 1, Etag: "etag-1",
			Bindings: []*cloudresourcemanager.Binding{{Role: "roles/viewer", Members: []string{"user:alice@example.com"}}},
		})}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			// The same members, made again, are not a change.
			p.Bindings[0].Members = append([]string(nil), p.Bindings[0].Members...)
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
	t.Run("Members の要素を直接書き換えても、変更として set request が送られる", func(t *testing.T) {
		// Without cloning, both policies share the Members array. A change in place
		// would then alter the policy compared with too, and look like no change.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members[0] = "user:bob@example.com"
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 1 {
			t.Errorf("got %d setIamPolicy requests, want 1", len(reqs))
		}
	})

	// The guard.
	getWithOwners := func(owners ...string) fakeIAMResponse {
		return ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
			{Role: "roles/owner", Members: owners},
		}})
	}
	t.Run("最後の owner を外す変更は、set request を送らずにエラーにする", func(t *testing.T) {
		srv := newFakeIAMServer(t, []fakeIAMResponse{getWithOwners("user:alice@example.com")}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings = nil // the only owner goes
			return nil
		})

		if !errors.Is(err, ErrLastOwner) {
			t.Errorf("updatePolicy() = %v, want ErrLastOwner", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
	t.Run("owner が複数いて、一部を外す変更は set request を送る", func(t *testing.T) {
		// Two owners, and update removes one. The set request goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{getWithOwners("user:alice@example.com", "user:bob@example.com")},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members = []string{"user:alice@example.com"}
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 1 {
			t.Errorf("got %d setIamPolicy requests, want 1", len(reqs))
		}
	})
	t.Run("owner を別のメンバーに入れ替える変更は set request を送る", func(t *testing.T) {
		// update removes the only owner and adds another in the same call. The
		// result still has an owner, so the set request goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{getWithOwners("user:alice@example.com")},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members = []string{"user:bob@example.com"}
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 1 {
			t.Errorf("got %d setIamPolicy requests, want 1", len(reqs))
		}
	})
	t.Run("もともと owner がいないポリシーは、そのまま set request を送る", func(t *testing.T) {
		// The get response has no owner, and update changes something else. The
		// guard does not apply, and the set request goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{setOK}) // answer ignored
		client := newTestIAMClient(t, srv.URL)

		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 1 {
			t.Errorf("got %d setIamPolicy requests, want 1", len(reqs))
		}
	})
	t.Run("deleted: のメンバーは owner に数えない", func(t *testing.T) {
		// A deleted member is left in roles/owner and grants nothing. update removes
		// the other owner, so no owner that counts is left, and it is ErrLastOwner.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{getWithOwners("user:alice@example.com", "deleted:user:old@example.com?uid=123456789")}, nil)
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members = []string{"deleted:user:old@example.com?uid=123456789"}
			return nil
		})
		if !errors.Is(err, ErrLastOwner) {
			t.Errorf("updatePolicy() = %v, want ErrLastOwner", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})

	// Conflicts and errors.
	setConflict := fakeIAMResponse{
		status: http.StatusConflict,
		body:   `{"error":{"code":409,"message":"There were concurrent policy changes.","status":"ABORTED"}}`,
	}
	getWithViewers := func(etag string, viewers ...string) fakeIAMResponse {
		return ok(&cloudresourcemanager.Policy{Version: 1, Etag: etag, Bindings: []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: viewers},
		}})
	}
	t.Run("etag が競合したら get し直して update をやり直す", func(t *testing.T) {
		// The second get response has a new etag and one more viewer. update must see
		// that viewer, and the second set request must carry the new etag.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{
				getWithViewers("etag-1", "user:alice@example.com"),                         // 1st get response
				getWithViewers("etag-2", "user:alice@example.com", "user:bob@example.com"), // 2nd get response
			},
			[]fakeIAMResponse{
				setConflict, // 1st set response
				setOK,       // 2nd set response (answer ignored)
			})
		client := newTestIAMClient(t, srv.URL)

		var viewersSeen []int
		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			viewersSeen = append(viewersSeen, len(p.Bindings[0].Members))
			p.Bindings = append(p.Bindings, Binding{Role: "roles/browser", Members: []string{"user:erin@example.com"}})
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		if !slices.Equal(viewersSeen, []int{1, 2}) {
			t.Errorf("update saw policies with %v viewers, want [1 2]", viewersSeen)
		}
		reqs := srv.SetRequests()
		if len(reqs) != 2 {
			t.Fatalf("got %d setIamPolicy requests, want 2", len(reqs))
		}
		if got := reqs[1].Policy; got.Etag != "etag-2" || len(got.Bindings[0].Members) != 2 {
			t.Errorf("second set request = %+v, want etag-2 and the 2 viewers of the second get response", got)
		}
	})
	t.Run("競合が続いたら上限で諦める", func(t *testing.T) {
		// Every set response is 409. The client stops at the limit of attempts, the
		// error says so, and update was called that many times.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{
				getWithViewers("etag-1", "user:alice@example.com"), // 1st get response
				getWithViewers("etag-2", "user:alice@example.com"), // 2nd get response
				getWithViewers("etag-3", "user:alice@example.com"), // 3rd get response
			},
			[]fakeIAMResponse{
				setConflict, // 1st set response
				setConflict, // 2nd set response
				setConflict, // 3rd set response
			})
		client := newTestIAMClient(t, srv.URL)

		calls := 0
		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			calls++
			p.Bindings = append(p.Bindings, Binding{Role: "roles/browser", Members: []string{"user:erin@example.com"}})
			return nil
		})

		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusConflict {
			t.Errorf("updatePolicy() = %v, want an error that wraps the 409", err)
		}
		if err == nil || !strings.Contains(err.Error(), "3 attempts") {
			t.Errorf("updatePolicy() = %v, want an error that says 3 attempts", err)
		}
		if calls != 3 {
			t.Errorf("update was called %d times, want 3", calls)
		}
		if reqs := srv.SetRequests(); len(reqs) != 3 {
			t.Errorf("got %d setIamPolicy requests, want 3", len(reqs))
		}
	})
	t.Run("再試行の前に待つ", func(t *testing.T) {
		// After each conflict the client waits before it reads again. The wait is
		// the same each time. The test checks the total from below, so it can't fail
		// for being slow.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{
				getWithViewers("etag-1", "user:alice@example.com"), // 1st get response
				getWithViewers("etag-2", "user:alice@example.com"), // 2nd get response
				getWithViewers("etag-3", "user:alice@example.com"), // 3rd get response
			},
			[]fakeIAMResponse{
				setConflict, // 1st set response
				setConflict, // 2nd set response
				setOK,       // 3rd set response (answer ignored)
			})
		client := newTestIAMClient(t, srv.URL)
		client.retryWait = 30 * time.Millisecond

		start := time.Now()
		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}
		if elapsed := time.Since(start); elapsed < 2*client.retryWait {
			t.Errorf("took %v for 2 retries, want at least %v", elapsed, 2*client.retryWait)
		}
	})
	t.Run("待っている間にコンテキストがキャンセルされたら戻る", func(t *testing.T) {
		// Cancelled during a wait, the client returns at once with the context
		// error, and sends no more get or set requests.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{getWithViewers("etag-1", "user:alice@example.com")},
			[]fakeIAMResponse{setConflict})
		client := newTestIAMClient(t, srv.URL)
		client.retryWait = time.Hour

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			for len(srv.SetRequests()) == 0 {
				time.Sleep(time.Millisecond)
			}
			cancel()
		}()

		start := time.Now()
		err := client.updatePolicy(ctx, "my-project", addViewer)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("updatePolicy() = %v, want context.Canceled", err)
		}
		if elapsed := time.Since(start); elapsed > 30*time.Second {
			t.Errorf("took %v, want it to return at once", elapsed)
		}
		if n := len(srv.GetRequests()); n != 1 {
			t.Errorf("got %d getIamPolicy requests, want 1", n)
		}
		if n := len(srv.SetRequests()); n != 1 {
			t.Errorf("got %d setIamPolicy requests, want 1", n)
		}
	})
	t.Run("競合以外のエラーは再試行しない", func(t *testing.T) {
		// The set response is 403. The error comes back at once, wrapping the
		// *googleapi.Error, and there is no second get request.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{getWithViewers("etag-1", "user:alice@example.com")},
			[]fakeIAMResponse{{status: http.StatusForbidden, body: `{"error":{"code":403,"status":"PERMISSION_DENIED"}}`}})
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", addViewer)

		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusForbidden {
			t.Errorf("updatePolicy() = %v, want an error that wraps the 403", err)
		}
		if n := len(srv.GetRequests()); n != 1 {
			t.Errorf("got %d getIamPolicy requests, want 1", n)
		}
	})
	t.Run("update がエラーを返したら set request を送らない", func(t *testing.T) {
		// The error of update comes back, and the fake has seen no set request.
		srv := newFakeIAMServer(t, []fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})}, nil)
		client := newTestIAMClient(t, srv.URL)
		boom := errors.New("boom")

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings = append(p.Bindings, Binding{Role: "roles/viewer", Members: []string{"user:alice@example.com"}})
			return boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("updatePolicy() = %v, want the error of update", err)
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
	t.Run("get request が失敗したら update も set request もしない", func(t *testing.T) {
		// The get response is 500. update was not called, and there was no set
		// request.
		srv := newFakeIAMServer(t, []fakeIAMResponse{{status: http.StatusInternalServerError}}, nil)
		client := newTestIAMClient(t, srv.URL)

		called := false
		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			called = true
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "get iam policy") {
			t.Errorf("updatePolicy() = %v, want an error that says get iam policy", err)
		}
		if called {
			t.Error("update was called, want it not to be")
		}
		if reqs := srv.SetRequests(); len(reqs) != 0 {
			t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
		}
	})
}
