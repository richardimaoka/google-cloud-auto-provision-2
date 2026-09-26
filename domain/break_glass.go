package domain

import (
	"fmt"
	"os"
)

const (
	// BreakGlassUltimateGroupEmailEnv names the environment variable that
	// configures the ultimate break-glass Google Group.
	BreakGlassUltimateGroupEmailEnv = "BREAK_GLASS_ULTIMATE_GROUP"
	// BreakGlassEmergencyGroupEmailEnv names the environment variable that
	// configures the emergency break-glass Google Group.
	BreakGlassEmergencyGroupEmailEnv = "BREAK_GLASS_EMERGENCY_GROUP"
)

// BreakGlassRequirement identifies the two Google Groups that must provide
// break-glass access. Its fields are private so callers cannot construct a
// requirement with arbitrary group addresses.
type BreakGlassRequirement struct {
	ultimateGroupEmail  Email
	emergencyGroupEmail Email
}

// NewBreakGlassRequirement loads the required break-glass group addresses from
// the environment.
func NewBreakGlassRequirement() (BreakGlassRequirement, error) {
	ultimate, err := requiredEmailFromEnvironment(BreakGlassUltimateGroupEmailEnv)
	if err != nil {
		return BreakGlassRequirement{}, err
	}
	emergency, err := requiredEmailFromEnvironment(BreakGlassEmergencyGroupEmailEnv)
	if err != nil {
		return BreakGlassRequirement{}, err
	}
	return BreakGlassRequirement{
		ultimateGroupEmail:  ultimate,
		emergencyGroupEmail: emergency,
	}, nil
}

// UltimateGroupEmail returns the ultimate break-glass group's address.
func (r BreakGlassRequirement) UltimateGroupEmail() Email { return r.ultimateGroupEmail }

// EmergencyGroupEmail returns the emergency break-glass group's address.
func (r BreakGlassRequirement) EmergencyGroupEmail() Email { return r.emergencyGroupEmail }

func requiredEmailFromEnvironment(name string) (Email, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return Email{}, fmt.Errorf("required environment variable %s is not set", name)
	}
	email, err := NewEmail(value)
	if err != nil {
		return Email{}, fmt.Errorf("%s: %w", name, err)
	}
	return email, nil
}

// BreakGlass holds the groups that retain emergency access to a project.
type BreakGlass struct {
	ultimateGroup  GoogleGroup
	emergencyGroup GoogleGroup
}

// NewBreakGlass returns BreakGlass if ultimateGroup and emergencyGroup match
// requirement.
func NewBreakGlass(requirement BreakGlassRequirement, ultimateGroup, emergencyGroup GoogleGroup) (BreakGlass, error) {
	if requirement.ultimateGroupEmail == (Email{}) || requirement.emergencyGroupEmail == (Email{}) {
		return BreakGlass{}, fmt.Errorf("break-glass requirement is not configured")
	}
	if ultimateGroup.Email() != requirement.ultimateGroupEmail {
		return BreakGlass{}, fmt.Errorf("ultimate break-glass group is %q, want %q", ultimateGroup.Email(), requirement.ultimateGroupEmail)
	}
	if emergencyGroup.Email() != requirement.emergencyGroupEmail {
		return BreakGlass{}, fmt.Errorf("emergency break-glass group is %q, want %q", emergencyGroup.Email(), requirement.emergencyGroupEmail)
	}
	return BreakGlass{
		ultimateGroup:  cloneGoogleGroup(ultimateGroup),
		emergencyGroup: cloneGoogleGroup(emergencyGroup),
	}, nil
}

// UltimateGroup returns a copy of the group with ultimate emergency access.
func (b BreakGlass) UltimateGroup() GoogleGroup { return cloneGoogleGroup(b.ultimateGroup) }

// EmergencyGroup returns a copy of the group with emergency access.
func (b BreakGlass) EmergencyGroup() GoogleGroup { return cloneGoogleGroup(b.emergencyGroup) }

func cloneGoogleGroup(group GoogleGroup) GoogleGroup {
	return NewGoogleGroup(group.Email(), group.Members())
}
