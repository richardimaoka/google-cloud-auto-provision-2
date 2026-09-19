package gcp

import "testing"

// What the unit tests of IAMClient are for.
//
// The integration test (iam_integration_test.go) runs against a real project. It
// is what shows how the API really behaves: that a policy survives being set and
// read back, that a condition is read with policy version 3, and what a 403
// looks like. So the unit tests do not repeat any of that, and do not check that
// a response we made up turns into the struct we expect. They cover what a real
// project can't give us:
//
//   - Failures we can't cause on demand: a conflict on the etag (409), and the
//     waits between retries.
//   - Logic that is our own: the read-modify-write loop of UpdatePolicy, and its
//     guard against removing the last owner of a project.
//   - What the client sends, when a wrong request would damage a real policy: the
//     policy version, the etag, and the conditions that must go back unchanged.
//
// They use a fake httptest server, like projects_test.go, that serves
//
//	POST /v3/projects/<id>:getIamPolicy
//	POST /v3/projects/<id>:setIamPolicy
//
// It remembers every request (method, path and body), and answers each one from
// a script for the test case: a policy, or a status. Responses are built from
// cloudresourcemanager types. The shapes follow the samples in samples/data/,
// which are only for reference:
//
//	projects.getIamPolicy.success.no_condition.json
//	projects.getIamPolicy.success.with_condition.json
//
// The client keeps its wait between retries in a field, like pollInterval in
// ProjectsClient, so a test replaces it and never sleeps.
//
// UpdatePolicy(ctx, projectID, update func(*Policy) error) error is the name
// used here for the read-modify-write operation. The etag is kept inside it, so
// Policy has no Etag.
//
// The guard: a project whose policy has no owner can't be changed by anyone who
// gets access only through it. So UpdatePolicy refuses a change that leaves a
// project with no owner, when the policy it read had one. An owner is a member of
// an unconditional roles/owner binding, and a "deleted:" member is not one, as it
// grants nothing. The error is ErrLastOwner (a provisional name), found with
// errors.Is, and nothing is written or retried. A policy that had no owner to
// begin with, for example because the owners come from the organization, is
// written as it is.
//
// The clients take a project ID, or a number, and do not check it, like
// ProjectsClient. Only projects are handled: nothing here is for folders or
// organizations.
//
// Every case below is skipped until it is written.

func TestIAMClient_GetPolicy(t *testing.T) {
	t.Run("バージョン3を要求する", func(t *testing.T) {
		// The method is POST, the path is /v3/projects/<id>:getIamPolicy, and the
		// body is {"options":{"requestedPolicyVersion":3}}, so a condition is never
		// left out. The response can be as small as possible.
		t.Skip("not implemented")
	})
}

func TestIAMClient_UpdatePolicy(t *testing.T) {
	t.Run("読んだ etag とバージョン3を書き込みに付ける", func(t *testing.T) {
		// The setIamPolicy body has the etag that getIamPolicy returned, and
		// version 3, whatever version was read.
		t.Skip("not implemented")
	})
	t.Run("変更していない条件付きバインディングは、条件ごとそのまま送り返す", func(t *testing.T) {
		// The policy read has an unconditional binding and a conditional one, like
		// with_condition.json. update adds a member to the first only. The body
		// still has the conditional binding, with its title, description and
		// expression, and nothing else changed. A write that dropped a condition
		// would change who has access.
		t.Skip("not implemented")
	})
	t.Run("update の変更が送られる", func(t *testing.T) {
		// A binding added, a member removed and a binding removed by update are in
		// the body. A Condition of the result goes out with its three fields.
		t.Skip("not implemented")
	})
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
	t.Run("変更がなければ書き込まない", func(t *testing.T) {
		// update leaves the policy as it was, so there is no setIamPolicy. To be
		// decided: it avoids a write that changes nothing, and the etag with it.
		t.Skip("not implemented")
	})
}

// If Policy gets methods of its own, like AddMember or RemoveMember, their tests
// go here, as plain tests on a Policy with no server: a member is added to the
// binding that has the same role and the same condition, and to no other; a
// member that is there already is not added twice; a binding is removed when its
// last member goes. Nothing is decided about them yet.
