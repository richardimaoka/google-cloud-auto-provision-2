package domain

import "testing"

func TestUserRoleMapping(t *testing.T) {
	user := email(t, "user@example.com")
	role := iamRole(t, "roles/viewer")
	binding := NewUserRoleBinding(role, iamCondition(t, "request.time < timestamp('2030-01-01T00:00:00Z')"))

	mapping := NewUserRoleMapping(user, []IAMRole{role}, []UserRoleBinding{binding})
	if got, want := mapping.User(), user; got != want {
		t.Errorf("User() = %q, want %q", got, want)
	}
	if got := mapping.UnconditionalRoles(); len(got) != 1 || got[0] != role {
		t.Errorf("UnconditionalRoles() = %v, want [%v]", got, role)
	}
	if got := mapping.ConditionalBindings(); len(got) != 1 || got[0] != binding {
		t.Errorf("ConditionalBindings() = %v, want [%v]", got, binding)
	}
}
