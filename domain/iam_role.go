package domain

import (
	"fmt"
	"strings"
)

// IAMRole is the name of a Google Cloud IAM role.
//
// A role can be a Google-managed role (including a basic role), a custom role
// in a project, or a custom role in an organization.
type IAMRole struct {
	name string
}

// NewIAMRole returns name as an IAMRole, or an error if it is not one of the
// role-name forms accepted by IAM:
//
//   - roles/IDENTIFIER
//   - projects/PROJECT_ID/roles/IDENTIFIER
//   - organizations/ORGANIZATION_ID/roles/IDENTIFIER
//
// A custom role identifier is 3 to 64 ASCII letters, digits, underscores, or
// periods. The constructor validates the shape of a project ID but does not
// check whether the project or role exists.
func NewIAMRole(name string) (IAMRole, error) {
	parts := strings.Split(name, "/")
	switch {
	case len(parts) == 2 && parts[0] == "roles":
		if !validPredefinedRoleIdentifier(parts[1]) {
			return IAMRole{}, fmt.Errorf("invalid IAM role %q", name)
		}
	case len(parts) == 4 && parts[0] == "projects" && parts[2] == "roles":
		if !validProjectID(parts[1]) || !validCustomRoleIdentifier(parts[3]) {
			return IAMRole{}, fmt.Errorf("invalid IAM role %q", name)
		}
	case len(parts) == 4 && parts[0] == "organizations" && parts[2] == "roles":
		if !validOrganizationID(parts[1]) || !validCustomRoleIdentifier(parts[3]) {
			return IAMRole{}, fmt.Errorf("invalid IAM role %q", name)
		}
	default:
		return IAMRole{}, fmt.Errorf("invalid IAM role %q", name)
	}
	return IAMRole{name: name}, nil
}

// String returns the role name in the form IAM APIs accept.
func (r IAMRole) String() string { return r.name }

func validPredefinedRoleIdentifier(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func validCustomRoleIdentifier(id string) bool {
	return len(id) >= 3 && len(id) <= 64 && validPredefinedRoleIdentifier(id)
}

func validProjectID(id string) bool {
	if len(id) < 6 || len(id) > 30 {
		return false
	}
	for i, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
		if i == 0 && !(c >= 'a' && c <= 'z') {
			return false
		}
	}
	last := id[len(id)-1]
	return (last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')
}

func validOrganizationID(id string) bool {
	return id != "" && strings.Trim(id, "0123456789") == ""
}
