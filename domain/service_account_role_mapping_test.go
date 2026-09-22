package domain

import "testing"

func TestServiceAccountRoleMapping(t *testing.T) {
	// サービスアカウントの無条件ロールと条件付きロールを保持できる。
	serviceAccount := email(t, "service@example-project.iam.gserviceaccount.com")
	role := iamRole(t, "roles/logging.viewer")
	binding := NewServiceAccountRoleBinding(role, iamCondition(t, "request.time < timestamp('2030-01-01T00:00:00Z')"))

	mapping := NewServiceAccountRoleMapping(serviceAccount, []IAMRole{role}, []ServiceAccountRoleBinding{binding})
	if got, want := mapping.ServiceAccount(), serviceAccount; got != want {
		t.Errorf("ServiceAccount() = %q, want %q", got, want)
	}
	if got := mapping.UnconditionalRoles(); len(got) != 1 || got[0] != role {
		t.Errorf("UnconditionalRoles() = %v, want [%v]", got, role)
	}
	if got := mapping.ConditionalBindings(); len(got) != 1 || got[0] != binding {
		t.Errorf("ConditionalBindings() = %v, want [%v]", got, binding)
	}
}
