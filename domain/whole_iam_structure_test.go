package domain

import "testing"

func TestWholeIAMStructure(t *testing.T) {
	user := NewUserRoleMapping(email(t, "user@example.com"), []IAMRole{iamRole(t, "roles/viewer")}, nil)
	developers := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "developers@example.com"), nil),
		[]IAMRole{iamRole(t, "roles/viewer")},
		nil,
	)
	admins := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "admins@example.com"), nil),
		[]IAMRole{iamRole(t, "roles/owner")},
		nil,
	)
	serviceAccount := NewServiceAccountRoleMapping(
		email(t, "service@example-project.iam.gserviceaccount.com"),
		[]IAMRole{iamRole(t, "roles/logging.viewer")},
		nil,
	)

	structure := NewWholeIAMStructure([]UserRoleMapping{user}, []GroupRoleMapping{developers, admins}, []ServiceAccountRoleMapping{serviceAccount})
	if got, want := structure.UserRoleMappings()[0].User(), user.User(); got != want {
		t.Errorf("UserRoleMappings()[0].User() = %q, want %q", got, want)
	}
	got := structure.GroupRoleMappings()
	if len(got) != 2 {
		t.Fatalf("len(GroupRoleMappings()) = %d, want 2", len(got))
	}
	if got, want := got[0].Group().Email(), developers.Group().Email(); got != want {
		t.Errorf("GroupRoleMappings()[0].Group().Email() = %q, want %q", got, want)
	}
	if got, want := got[1].Group().Email(), admins.Group().Email(); got != want {
		t.Errorf("GroupRoleMappings()[1].Group().Email() = %q, want %q", got, want)
	}
	if got, want := structure.ServiceAccountRoleMappings()[0].ServiceAccount(), serviceAccount.ServiceAccount(); got != want {
		t.Errorf("ServiceAccountRoleMappings()[0].ServiceAccount() = %q, want %q", got, want)
	}
}

func TestWholeIAMStructure_mappingsAreCopied(t *testing.T) {
	developers := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "developers@example.com"), nil),
		nil,
		nil,
	)
	admins := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "admins@example.com"), nil),
		nil,
		nil,
	)
	mappings := []GroupRoleMapping{developers}
	structure := NewWholeIAMStructure(nil, mappings, nil)

	mappings[0] = admins
	if got, want := structure.GroupRoleMappings()[0].Group().Email(), developers.Group().Email(); got != want {
		t.Errorf("GroupRoleMappings()[0].Group().Email() = %q, want %q", got, want)
	}

	returned := structure.GroupRoleMappings()
	returned[0] = admins
	if got, want := structure.GroupRoleMappings()[0].Group().Email(), developers.Group().Email(); got != want {
		t.Errorf("GroupRoleMappings() followed a change to its returned slice: got %q, want %q", got, want)
	}
}
