//go:build integration

package gcp

import "testing"

// This test calls the real Resource Manager API, and rewrites the IAM policy of a
// real project. It costs no quota, but a wrong project would lose its policy, so
// it must only be pointed at a project made for it. It shares the environment of
// the other integration tests, see groups_integration_test.go. The project is
// not created for each run: the ID of a deleted project can't be reused, and the
// project of the lifecycle test is deleted at its end. The variable that names
// the project is not decided yet.
//
// A run does this, in order. Later steps depend on the earlier ones, so they are
// steps of one test.
//
//  1. Save the policy the project has now, and restore it when the test ends,
//     even if it failed.
//  2. Reduce the policy to almost nothing: keep the roles/owner binding of the
//     account that runs the test. Removing it would leave nobody who can change
//     the policy again.
//  3. Check what the API does that the fake can't show, in the steps below.
//
// A member has to exist, because the API checks it: use the account that runs
// the test, a service account made for the test, and the group of the groups
// integration test. A member that doesn't exist is rejected.
func TestIAMClient_Policy_Integration(t *testing.T) {
	t.Run("ほぼ空のポリシーを読める", func(t *testing.T) {
		// GetPolicy returns the roles/owner binding whose Members include the
		// caller, and nothing else. Version is 1.
		t.Skip("not implemented")
	})
	t.Run("無条件のバインディングを足して読み戻せる", func(t *testing.T) {
		// UpdatePolicy adds a service account and a group, each with a role, and
		// one binding with two members. GetPolicy returns the same Bindings, so
		// this also shows how the Policy of ours turns into the API's and back.
		t.Skip("not implemented")
	})
	t.Run("条件付きバインディングを足して読み戻せる", func(t *testing.T) {
		// UpdatePolicy adds a binding with a Condition (a title, a description and
		// an expression on request.time). GetPolicy returns it with the same three
		// fields, and Version 3. The same role without a condition, kept as its own
		// binding, is still there. This is what needs policy version 3.
		t.Skip("not implemented")
	})
	t.Run("条件付きバインディングのあるポリシーを更新しても条件が残る", func(t *testing.T) {
		// UpdatePolicy adds a member to an unconditional binding. The conditional
		// binding from the step before has its condition still.
		t.Skip("not implemented")
	})
	t.Run("存在しないプロジェクトは 403", func(t *testing.T) {
		// GetPolicy of a project that does not exist returns an error that wraps a
		// *googleapi.Error with code 403, and says "get iam policy". The API does
		// not tell a project that does not exist from one the caller can't use.
		t.Skip("not implemented")
	})
}

// Not here: a conflict on the etag, which a real project can't be made to give in
// a fixed order (the unit tests have it), and the policy of a project right after
// Create, which is one more check for the lifecycle test in
// projects_integration_test.go: a roles/owner binding whose Members include the
// caller.
