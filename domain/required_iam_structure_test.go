package domain

import (
	"slices"
	"testing"
)

func TestRequiredIAMStructure(t *testing.T) {
	ultimate := NewGoogleGroup(email(t, "ultimate@example.com"), nil)
	emergency := NewGoogleGroup(email(t, "emergency@example.com"), nil)
	breakGlass, err := NewBreakGlass(
		breakGlassRequirement(t, ultimate.Email().String(), emergency.Email().String()),
		ultimate,
		emergency,
	)
	if err != nil {
		t.Fatalf("NewBreakGlass() error = %v", err)
	}
	developerMember := NewGoogleGroupMember(email(t, "developer@example.com"), "MEMBER")
	developerGroup := NewGoogleGroup(email(t, "developers@example.com"), []GoogleGroupMember{developerMember})
	developerRoles := []IAMRole{
		iamRole(t, "roles/viewer"),
		iamRole(t, "roles/logging.viewer"),
		iamRole(t, "roles/monitoring.viewer"),
	}
	observerEmail := email(t, "observer@example-project.iam.gserviceaccount.com")

	t.Run("ultimateグループに無条件のOwnerを割り当てる", func(t *testing.T) {
		structure := NewRequiredIAMStructure(breakGlass, developerGroup, developerRoles, observerEmail)
		mapping := structure.BreakGlassUltimateGroup
		if got, want := mapping.Group().Email(), breakGlass.UltimateGroup().Email(); got != want {
			t.Errorf("Group().Email() = %q, want %q", got, want)
		}
		if got, want := mapping.UnconditionalRoles(), []IAMRole{iamRole(t, "roles/owner")}; !slices.Equal(got, want) {
			t.Errorf("UnconditionalRoles() = %v, want %v", got, want)
		}
		if got := mapping.ConditionalBindings(); len(got) != 0 {
			t.Errorf("ConditionalBindings() = %v, want none", got)
		}
	})
	t.Run("emergencyグループにOwnerを割り当てずProject IAM Adminを割り当てる", func(t *testing.T) {
		structure := NewRequiredIAMStructure(breakGlass, developerGroup, developerRoles, observerEmail)
		mapping := structure.BreakGlassEmergencyGroup
		if got, want := mapping.Group().Email(), breakGlass.EmergencyGroup().Email(); got != want {
			t.Errorf("Group().Email() = %q, want %q", got, want)
		}
		if got, want := mapping.UnconditionalRoles(), []IAMRole{iamRole(t, "roles/resourcemanager.projectIamAdmin")}; !slices.Equal(got, want) {
			t.Errorf("UnconditionalRoles() = %v, want %v", got, want)
		}
		if got := mapping.ConditionalBindings(); len(got) != 0 {
			t.Errorf("ConditionalBindings() = %v, want none", got)
		}
	})
	t.Run("developerグループに指定されたロールを割り当てる", func(t *testing.T) {
		structure := NewRequiredIAMStructure(breakGlass, developerGroup, developerRoles, observerEmail)
		mapping := structure.DeveloperGroup
		if got, want := mapping.Group().Email(), developerGroup.Email(); got != want {
			t.Errorf("Group().Email() = %q, want %q", got, want)
		}
		if got, want := mapping.Group().Members(), developerGroup.Members(); !slices.Equal(got, want) {
			t.Errorf("Group().Members() = %v, want %v", got, want)
		}
		if got := mapping.UnconditionalRoles(); !slices.Equal(got, developerRoles) {
			t.Errorf("UnconditionalRoles() = %v, want %v", got, developerRoles)
		}
		if got := mapping.ConditionalBindings(); len(got) != 0 {
			t.Errorf("ConditionalBindings() = %v, want none", got)
		}
	})
	t.Run("target project observerに必要な参照ロールを割り当てる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}
