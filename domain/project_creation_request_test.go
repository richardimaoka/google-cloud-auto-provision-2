package domain

import "testing"

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
		{name: "組織のドメインで揃っていて、メンバーが2人いれば有効", requester: "requester@example.com", groupEmail: "developer-group@example.com", members: twoMembers},
		{name: "開発者グループのメンバーが0人だとエラー1件", requester: "requester@example.com", groupEmail: "developer-group@example.com", wantErrs: 1},
		{name: "開発者グループのメンバーが1人だけ（2人未満）だとエラー1件", requester: "requester@example.com", groupEmail: "developer-group@example.com", members: []string{"member-1@example.com"}, wantErrs: 1},
		{name: "ドメインの大文字小文字が違っても同じドメインとして扱い、有効", requester: "requester@EXAMPLE.com", groupEmail: "developer-group@Example.com", members: []string{"member-1@EXAMPLE.COM", "member-2@Example.com"}},
		{name: "requesterのドメインが組織と違うとエラー1件", requester: "requester@other.com", groupEmail: "developer-group@example.com", members: twoMembers, wantErrs: 1},
		{name: "開発者グループのメールアドレスのドメインが組織と違うとエラー1件", requester: "requester@example.com", groupEmail: "developer-group@other.com", members: twoMembers, wantErrs: 1},
		{name: "メンバー2人のうち1人のドメインが組織と違うとエラー1件", requester: "requester@example.com", groupEmail: "developer-group@example.com", members: []string{"member-1@example.com", "outside-member@other.com"}, wantErrs: 1},
		{name: "違反が重なったときは1件で止めず、すべて報告する（4件）", requester: "requester@other.com", groupEmail: "developer-group@other.com", members: []string{"outside-member-1@other.com", "inside-member@example.com", "outside-member-2@other.com"}, wantErrs: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var members []GoogleGroupMember
			for _, m := range tt.members {
				members = append(members, NewGoogleGroupMember(email(t, m), "MEMBER"))
			}
			group := NewGoogleGroup(email(t, tt.groupEmail), members)
			r, err := NewProjectCreationRequest("Example Project", email(t, tt.requester), group)
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

func TestNewProjectCreationRequest(t *testing.T) {
	requester := email(t, "requester@example.com")
	group := NewGoogleGroup(email(t, "developer-group@example.com"), nil)

	tests := []struct {
		name        string
		displayName string
		requester   Email
		group       GoogleGroup
		wantErr     bool
	}{
		{name: "有効な表示名を保持する", displayName: "Example Project", requester: requester, group: group},
		{name: "最短の4文字", displayName: "Abc1", requester: requester, group: group},
		{name: "最長の30文字", displayName: "123456789012345678901234567890", requester: requester, group: group},
		{name: "許可される記号", displayName: "Dev's \"App\"! 1", requester: requester, group: group},
		{name: "空文字", displayName: "", requester: requester, group: group, wantErr: true},
		{name: "3文字", displayName: "Ab1", requester: requester, group: group, wantErr: true},
		{name: "31文字", displayName: "1234567890123456789012345678901", requester: requester, group: group, wantErr: true},
		{name: "許可されないアンダースコア", displayName: "my_project", requester: requester, group: group, wantErr: true},
		{name: "許可されないスラッシュ", displayName: "my/project", requester: requester, group: group, wantErr: true},
		{name: "許可されない日本語", displayName: "開発プロジェクト", requester: requester, group: group, wantErr: true},
		{name: "ハイフンで終わる", displayName: "Project-", requester: requester, group: group, wantErr: true},
		{name: "空白で終わる", displayName: "Project ", requester: requester, group: group, wantErr: true},
		{name: "感嘆符で終わる", displayName: "Project!", requester: requester, group: group, wantErr: true},
		{name: "requesterが空", displayName: "Example Project", group: group, wantErr: true},
		{name: "開発者グループのメールアドレスが空", displayName: "Example Project", requester: requester, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProjectCreationRequest(tt.displayName, tt.requester, tt.group)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NewProjectCreationRequest() = nil error, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProjectCreationRequest() = %v, want nil error", err)
			}
			if got.DisplayName != tt.displayName {
				t.Errorf("DisplayName = %q, want %q", got.DisplayName, tt.displayName)
			}
		})
	}
}
