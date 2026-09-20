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
