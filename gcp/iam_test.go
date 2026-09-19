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

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
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

// fakeIAMServer fakes POST /v3/projects/my-project:getIamPolicy and
// :setIamPolicy, and records what it receives. Any other method or path is not
// served, so a client that uses one fails.
type fakeIAMServer struct {
	*httptest.Server

	mu      sync.Mutex
	getReqs []cloudresourcemanager.GetIamPolicyRequest
	setReqs []cloudresourcemanager.SetIamPolicyRequest
}

// newFakeIAMServer starts a fakeIAMServer. Each getIamPolicy request answers the
// next entry of getResponses, and each setIamPolicy request the next entry of
// setResponses.
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
	// TODO: shorten the wait between retries, like pollInterval in
	// newTestProjectsClient.
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
	t.Run("ロールがなければ新しいバインディングを書き込む", func(t *testing.T) {
		// The setIamPolicy body has a binding of the role with the member, and the
		// rest of the policy as it was.
		t.Skip("not implemented")
	})
	t.Run("ロールに条件なしのバインディングがあれば、そこにメンバーを足す", func(t *testing.T) {
		// The number of bindings does not grow, and the member is the last of the
		// binding. The policy read has a conditional binding too, like
		// with_condition.json, and the body still has it with its title,
		// description and expression. A write that dropped a condition would change
		// who has access.
		t.Skip("not implemented")
	})
	t.Run("ロールに条件付きのバインディングしかなければ、条件なしを新しく作る", func(t *testing.T) {
		// The member is not put into the conditional binding, which would give it
		// the role only while the condition holds. A new unconditional binding is
		// made, and the conditional one goes back as it was.
		t.Skip("not implemented")
	})
	t.Run("すでにあれば書き込まずに成功する", func(t *testing.T) {
		// No error, and the fake has seen no setIamPolicy.
		t.Skip("not implemented")
	})
}

