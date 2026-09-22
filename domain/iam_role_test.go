package domain

import (
	"strings"
	"testing"
)

func TestNewIAMRole(t *testing.T) {
	validCustomID := strings.Repeat("a", 64)
	for _, tt := range []struct {
		name    string
		role    string
		wantErr bool
	}{
		{name: "基本ロール", role: "roles/owner"},
		{name: "定義済みロール", role: "roles/iam.serviceAccountTokenCreator"},
		{name: "プロジェクトのカスタムロール", role: "projects/my-project/roles/custom_role.1"},
		{name: "組織のカスタムロール", role: "organizations/123456789012/roles/" + validCustomID},
		{name: "空", role: "", wantErr: true},
		{name: "空の定義済みロールID", role: "roles/", wantErr: true},
		{name: "余分なパス要素", role: "roles/iam/viewer", wantErr: true},
		{name: "短すぎるカスタムロールID", role: "projects/my-project/roles/ab", wantErr: true},
		{name: "長すぎるカスタムロールID", role: "projects/my-project/roles/" + strings.Repeat("a", 65), wantErr: true},
		{name: "不正なカスタムロールID", role: "projects/my-project/roles/a-b", wantErr: true},
		{name: "不正なプロジェクトID", role: "projects/My_Project/roles/custom", wantErr: true},
		{name: "数値以外の組織ID", role: "organizations/example/roles/custom", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewIAMRole(tt.role)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NewIAMRole() = nil error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewIAMRole() error = %v", err)
			}
			if got.String() != tt.role {
				t.Errorf("String() = %q, want %q", got.String(), tt.role)
			}
		})
	}
}
