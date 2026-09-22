package domain

import (
	"errors"
	"fmt"
	"slices"
)

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

// Validate returns an error for every required IAM role or binding that is
// absent from actual. Extra principals are allowed.
//
// A required unconditional role must be unconditional in actual; a
// conditional binding cannot satisfy it. Required mappings with no roles or
// bindings are ignored until their requirements are defined. The ultimate
// and emergency break-glass group mappings, and the developer group mapping,
// must exactly match their requirements.
func (r RequiredIAMStructure) Validate(actual WholeIAMStructure) error {
	var errs []error
	errs = append(errs, validateExactlyRequiredGroupRoleMapping(r.BreakGlassUltimateGroup, actual.GroupRoleMappings())...)
	errs = append(errs, validateExactlyRequiredGroupRoleMapping(r.BreakGlassEmergencyGroup, actual.GroupRoleMappings())...)
	errs = append(errs, validateExactlyRequiredGroupRoleMapping(r.DeveloperGroup, actual.GroupRoleMappings())...)
	errs = append(errs, validateRequiredServiceAccountRoleMapping(r.TargetProjectObserver, actual.ServiceAccountRoleMappings())...)
	return errors.Join(errs...)
}

func validateExactlyRequiredGroupRoleMapping(required GroupRoleMapping, actualMappings []GroupRoleMapping) []error {
	for _, actual := range actualMappings {
		if actual.Group().Email() != required.Group().Email() {
			continue
		}
		if sameIAMRoles(required.UnconditionalRoles(), actual.UnconditionalRoles()) &&
			sameGroupRoleBindings(required.ConditionalBindings(), actual.ConditionalBindings()) {
			return nil
		}
		return []error{fmt.Errorf("group %q roles and conditional bindings do not exactly match the required mapping", required.Group().Email())}
	}
	return []error{fmt.Errorf("required group %q has no IAM role mapping", required.Group().Email())}
}

func sameIAMRoles(a, b []IAMRole) bool {
	return len(a) == len(b) && allIAMRolesIn(a, b)
}

func allIAMRolesIn(want, got []IAMRole) bool {
	for _, role := range want {
		if !slices.Contains(got, role) {
			return false
		}
	}
	return true
}

func sameGroupRoleBindings(a, b []GroupRoleBinding) bool {
	return len(a) == len(b) && allGroupRoleBindingsIn(a, b)
}

func allGroupRoleBindingsIn(want, got []GroupRoleBinding) bool {
	for _, binding := range want {
		if !slices.Contains(got, binding) {
			return false
		}
	}
	return true
}

func validateRequiredServiceAccountRoleMapping(required ServiceAccountRoleMapping, actualMappings []ServiceAccountRoleMapping) []error {
	if len(required.UnconditionalRoles()) == 0 && len(required.ConditionalBindings()) == 0 {
		return nil
	}
	for _, actual := range actualMappings {
		if actual.ServiceAccount() == required.ServiceAccount() {
			return missingUnconditionalServiceAccountRoles(required, actual)
		}
	}
	return []error{fmt.Errorf("required service account %q has no IAM role mapping", required.ServiceAccount())}
}

func missingUnconditionalServiceAccountRoles(required, actual ServiceAccountRoleMapping) []error {
	var errs []error
	for _, role := range required.UnconditionalRoles() {
		if !slices.Contains(actual.UnconditionalRoles(), role) {
			errs = append(errs, fmt.Errorf("service account %q is missing unconditional role %q", required.ServiceAccount(), role))
		}
	}
	for _, binding := range required.ConditionalBindings() {
		if !slices.Contains(actual.ConditionalBindings(), binding) {
			errs = append(errs, fmt.Errorf("service account %q is missing conditional binding for role %q", required.ServiceAccount(), binding.Role()))
		}
	}
	return errs
}
