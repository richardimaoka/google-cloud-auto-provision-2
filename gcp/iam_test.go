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
	// Fake answer: the write worked. Its content is ignored.
	written := ok(&cloudresourcemanager.Policy{})

	t.Run("書き込みには、読んだときの etag が付く", func(t *testing.T) {
		// Without it, the API can't tell that someone else changed the policy after
		// we read it, and the write would overwrite that change.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
	t.Run("書き込みは、読んだ version に関わらず version 3 になる", func(t *testing.T) {
		// Google answers with version 1 for a policy that has no conditional binding,
		// even though we asked for version 3, so the version read says nothing about
		// the version to write with. A write with a lower version drops conditions.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
	t.Run("書き込みは bindings と etag だけに限る", func(t *testing.T) {
		// A Policy does not hold the audit configs, so a write that did not name its
		// fields could remove them from the project.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
	ownerRead := func(owners ...string) fakeIAMResponse {
		return ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1", Bindings: []*cloudresourcemanager.Binding{
			{Role: "roles/owner", Members: owners},
			{Role: "roles/viewer", Members: []string{"user:viewer@example.com"}},
		}})
	}
	t.Run("最後の owner を外す変更は、書き込まずにエラーにする", func(t *testing.T) {
		// The policy read has one owner, and update removes it (or the whole
		// binding). The error is ErrLastOwner. The fake has seen no setIamPolicy,
		// update was called once, and nothing is retried.
		removals := map[string]func(*Policy){
			"バインディングごと外す": func(p *Policy) { p.Bindings = slices.Delete(p.Bindings, 0, 1) },
			"メンバーを空にする":   func(p *Policy) { p.Bindings[0].Members = nil },
		}
		for name, remove := range removals {
			t.Run(name, func(t *testing.T) {
				srv := newFakeIAMServer(t, []fakeIAMResponse{ownerRead("user:alice@example.com")}, nil)
				client := newTestIAMClient(t, srv.URL)

				calls := 0
				err := client.updatePolicy(context.Background(), "my-project", func(p *Policy) error {
					calls++
					remove(p)
					return nil
				})
				if !errors.Is(err, ErrLastOwner) {
					t.Errorf("updatePolicy() = %v, want ErrLastOwner", err)
				}
				if calls != 1 {
					t.Errorf("update was called %d times, want 1", calls)
				}
				if reqs := srv.SetRequests(); len(reqs) != 0 {
					t.Errorf("got %d setIamPolicy requests, want none", len(reqs))
				}
				if reqs := srv.GetRequests(); len(reqs) != 1 {
					t.Errorf("got %d getIamPolicy requests, want 1", len(reqs))
				}
			})
		}
	})
	t.Run("owner が複数いて、一部を外す変更は書き込む", func(t *testing.T) {
		// Two owners, and update removes one. The write goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ownerRead("user:alice@example.com", "user:bob@example.com")},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
	t.Run("owner を別のメンバーに入れ替える変更は書き込む", func(t *testing.T) {
		// update removes the only owner and adds another in the same call. The
		// result still has an owner, so the write goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ownerRead("user:alice@example.com")},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
	t.Run("もともと owner がいないポリシーは、そのまま書き込む", func(t *testing.T) {
		// The policy read has no owner, and update changes something else. The
		// guard does not apply, and the write goes out.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{ok(&cloudresourcemanager.Policy{Version: 1, Etag: "etag-1"})},
			[]fakeIAMResponse{written}) // the write works (answer ignored)
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
			[]fakeIAMResponse{ownerRead("user:alice@example.com", "deleted:user:old@example.com?uid=123456789")}, nil)
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
	conflict := fakeIAMResponse{
		status: http.StatusConflict,
		body:   `{"error":{"code":409,"message":"There were concurrent policy changes.","status":"ABORTED"}}`,
	}
	viewerRead := func(etag string, viewers ...string) fakeIAMResponse {
		return ok(&cloudresourcemanager.Policy{Version: 1, Etag: etag, Bindings: []*cloudresourcemanager.Binding{
			{Role: "roles/viewer", Members: viewers},
		}})
	}
	t.Run("etag が競合したら読み直して update をやり直す", func(t *testing.T) {
		// The first setIamPolicy answers 409 ABORTED. The client reads again, calls
		// update again with the policy it read this time, and writes with the new
		// etag. The second write works. update was called twice, on the first and
		// on the second policy.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{viewerRead("etag-1", "user:alice@example.com"), viewerRead("etag-2", "user:alice@example.com", "user:bob@example.com")},
			[]fakeIAMResponse{conflict, written}) // the first write conflicts, then one works (answer ignored)
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
			t.Errorf("second write = %+v, want etag-2 and the 2 viewers of the second read", got)
		}
	})
	t.Run("競合が続いたら上限で諦める", func(t *testing.T) {
		// setIamPolicy answers 409 every time. The client stops at the limit of
		// attempts, the error says so, and update was called that many times.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{viewerRead("etag-1", "user:alice@example.com"), viewerRead("etag-2", "user:alice@example.com"), viewerRead("etag-3", "user:alice@example.com")},
			[]fakeIAMResponse{conflict, conflict, conflict})
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
			[]fakeIAMResponse{viewerRead("etag-1", "user:alice@example.com"), viewerRead("etag-2", "user:alice@example.com"), viewerRead("etag-3", "user:alice@example.com")},
			[]fakeIAMResponse{conflict, conflict, written}) // two writes conflict, then one works (answer ignored)
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
		// error, and does not read or write again.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{viewerRead("etag-1", "user:alice@example.com")},
			[]fakeIAMResponse{conflict})
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
		// setIamPolicy answers 403. The error comes back at once, wrapping the
		// *googleapi.Error, and there is no second read.
		srv := newFakeIAMServer(t,
			[]fakeIAMResponse{viewerRead("etag-1", "user:alice@example.com")},
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
