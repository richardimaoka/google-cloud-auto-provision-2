package domain

import (
	"errors"
	"fmt"
	"strings"
)

type ProjectCreationRequest struct {
	DisplayName    string
	Requester      Email
	DeveloperGroup GoogleGroup
}

// NewProjectCreationRequest returns an error if displayName is invalid, or if
// requester or developerGroup's email is the zero Email.
func NewProjectCreationRequest(displayName string, requester Email, developerGroup GoogleGroup) (ProjectCreationRequest, error) {
	if !validProjectDisplayName(displayName) {
		return ProjectCreationRequest{}, fmt.Errorf("invalid project display name %q", displayName)
	}
	if requester == (Email{}) {
		return ProjectCreationRequest{}, errors.New("requester is empty")
	}
	if developerGroup.email == (Email{}) {
		return ProjectCreationRequest{}, errors.New("developer group email is empty")
	}
	return ProjectCreationRequest{
		DisplayName:    displayName,
		Requester:      requester,
		DeveloperGroup: developerGroup,
	}, nil
}

// validProjectDisplayName reports whether name is a valid Google Cloud project
// display name: 4 to 30 permitted ASCII characters, ending in a letter or digit.
func validProjectDisplayName(name string) bool {
	if len(name) < 4 || len(name) > 30 {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '\'' || c == '"' || c == ' ' || c == '!') {
			return false
		}
	}
	last := name[len(name)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9')
}

// minDeveloperGroupMembers is the fewest members a developer group may have.
const minDeveloperGroupMembers = 2

// Validate reports every way r does not fit org. The requester, the developer
// group and its members must be in org's domain (case-insensitive), and the
// group must have at least minDeveloperGroupMembers members.
func (r ProjectCreationRequest) Validate(org Organization) error {
	var errs []error
	if n := len(r.DeveloperGroup.members); n < minDeveloperGroupMembers {
		errs = append(errs, fmt.Errorf("developer group %q has %d member(s), want at least %d", r.DeveloperGroup.email, n, minDeveloperGroupMembers))
	}
	if !strings.EqualFold(r.Requester.Domain(), org.domain) {
		errs = append(errs, fmt.Errorf("requester %q is not in the organization domain %q", r.Requester, org.domain))
	}
	if !strings.EqualFold(r.DeveloperGroup.email.Domain(), org.domain) {
		errs = append(errs, fmt.Errorf("developer group %q is not in the organization domain %q", r.DeveloperGroup.email, org.domain))
	}
	for _, m := range r.DeveloperGroup.members {
		if !strings.EqualFold(m.email.Domain(), org.domain) {
			errs = append(errs, fmt.Errorf("developer group member %q is not in the organization domain %q", m.email, org.domain))
		}
	}
	return errors.Join(errs...)
}
