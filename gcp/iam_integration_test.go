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
//     even if it failed. It goes through updatePolicy, which the test can call
//     as it is in package gcp.
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
		// AddBinding adds a service account and a group, each with a role, and a
		// second member to one of those roles, one call for each. GetPolicy returns
		// the same Bindings, so this also shows how the Policy of ours turns into
		// the API's and back.
		t.Skip("not implemented")
	})
	t.Run("同じものをもう一度足しても、ポリシーは変わらない", func(t *testing.T) {
		// AddBinding of a role and a member that are there returns no error, and the
		// policy GetPolicy returns is the same, member by member.
		t.Skip("not implemented")
	})
	t.Run("条件付きバインディングを読める", func(t *testing.T) {
		// The test makes the binding with updatePolicy, which the client does not
		// offer as a way to set a condition: a Condition (a title, a description
		// and an expression on request.time) on a role that has an unconditional
		// binding too. GetPolicy returns it with the same three fields, and Version
		// 3, and the unconditional binding as its own. This is what needs policy
		// version 3.
		t.Skip("not implemented")
	})
	t.Run("条件付きバインディングのあるポリシーを更新しても条件が残る", func(t *testing.T) {
		// AddBinding adds a member to an unconditional binding. The conditional
		// binding from the step before has its condition still.
		t.Skip("not implemented")
	})
	t.Run("存在しないメンバーやロールは足せず、ポリシーは変わらない", func(t *testing.T) {
		// AddBinding of a role that does not exist, and of a member that does not
		// exist, each return an error that wraps a *googleapi.Error with code 400.
		// The API checks both, which the fake can't. GetPolicy afterwards returns the
		// same policy as before.
		t.Skip("not implemented")
	})
	t.Run("ないメンバーを外すと ErrBindingNotFound", func(t *testing.T) {
		// RemoveBinding of a member that does not have the role returns
		// ErrBindingNotFound, and the policy is the same.
		t.Skip("not implemented")
	})
	t.Run("最後の owner を外そうとするとエラーで、ポリシーは変わらない", func(t *testing.T) {
		// RemoveBinding of every roles/owner member returns ErrLastOwner. GetPolicy
		// afterwards returns the same policy as before, so the guard stopped the
		// write, and the account can still change the policy.
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
