package domain

import "slices"

// WholeIAMStructure is the complete set of user, Google Group, and service
// account role mappings projected from IAM v1 allow-policy bindings.
type WholeIAMStructure struct {
	userRoleMappings           []UserRoleMapping
	groupRoleMappings          []GroupRoleMapping
	serviceAccountRoleMappings []ServiceAccountRoleMapping
}

// NewWholeIAMStructure returns a WholeIAMStructure containing mappings. It
// copies the supplied slice.
func NewWholeIAMStructure(users []UserRoleMapping, groups []GroupRoleMapping, serviceAccounts []ServiceAccountRoleMapping) WholeIAMStructure {
	return WholeIAMStructure{
		userRoleMappings:           slices.Clone(users),
		groupRoleMappings:          slices.Clone(groups),
		serviceAccountRoleMappings: slices.Clone(serviceAccounts),
	}
}

// UserRoleMappings returns a copy of all user role mappings in the structure.
func (s WholeIAMStructure) UserRoleMappings() []UserRoleMapping {
	return slices.Clone(s.userRoleMappings)
}

// GroupRoleMappings returns a copy of all group role mappings in the
// structure.
func (s WholeIAMStructure) GroupRoleMappings() []GroupRoleMapping {
	return slices.Clone(s.groupRoleMappings)
}

// ServiceAccountRoleMappings returns a copy of all service account role
// mappings in the structure.
func (s WholeIAMStructure) ServiceAccountRoleMappings() []ServiceAccountRoleMapping {
	return slices.Clone(s.serviceAccountRoleMappings)
}
