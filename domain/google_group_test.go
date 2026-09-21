package domain

import "testing"

func TestGoogleGroup_membersAreCopied(t *testing.T) {
	member1 := NewGoogleGroupMember(email(t, "member-1@example.com"), "MEMBER")
	member2 := NewGoogleGroupMember(email(t, "member-2@example.com"), "MEMBER")
	other := NewGoogleGroupMember(email(t, "outside-member@other.com"), "MEMBER")

	input := []GoogleGroupMember{member1, member2}
	g := NewGoogleGroup(email(t, "developer-group@example.com"), input)

	// Changing the slice passed to NewGoogleGroup must not change the group.
	input[0] = other
	if got := g.Members()[0]; got != member1 {
		t.Errorf("the group followed a change to the input slice: first member is %v", got.Email())
	}

	// Changing the slice returned by Members must not change the group either.
	g.Members()[1] = other
	if got := g.Members()[1]; got != member2 {
		t.Errorf("the group followed a change to the returned slice: second member is %v", got.Email())
	}
}
