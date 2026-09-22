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
