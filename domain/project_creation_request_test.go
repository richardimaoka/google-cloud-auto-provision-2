package domain

import "testing"

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
