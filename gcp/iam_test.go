package gcp

import "testing"

// Unit tests for IAMClient.
//
// They cover failures we can't cause on demand (a conflict on the etag), our own
// logic (the read-modify-write loop and the last-owner guard), and what the client
// sends (policy version, etag, conditions).
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
// The clients take a project ID, or a number, and do not check it, like
// ProjectsClient. Only projects are handled: nothing here is for folders or
// organizations.
//
// What is tested, from the outside in. The names are the ones planned, and the
// errors are provisional.
//
//	GetPolicy(ctx, projectID) (*Policy, error)
//	AddBinding(ctx, projectID, role, member string) error
//	RemoveBinding(ctx, projectID, role, member string) error
//
// AddBinding and RemoveBinding take one role and one member, both strings, the
// member in the form the API uses, like "user:alice@example.com". They are named
// after gcloud's add-iam-policy-binding and remove-iam-policy-binding.
// AddBinding does nothing if the member has the role already, and writes nothing.
// RemoveBinding returns ErrBindingNotFound if the member does not have the role:
// the name given does not match anything, like a role that does not exist does for
// AddBinding.
//
// This client reads conditional bindings, but does not set them. Neither method
// takes a condition, and they work on the unconditional bindings only, like gcloud
// does with --condition=None: a member is added to the unconditional binding of a
// role, and removed from it, and a binding that has a Condition is never touched.
// A write sends the conditional bindings back as they were read, so it does not
// change who has access.
//
// Below them is updatePolicy, in package gcp and not exported:
//
//	(*IAMClient).updatePolicy(ctx, projectID, update func(*Policy) error) error
//
// It is the read-modify-write operation. The etag is kept inside it, so Policy has
// no Etag. It does not write if update left the policy equal to the one it read.
// What AddBinding and RemoveBinding do to a policy, find the unconditional binding
// of a role and put a member in it or take one out, is small, and is tested through
// them, not on a Policy by itself. A binding whose last member goes is removed.
//
// The guard: a project whose policy has no owner can't be changed by anyone who
// gets access only through it. So updatePolicy refuses a change that leaves a
// project with no owner, when the policy it read had one. An owner is a member of
// an unconditional roles/owner binding, and a "deleted:" member is not one, as it
// grants nothing. The error is ErrLastOwner, found with errors.Is, and nothing is
// written or retried. A policy that had no owner to begin with, for example
// because the owners come from the organization, is written as it is.
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
