package domain

import "testing"

func TestNewBreakGlass(t *testing.T) {
	ultimateMember := NewGoogleGroupMember(email(t, "ultimate-member@example.com"), "MEMBER")
	emergencyMember := NewGoogleGroupMember(email(t, "emergency-member@example.com"), "MEMBER")
	ultimateGroup := NewGoogleGroup(email(t, "ultimate@example.com"), []GoogleGroupMember{ultimateMember})
	emergencyGroup := NewGoogleGroup(email(t, "emergency@example.com"), []GoogleGroupMember{emergencyMember})

	breakGlass := NewBreakGlass(ultimateGroup, emergencyGroup)
	if got := breakGlass.UltimateGroup(); got.Email() != ultimateGroup.Email() || got.Members()[0] != ultimateMember {
		t.Errorf("UltimateGroup() = %#v, want %#v", got, ultimateGroup)
	}
	if got := breakGlass.EmergencyGroup(); got.Email() != emergencyGroup.Email() || got.Members()[0] != emergencyMember {
		t.Errorf("EmergencyGroup() = %#v, want %#v", got, emergencyGroup)
	}
}

func TestBreakGlass_GroupsAreCopied(t *testing.T) {
	member := NewGoogleGroupMember(email(t, "member@example.com"), "MEMBER")
	other := NewGoogleGroupMember(email(t, "other@example.com"), "MEMBER")
	ultimateGroup := NewGoogleGroup(email(t, "ultimate@example.com"), []GoogleGroupMember{member})
	breakGlass := NewBreakGlass(ultimateGroup, GoogleGroup{})

	breakGlass.ultimateGroup.members[0] = other
	if got := ultimateGroup.Members()[0]; got != member {
		t.Errorf("input group changed to %v, want %v", got, member)
	}

	returnedGroup := breakGlass.UltimateGroup()
	returnedGroup.members[0] = member
	if got := breakGlass.UltimateGroup().Members()[0]; got != other {
		t.Errorf("BreakGlass followed a change to a returned group: got %v, want %v", got, other)
	}
}
