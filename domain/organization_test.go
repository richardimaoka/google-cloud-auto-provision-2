package domain

import "testing"

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
