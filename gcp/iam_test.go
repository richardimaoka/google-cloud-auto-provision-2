package gcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// iamRequests records what the fake server received.
type iamRequests struct {
	gets []string // bodies of the getIamPolicy requests
	sets []string // bodies of the setIamPolicy requests
}

// newFakeIAMServer fakes POST /v3/projects/my-project:getIamPolicy and
// :setIamPolicy. Each request answers the next entry of gets or of sets.
func newFakeIAMServer(t *testing.T, gets, sets []fakeIAMResponse) (*httptest.Server, *iamRequests) {
	t.Helper()

	got := &iamRequests{}
	handler := func(name string, script []fakeIAMResponse, bodies *[]string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read %s request: %v", name, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			*bodies = append(*bodies, string(body))

			i := len(*bodies) - 1
			if i >= len(script) {
				t.Errorf("unexpected %s request %d, only %d scripted", name, i+1, len(script))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			script[i].write(w)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v3/projects/my-project:getIamPolicy", handler("getIamPolicy", gets, &got.gets))
	mux.HandleFunc("POST /v3/projects/my-project:setIamPolicy", handler("setIamPolicy", sets, &got.sets))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
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
		// The fake serves POST /v3/projects/my-project:getIamPolicy only, so the
		// method and the path are checked by it answering. The response is as small
		// as possible.
		srv, got := newFakeIAMServer(t, []fakeIAMResponse{
			{status: http.StatusOK, policy: &cloudresourcemanager.Policy{Version: 1}},
		}, nil)
		client := newTestIAMClient(t, srv.URL)

		if _, err := client.GetPolicy(context.Background(), "my-project"); err != nil {
			t.Fatalf("GetPolicy() = %v, want nil error", err)
		}

		if len(got.gets) != 1 {
			t.Fatalf("got %d getIamPolicy requests, want 1", len(got.gets))
		}
		var req cloudresourcemanager.GetIamPolicyRequest
		if err := json.Unmarshal([]byte(got.gets[0]), &req); err != nil {
			t.Fatalf("decode request %q: %v", got.gets[0], err)
		}
		if req.Options == nil || req.Options.RequestedPolicyVersion != 3 {
			t.Errorf("request options = %+v, want requestedPolicyVersion 3", req.Options)
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
	t.Run("読んだ etag とバージョン3を書き込みに付ける", func(t *testing.T) {
		// The setIamPolicy body has the etag that getIamPolicy returned, and
		// version 3, whatever version was read.
		t.Skip("not implemented")
	})
	t.Run("update の変更が送られる", func(t *testing.T) {
		// A binding added, a member removed and a binding removed by update are in
		// the body. A Condition of the result goes out with its title, description
		// and expression. Nothing in the client sets one, but the integration test
		// does, through updatePolicy.
		t.Skip("not implemented")
	})
	t.Run("変更がなければ書き込まない", func(t *testing.T) {
		// update leaves the policy equal to the one read, so there is no
		// setIamPolicy, and the etag is not spent on a write that changes nothing.
		t.Skip("not implemented")
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
		t.Skip("not implemented")
	})
	t.Run("読み取りに失敗したら update も書き込みもしない", func(t *testing.T) {
		// getIamPolicy answers 500. update was not called, and there was no
		// setIamPolicy.
		t.Skip("not implemented")
	})
}
