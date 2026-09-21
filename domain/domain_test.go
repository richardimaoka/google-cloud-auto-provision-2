package domain

import "testing"

// email returns s as an Email, or the zero Email if s is empty.
func email(t *testing.T, s string) Email {
	t.Helper()
	if s == "" {
		return Email{}
	}
	e, err := NewEmail(s)
	if err != nil {
		t.Fatalf("NewEmail(%q): %v", s, err)
	}
	return e
}

func TestProjectCreationRequest_Validate(t *testing.T) {
	org := Organization{ID: "123456789012", Domain: "example.com"}

	// Two members in the organization, the fewest a valid group may have.
	twoMembers := []string{"member-1@example.com", "member-2@example.com"}

	tests := []struct {
		name string
		// The request, written as plain strings. An empty email is the zero Email.
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
			name:       "requesterが未設定（ゼロ値のEmail）だとエラー1件",
			requester:  "",
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
				members = append(members, GoogleGroupMember{Email: email(t, m)})
			}
			r := ProjectCreationRequest{
				Requester: email(t, tt.requester),
				DeveloperGroup: GoogleGroup{
					Email:   email(t, tt.groupEmail),
					Members: members,
				},
			}

			err := r.Validate(org)

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
