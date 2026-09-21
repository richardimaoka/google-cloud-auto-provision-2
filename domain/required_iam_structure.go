package domain

// RequiredIAMStructure is the required IAM allow-policy structure for the
// break-glass and developer Google Groups.
//
// Each mapping has distinct validation requirements: the ultimate break-glass
// group must be an Owner, the emergency break-glass group must retain IAM
// administration capabilities without being an Owner, and the developer group
// must not be an Owner.
type RequiredIAMStructure struct {
	BreakGlassUltimateGroup  GroupRoleMapping
	BreakGlassEmergencyGroup GroupRoleMapping
	DeveloperGroup           GroupRoleMapping
}
