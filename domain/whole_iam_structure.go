package domain

import "slices"

// WholeIAMStructure is the complete set of Google Group role mappings
// projected from IAM v1 allow-policy bindings.
type WholeIAMStructure struct {
	groupRoleMappings []GroupRoleMapping
}

// NewWholeIAMStructure returns a WholeIAMStructure containing mappings. It
// copies the supplied slice.
func NewWholeIAMStructure(mappings []GroupRoleMapping) WholeIAMStructure {
	return WholeIAMStructure{groupRoleMappings: slices.Clone(mappings)}
}

// GroupRoleMappings returns a copy of all group role mappings in the
// structure.
func (s WholeIAMStructure) GroupRoleMappings() []GroupRoleMapping {
	return slices.Clone(s.groupRoleMappings)
}
