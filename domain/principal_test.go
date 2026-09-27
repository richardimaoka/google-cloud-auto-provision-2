package domain

import "testing"

func TestPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		principal Principal
		want      string
	}{
		{name: "ユーザー", principal: NewUserPrincipal(email(t, "alice@example.com")), want: "user:alice@example.com"},
		{name: "グループ", principal: NewGroupPrincipal(email(t, "eng@example.com")), want: "group:eng@example.com"},
		{name: "Service Account", principal: NewServiceAccountPrincipal(email(t, "sa@project.iam.gserviceaccount.com")), want: "serviceAccount:sa@project.iam.gserviceaccount.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.principal.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("同じメールでも種類が違えば別のprincipal", func(t *testing.T) {
		e := email(t, "x@example.com")
		if NewUserPrincipal(e) == NewGroupPrincipal(e) {
			t.Error("NewUserPrincipal(e) == NewGroupPrincipal(e), want them different")
		}
	})
}
