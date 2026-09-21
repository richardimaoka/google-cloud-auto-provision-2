package domain

import "testing"

// email returns s as an Email.
func email(t *testing.T, s string) Email {
	t.Helper()
	e, err := NewEmail(s)
	if err != nil {
		t.Fatalf("NewEmail(%q): %v", s, err)
	}
	return e
}

func TestProjectCreationRequest_Validate(t *testing.T) {
	org, err := NewOrganization("123456789012", "example.com")
	if err != nil {
		t.Fatal(err)
	}

	// Two members in the organization, the fewest a valid group may have.
	twoMembers := []string{"member-1@example.com", "member-2@example.com"}

	tests := []struct {
		name string
		// The request, written as plain strings.
		requester  string
		groupEmail string
		members    []string
		// The number of problems Validate should report.
		wantErrs int
	}{
		{
			name:       "組織のドメインで揃っていて、メンバーが2人いれば有効",
			requester:  "requester@example.com",
			groupEmail: "developer-group@example.com",
			members:    twoMembers,
			wantErrs:   0,
		},
		{
			name:       "開発者グループのメンバーが0人だとエラー1件",
			requester:  "requester@example.com",
			groupEmail: "developer-group@example.com",
			wantErrs:   1,
		},
		{
			name:       "開発者グループのメンバーが1人だけ（2人未満）だとエラー1件",
			requester:  "requester@example.com",
			groupEmail: "developer-group@example.com",
			members:    []string{"member-1@example.com"},
			wantErrs:   1,
		},
		{
			name:       "ドメインの大文字小文字が違っても同じドメインとして扱い、有効",
			requester:  "requester@EXAMPLE.com",
			groupEmail: "developer-group@Example.com",
			members:    []string{"member-1@EXAMPLE.COM", "member-2@Example.com"},
			wantErrs:   0,
		},
		{
			name:       "requesterのドメインが組織と違うとエラー1件",
			requester:  "requester@other.com",
			groupEmail: "developer-group@example.com",
			members:    twoMembers,
			wantErrs:   1,
		},
		{
			name:       "開発者グループのメールアドレスのドメインが組織と違うとエラー1件",
			requester:  "requester@example.com",
			groupEmail: "developer-group@other.com",
			members:    twoMembers,
			wantErrs:   1,
		},
		{
			name:       "メンバー2人のうち1人のドメインが組織と違うとエラー1件",
			requester:  "requester@example.com",
			groupEmail: "developer-group@example.com",
			members:    []string{"member-1@example.com", "outside-member@other.com"},
			wantErrs:   1,
		},
		{
			name:       "違反が重なったときは1件で止めず、すべて報告する（4件）",
			requester:  "requester@other.com",
			groupEmail: "developer-group@other.com",
			members:    []string{"outside-member-1@other.com", "inside-member@example.com", "outside-member-2@other.com"},
			// requester, group email, and two members
			wantErrs: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var members []GoogleGroupMember
			for _, m := range tt.members {
				members = append(members, NewGoogleGroupMember(email(t, m), "MEMBER"))
			}
			group := NewGoogleGroup(email(t, tt.groupEmail), members)
			r, err := NewProjectCreationRequest(email(t, tt.requester), group)
			if err != nil {
				t.Fatal(err)
			}

			err = r.Validate(org)

			if tt.wantErrs == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want %d error(s)", tt.wantErrs)
			}
			// errors.Join returns an error that unwraps to the joined errors.
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok {
				t.Fatalf("Validate() = %T, want an error made by errors.Join", err)
			}
			if got := len(joined.Unwrap()); got != tt.wantErrs {
				t.Errorf("Validate() reported %d error(s), want %d: %v", got, tt.wantErrs, err)
			}
		})
	}
}

func TestNewOrganization(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		domain  string
		wantErr bool
	}{
		{name: "数字のIDとドメインがあれば有効", id: "123456789012", domain: "example.com"},
		{name: "IDが空だとエラー", id: "", domain: "example.com", wantErr: true},
		{name: "IDに数字以外が含まれるとエラー", id: "12345abc", domain: "example.com", wantErr: true},
		{name: "IDにorganizations/の接頭辞が付いているとエラー", id: "organizations/123456789012", domain: "example.com", wantErr: true},
		{name: "ドメインが空だとエラー", id: "123456789012", domain: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, err := NewOrganization(tt.id, tt.domain)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewOrganization(%q, %q) = nil error, want an error", tt.id, tt.domain)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewOrganization(%q, %q): %v", tt.id, tt.domain, err)
			}
			if org.ID() != tt.id || org.Domain() != tt.domain {
				t.Errorf("got ID %q and Domain %q, want %q and %q", org.ID(), org.Domain(), tt.id, tt.domain)
			}
		})
	}
}

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
