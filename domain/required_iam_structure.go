package domain

import "fmt"

// RequiredIAMStructure is the required IAM allow-policy structure for the
// break-glass and developer Google Groups, and the target-project observer.
//
// Each mapping has distinct validation requirements: the ultimate break-glass
// group must be an Owner, the emergency break-glass group must retain IAM
// administration capabilities without being an Owner, and the developer group
// must not be an Owner.
type RequiredIAMStructure struct {
	BreakGlassUltimateGroup  GroupRoleMapping
	BreakGlassEmergencyGroup GroupRoleMapping
	DeveloperGroup           GroupRoleMapping
	TargetProjectObserver    ServiceAccountRoleMapping
}

// NewRequiredIAMStructure returns the IAM structure required for break-glass
// groups and the developer group.
//
// The target-project observer role mapping is populated without roles until
// its required read permissions are defined.
func NewRequiredIAMStructure(breakGlass BreakGlass, developerGroup GoogleGroup, developerRoles []IAMRole, targetProjectObserverEmail Email) RequiredIAMStructure {
	return RequiredIAMStructure{
		BreakGlassUltimateGroup: NewGroupRoleMapping(
			breakGlass.UltimateGroup(),
			[]IAMRole{requiredIAMRole("roles/owner")},
			nil,
		),
		BreakGlassEmergencyGroup: NewGroupRoleMapping(
			breakGlass.EmergencyGroup(),
			[]IAMRole{requiredIAMRole("roles/resourcemanager.projectIamAdmin")},
			nil,
		),
		DeveloperGroup: NewGroupRoleMapping(
			developerGroup,
			developerRoles,
			nil,
		),
		TargetProjectObserver: NewServiceAccountRoleMapping(targetProjectObserverEmail, nil, nil),
	}
}

func requiredIAMRole(name string) IAMRole {
	role, err := NewIAMRole(name)
	if err != nil {
		panic(fmt.Sprintf("invalid required IAM role %q: %v", name, err))
	}
	return role
}
