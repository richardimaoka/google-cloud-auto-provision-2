package domain

import "slices"

// ServiceAccountRoleMapping is the IAM allow-policy roles granted to a service
// account.
type ServiceAccountRoleMapping struct {
	serviceAccount      Email
	unconditionalRoles  []IAMRole
	conditionalBindings []ServiceAccountRoleBinding
}

// NewServiceAccountRoleMapping returns a ServiceAccountRoleMapping for service
// account. It copies the supplied slices.
func NewServiceAccountRoleMapping(serviceAccount Email, unconditionalRoles []IAMRole, conditionalBindings []ServiceAccountRoleBinding) ServiceAccountRoleMapping {
	return ServiceAccountRoleMapping{
		serviceAccount:      serviceAccount,
		unconditionalRoles:  slices.Clone(unconditionalRoles),
		conditionalBindings: slices.Clone(conditionalBindings),
	}
}

func (m ServiceAccountRoleMapping) ServiceAccount() Email { return m.serviceAccount }

// UnconditionalRoles returns a copy of the roles that are always granted.
func (m ServiceAccountRoleMapping) UnconditionalRoles() []IAMRole {
	return slices.Clone(m.unconditionalRoles)
}

// ConditionalBindings returns a copy of the conditionally granted roles.
func (m ServiceAccountRoleMapping) ConditionalBindings() []ServiceAccountRoleBinding {
	return slices.Clone(m.conditionalBindings)
}

// ServiceAccountRoleBinding is a role granted to a service account subject to
// an IAM condition.
type ServiceAccountRoleBinding struct {
	role      IAMRole
	condition IAMCondition
}

// NewServiceAccountRoleBinding returns a conditional role grant.
func NewServiceAccountRoleBinding(role IAMRole, condition IAMCondition) ServiceAccountRoleBinding {
	return ServiceAccountRoleBinding{role: role, condition: condition}
}

func (b ServiceAccountRoleBinding) Role() IAMRole { return b.role }

func (b ServiceAccountRoleBinding) Condition() IAMCondition { return b.condition }
