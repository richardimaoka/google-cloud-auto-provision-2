// Package domain holds the domain model, independent of the Google Cloud API types.
//
// It depends on the standard library only, and on nothing that does I/O.
package domain

import (
	"errors"
	"fmt"
	// Only parses addresses, never touches the network, so it keeps this package free of I/O.
	"net/mail"
	"strings"
)

type ProjectCreationRequest struct {
	Requester      Email
	DeveloperGroup GoogleGroup
}

// NewProjectCreationRequest returns a ProjectCreationRequest, or an error if
// requester or developerGroup's Email is the zero Email.
func NewProjectCreationRequest(requester Email, developerGroup GoogleGroup) (ProjectCreationRequest, error) {
	if requester == (Email{}) {
		return ProjectCreationRequest{}, errors.New("requester is empty")
	}
	if developerGroup.Email == (Email{}) {
		return ProjectCreationRequest{}, errors.New("developer group email is empty")
	}
	return ProjectCreationRequest{
		Requester:      requester,
		DeveloperGroup: developerGroup,
	}, nil
}

// minDeveloperGroupMembers is the fewest members a developer group may have.
const minDeveloperGroupMembers = 2

// Validate returns an error if r does not fit org, or nil if it does. It reports
// every problem found, not just the first. Domains are compared case-insensitively.
//
// The requester and the developer group must belong to org's domain, and so must
// every member of the developer group. The developer group must have at least
// minDeveloperGroupMembers members.
func (r ProjectCreationRequest) Validate(org Organization) error {
	var errs []error
	if n := len(r.DeveloperGroup.Members); n < minDeveloperGroupMembers {
		errs = append(errs, fmt.Errorf("developer group %q has %d member(s), want at least %d", r.DeveloperGroup.Email, n, minDeveloperGroupMembers))
	}
	if !strings.EqualFold(r.Requester.Domain(), org.Domain) {
		errs = append(errs, fmt.Errorf("requester %q is not in the organization domain %q", r.Requester, org.Domain))
	}
	if !strings.EqualFold(r.DeveloperGroup.Email.Domain(), org.Domain) {
		errs = append(errs, fmt.Errorf("developer group %q is not in the organization domain %q", r.DeveloperGroup.Email, org.Domain))
	}
	if !strings.EqualFold(r.DeveloperGroup.Domain, org.Domain) {
		errs = append(errs, fmt.Errorf("developer group domain %q is not the organization domain %q", r.DeveloperGroup.Domain, org.Domain))
	}
	for _, m := range r.DeveloperGroup.Members {
		if !strings.EqualFold(m.Domain, org.Domain) {
			errs = append(errs, fmt.Errorf("developer group member %q is not in the organization domain %q", m.Email, org.Domain))
		}
	}
	return errors.Join(errs...)
}

// Email is a validated email address.
type Email struct {
	value string
}

// NewEmail returns s as an Email, or an error if s is not a bare email address.
//
// It checks the syntax only, not that the address exists.
func NewEmail(s string) (Email, error) {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return Email{}, fmt.Errorf("invalid email address %q: %w", s, err)
	}
	// ParseAddress also accepts `Name <a@b.com>`; reject anything but a bare address.
	if addr.Address != s {
		return Email{}, fmt.Errorf("invalid email address %q: must be a bare address", s)
	}
	return Email{value: addr.Address}, nil
}

func (e Email) String() string { return e.value }

// Domain returns the part after the last "@", or "" for the zero Email.
func (e Email) Domain() string {
	i := strings.LastIndex(e.value, "@")
	if i < 0 {
		return ""
	}
	return e.value[i+1:]
}

// Organization represents the Google Cloud organization this application is set up for.
type Organization struct {
	// ID is the numeric ID, without the "organizations/" prefix.
	ID string
	// Domain is the domain of the organization, such as "example.com".
	Domain string
}

// GoogleGroup represents a Google Group.
type GoogleGroup struct {
	Email   Email
	Domain  string
	Members []GoogleGroupMember
}

// GoogleGroupMember represents a Google Group member. Role is the strongest of
// the roles the member holds.
type GoogleGroupMember struct {
	Email  Email
	Role   string
	Domain string
}

// SameDomain reports whether all of g's members have g's Domain. Domains are
// compared case-insensitively.
func (g *GoogleGroup) SameDomain() bool {
	for _, m := range g.Members {
		if !strings.EqualFold(m.Domain, g.Domain) {
			return false
		}
	}
	return true
}