func TestIAMClient_RemoveBinding(t *testing.T) {
	t.Run("メンバーを外して書き込む", func(t *testing.T) {
		// The other members of the binding, and the other bindings, stay.
		t.Skip("not implemented")
	})
	t.Run("最後のメンバーを外すとバインディングごと消える", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("条件付きバインディングのメンバーは外さない", func(t *testing.T) {
		// The member is in the unconditional binding of the role and in a
		// conditional one. It is removed from the first only, and the conditional
		// binding goes back with its condition.
		t.Skip("not implemented")
	})
	t.Run("見つからなければ ErrBindingNotFound で、書き込まない", func(t *testing.T) {
		// errors.Is finds ErrBindingNotFound, and the fake has seen no
		// setIamPolicy. A member that is only in a conditional binding of the role
		// counts as not found.
		t.Skip("not implemented")
	})
	t.Run("最後の owner を外すと ErrLastOwner", func(t *testing.T) {
		// The guard of updatePolicy reaches the caller. The cases of the guard
		// are in TestIAMClient_updatePolicy.
		t.Skip("not implemented")
	})
}

func TestIAMClient_updatePolicy(t *testing.T) {
	// addViewer changes the policy, so that updatePolicy has something to write.
	addViewer := func(p *Policy) error {
		p.Bindings = append(p.Bindings, Binding{Role: "roles/viewer", Members: []string{"user:alice@example.com"}})
		return nil
	}
	ok := func(p *cloudresourcemanager.Policy) fakeIAMResponse {
		return fakeIAMResponse{status: http.StatusOK, policy: p}
	}

	t.Run("読んだ etag とバージョン3を書き込みに付ける", func(t *testing.T) {
		// The setIamPolicy body has the etag that getIamPolicy returned, and
		// version 3, whatever version was read.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{})})
		client := newTestIAMClient(t, srv.URL)

		if err := client.updatePolicy(context.Background(), "my-project", addViewer); err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		reqs := srv.SetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
		}
		if p := reqs[0].Policy; p == nil || p.Etag != "etag-1" || p.Version != 3 {
			t.Errorf("request policy = %+v, want etag %q and version 3", p, "etag-1")
		}
	})
	t.Run("書き込みは bindings と etag だけに限る", func(t *testing.T) {
		// A Policy does not hold the audit configs, so a write that did not name its
		// fields could remove them from the project.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{})})
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
	t.Run("update の変更が送られる", func(t *testing.T) {
		// A member removed, a binding removed and a binding added by update are in the
		// body, and a Condition goes out with its title, description and expression.
		condition := &cloudresourcemanager.Expr{
			Title:       "condition title",
			Description: "condition description",
			Expression:  `request.time < timestamp("2030-01-01T00:00:00Z")`,
		}
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 3, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
				{Role: "roles/editor", Members: []string{"user:carol@example.com"}},
				{Role: "roles/bigquery.jobUser", Members: []string{"user:dave@example.com"}, Condition: condition},
			}})},
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{})})
		client := newTestIAMClient(t, srv.URL)

		err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
			p.Bindings[0].Members = []string{"user:alice@example.com"}
			p.Bindings = slices.Delete(p.Bindings, 1, 2)
			p.Bindings = append(p.Bindings, Binding{Role: "roles/browser", Members: []string{"user:erin@example.com"}})
			return nil
		})
		if err != nil {
			t.Fatalf("updatePolicy() = %v, want nil error", err)
		}

		reqs := srv.SetRequests()
		if len(reqs) != 1 {
			t.Fatalf("got %d setIamPolicy requests, want 1", len(reqs))
		}
		want := []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			{Role: "roles/bigquery.jobUser", Members: []string{"user:dave@example.com"}, Condition: condition},
			{Role: "roles/browser", Members: []string{"user:erin@example.com"}},
		}
		if got := reqs[0].Policy.Bindings; !reflect.DeepEqual(got, want) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			t.Errorf("request bindings = %s, want %s", gotJSON, wantJSON)
		}
	})
	t.Run("変更がなければ書き込まない", func(t *testing.T) {
		// update leaves the policy equal to the one read, so there is no
		// setIamPolicy, and the etag is not spent on a write that changes nothing.
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
	t.Run("メンバーをその場で書き換える変更も書き込まれる", func(t *testing.T) {
		// update changes an element of Members itself. The policy it is compared with
		// must not change along with it, or the write would be left out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
				{Role: "roles/viewer", Members: []string{"user:alice@example.com"}},
			}})},
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{})})
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
	t.Run("最後の owner を外す変更は、書き込まずにエラーにする", func(t *testing.T) {
		// The policy read has one owner, and update removes it (or the whole
		// binding). The error is ErrLastOwner. The fake has seen no setIamPolicy,
		// update was called once, and nothing is retried.
		t.Skip("not implemented")
	})
	t.Run("owner が複数いて、一部を外す変更は書き込む", func(t *testing.T) {
		// Two owners, and update removes one. The write goes out.
		t.Skip("not implemented")
	})
	t.Run("owner を別のメンバーに入れ替える変更は書き込む", func(t *testing.T) {
		// update removes the only owner and adds another in the same call. The
		// result still has an owner, so the write goes out.
		t.Skip("not implemented")
	})
	t.Run("もともと owner がいないポリシーは、そのまま書き込む", func(t *testing.T) {
		// The policy read has no owner, and update changes something else. The
		// guard does not apply, and the write goes out.
		t.Skip("not implemented")
	})
	t.Run("deleted: のメンバーは owner に数えない", func(t *testing.T) {
		// The only member of roles/owner is "deleted:user:...", and update
		// removes the other owner: the result has no owner that counts, so it is
		// ErrLastOwner. To be decided, together with what counts as an owner.
		t.Skip("not implemented")
	})

	// Conflicts and errors.
	t.Run("etag が競合したら読み直して update をやり直す", func(t *testing.T) {
		// The first setIamPolicy answers 409 ABORTED. The client reads again, calls
		// update again with the policy it read this time, and writes with the new
		// etag. The second write works. update was called twice, on the first and
		// on the second policy.
		t.Skip("not implemented")
	})
	t.Run("競合が続いたら上限で諦める", func(t *testing.T) {
		// setIamPolicy answers 409 every time. The client stops at the limit of
		// attempts, the error says so, and update was called that many times.
		t.Skip("not implemented")
	})
	t.Run("再試行の間隔は増えていく", func(t *testing.T) {
		// The waits the client asked for, recorded through the replaced wait, get
		// longer with each conflict. The test does not sleep.
		t.Skip("not implemented")
	})
	t.Run("待っている間にコンテキストがキャンセルされたら戻る", func(t *testing.T) {
		// Cancelled during a wait, the client returns at once with the context
		// error, and does not read or write again.
		t.Skip("not implemented")
	})
	t.Run("競合以外のエラーは再試行しない", func(t *testing.T) {
		// setIamPolicy answers 403. The error comes back at once, wrapping the
		// *googleapi.Error, and there is no second read.
		t.Skip("not implemented")
	})
	t.Run("update がエラーを返したら書き込まない", func(t *testing.T) {
		// The error of update comes back, and the fake has seen no setIamPolicy.
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
	t.Run("読み取りに失敗したら update も書き込みもしない", func(t *testing.T) {
		// getIamPolicy answers 500. update was not called, and there was no
		// setIamPolicy.
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
