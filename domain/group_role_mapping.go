package domain

import (
	"fmt"
	"slices"
	"strings"
)

// GroupRoleMapping is the IAM allow-policy roles granted to a Google Group.
//
// UnconditionalRoles are always granted. ConditionalBindings are granted only
// when their IAM condition evaluates to true.
type GroupRoleMapping struct {
	group               GoogleGroup
	unconditionalRoles  []IAMRole
	conditionalBindings []GroupRoleBinding
}

// NewGroupRoleMapping returns a GroupRoleMapping for group. It copies the
// supplied slices.
func NewGroupRoleMapping(group GoogleGroup, unconditionalRoles []IAMRole, conditionalBindings []GroupRoleBinding) GroupRoleMapping {
	return GroupRoleMapping{
		group:               group,
		unconditionalRoles:  slices.Clone(unconditionalRoles),
		conditionalBindings: slices.Clone(conditionalBindings),
	}
}

func (m GroupRoleMapping) Group() GoogleGroup { return m.group }

// UnconditionalRoles returns a copy of the roles that are always granted.
func (m GroupRoleMapping) UnconditionalRoles() []IAMRole {
	return slices.Clone(m.unconditionalRoles)
}

// ConditionalBindings returns a copy of the conditionally granted roles.
func (m GroupRoleMapping) ConditionalBindings() []GroupRoleBinding {
	return slices.Clone(m.conditionalBindings)
}

// GroupRoleBinding is a role granted to a group subject to an IAM condition.
type GroupRoleBinding struct {
	role      IAMRole
	condition IAMCondition
}

// NewGroupRoleBinding returns a conditional role grant.
func NewGroupRoleBinding(role IAMRole, condition IAMCondition) GroupRoleBinding {
	return GroupRoleBinding{role: role, condition: condition}
}

func (b GroupRoleBinding) Role() IAMRole { return b.role }

func (b GroupRoleBinding) Condition() IAMCondition { return b.condition }

// IAMCondition is the condition attached to an IAM allow-policy binding.
type IAMCondition struct {
	expression  string
	title       string
	description string
}

// NewIAMCondition returns an IAMCondition with a non-empty CEL expression.
func NewIAMCondition(expression, title, description string) (IAMCondition, error) {
	if strings.TrimSpace(expression) == "" {
		return IAMCondition{}, fmt.Errorf("IAM condition expression must not be empty")
	}
	return IAMCondition{
		expression:  expression,
		title:       title,
		description: description,
	}, nil
}

func (c IAMCondition) Expression() string { return c.expression }

func (c IAMCondition) Title() string { return c.title }

func (c IAMCondition) Description() string { return c.description }
