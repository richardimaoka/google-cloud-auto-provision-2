package domain

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
// TODO: Implement the required role mappings and their validation.
func NewRequiredIAMStructure(breakGlass BreakGlass, developerGroupEmail Email, developerRole IAMRole, targetProjectObserverEmail Email) RequiredIAMStructure {
	panic("NewRequiredIAMStructure is not implemented")
}
