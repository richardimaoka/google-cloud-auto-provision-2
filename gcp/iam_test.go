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
//   - Inputs a real project can't have: folders and organizations, and resource
//     names that are not valid.
//   - Failures we can't cause on demand: a conflict on the etag (409), and the
//     waits between retries.
//   - Logic that is our own: the read-modify-write loop of UpdatePolicy.
//   - What the client sends, when a wrong request would damage a real policy: the
//     policy version, the etag, and the conditions that must go back unchanged.
//
// They use a fake httptest server, like projects_test.go, that serves
//
//	POST /v3/projects/<id>:getIamPolicy
//	POST /v3/projects/<id>:setIamPolicy
//	POST /v3/folders/<id>:getIamPolicy    (and :setIamPolicy)
//	POST /v3/organizations/<id>:getIamPolicy    (and :setIamPolicy)
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
// UpdatePolicy(ctx, resource, update func(*Policy) error) error is the name
// used here for the read-modify-write operation. The etag is kept inside it, so
// Policy has no Etag.
//
// Every case below is skipped until it is written.

// The resource names, on their own: a table test on the function that checks a
// name and picks the endpoint, with no server.
func TestParseIAMResource(t *testing.T) {
	t.Run("有効なリソース名", func(t *testing.T) {
		// "projects/<id>", "projects/<number>", "folders/<number>" and
		// "organizations/<number>".
		t.Skip("not implemented")
	})
	t.Run("無効なリソース名", func(t *testing.T) {
		// "", "projects/", "projects/a/b", "buckets/x" and "my-project" are an
		// error.
		t.Skip("not implemented")
	})
}

func TestIAMClient_GetPolicy(t *testing.T) {
	t.Run("バージョン3を要求する", func(t *testing.T) {
		// The method is POST, and the body is
		// {"options":{"requestedPolicyVersion":3}}, so a condition is never left
		// out. The response can be as small as possible.
		t.Skip("not implemented")
	})
	t.Run("リソースの種類ごとのエンドポイント", func(t *testing.T) {
		// "projects/<id>", "folders/<id>" and "organizations/<id>" go to their own
		// endpoints. A real project can't show this for folders and organizations.
		t.Skip("not implemented")
	})
	t.Run("無効なリソース名ならリクエストを送らない", func(t *testing.T) {
		// The error comes back and the fake has seen no request. The names
		// themselves are in TestParseIAMResource.
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
	t.Run("無効なリソース名ならリクエストを送らない", func(t *testing.T) {
		// Like GetPolicy, and update is not called.
		t.Skip("not implemented")
	})
	t.Run("リソースの種類ごとのエンドポイント", func(t *testing.T) {
		// Read and write both go to the endpoints of "folders/<id>" and
		// "organizations/<id>".
		t.Skip("not implemented")
	})
}

// If Policy gets methods of its own, like AddMember or RemoveMember, their tests
// go here, as plain tests on a Policy with no server: a member is added to the
// binding that has the same role and the same condition, and to no other; a
// member that is there already is not added twice; a binding is removed when its
// last member goes. Nothing is decided about them yet.
