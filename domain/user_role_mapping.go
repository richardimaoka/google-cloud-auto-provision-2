package domain

import "slices"

// UserRoleMapping is the IAM allow-policy roles granted to an individual user.
type UserRoleMapping struct {
	user                Email
	unconditionalRoles  []IAMRole
	conditionalBindings []UserRoleBinding
}

// NewUserRoleMapping returns a UserRoleMapping for user. It copies the
// supplied slices.
func NewUserRoleMapping(user Email, unconditionalRoles []IAMRole, conditionalBindings []UserRoleBinding) UserRoleMapping {
	return UserRoleMapping{
		user:                user,
		unconditionalRoles:  slices.Clone(unconditionalRoles),
		conditionalBindings: slices.Clone(conditionalBindings),
	}
}

func (m UserRoleMapping) User() Email { return m.user }

// UnconditionalRoles returns a copy of the roles that are always granted.
func (m UserRoleMapping) UnconditionalRoles() []IAMRole {
	return slices.Clone(m.unconditionalRoles)
}

// ConditionalBindings returns a copy of the conditionally granted roles.
func (m UserRoleMapping) ConditionalBindings() []UserRoleBinding {
	return slices.Clone(m.conditionalBindings)
}

// UserRoleBinding is a role granted to a user subject to an IAM condition.
type UserRoleBinding struct {
	role      IAMRole
	condition IAMCondition
}

// NewUserRoleBinding returns a conditional role grant.
func NewUserRoleBinding(role IAMRole, condition IAMCondition) UserRoleBinding {
	return UserRoleBinding{role: role, condition: condition}
}

func (b UserRoleBinding) Role() IAMRole { return b.role }

func (b UserRoleBinding) Condition() IAMCondition { return b.condition }
