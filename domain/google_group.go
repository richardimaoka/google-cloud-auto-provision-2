package domain

import "slices"

// GoogleGroup represents a Google Group.
type GoogleGroup struct {
	email   Email
	members []GoogleGroupMember
}

// NewGoogleGroup returns a GoogleGroup with the given email and members.
func NewGoogleGroup(email Email, members []GoogleGroupMember) GoogleGroup {
	return GoogleGroup{email: email, members: slices.Clone(members)}
}

func (g GoogleGroup) Email() Email { return g.email }

// Members returns a copy of the members of g.
func (g GoogleGroup) Members() []GoogleGroupMember { return slices.Clone(g.members) }

// GoogleGroupMember represents a Google Group member. Role is the strongest of
// the roles the member holds.
type GoogleGroupMember struct {
	email Email
	role  string
}

// NewGoogleGroupMember returns a GoogleGroupMember with the given email and role.
func NewGoogleGroupMember(email Email, role string) GoogleGroupMember {
	return GoogleGroupMember{email: email, role: role}
}

func (m GoogleGroupMember) Email() Email { return m.email }

func (m GoogleGroupMember) Role() string { return m.role }
