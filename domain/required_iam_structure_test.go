package domain

import "testing"

func TestRequiredIAMStructure(t *testing.T) {
	// 3 種類の必須グループマッピングを保持できる。
	ultimate := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "ultimate@example.com"), nil),
		[]IAMRole{iamRole(t, "roles/owner")},
		nil,
	)
	emergency := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "emergency@example.com"), nil),
		[]IAMRole{iamRole(t, "roles/resourcemanager.projectIamAdmin")},
		nil,
	)
	developer := NewGroupRoleMapping(
		NewGoogleGroup(email(t, "developers@example.com"), nil),
		nil,
		nil,
	)

	structure := RequiredIAMStructure{
		BreakGlassUltimateGroup:  ultimate,
		BreakGlassEmergencyGroup: emergency,
		DeveloperGroup:           developer,
	}

	if got, want := structure.BreakGlassUltimateGroup.Group().Email(), ultimate.Group().Email(); got != want {
		t.Errorf("BreakGlassUltimateGroup.Group().Email() = %q, want %q", got, want)
	}
	if got, want := structure.BreakGlassEmergencyGroup.Group().Email(), emergency.Group().Email(); got != want {
		t.Errorf("BreakGlassEmergencyGroup.Group().Email() = %q, want %q", got, want)
	}
	if got, want := structure.DeveloperGroup.Group().Email(), developer.Group().Email(); got != want {
		t.Errorf("DeveloperGroup.Group().Email() = %q, want %q", got, want)
	}
}
