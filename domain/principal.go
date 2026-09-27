package domain

// Principal is an identity that IAM grants roles to: a user, a group or a
// service account, identified by its email.
type Principal struct {
	kind  string
	email Email
}

// NewUserPrincipal returns the principal for a Google account.
func NewUserPrincipal(email Email) Principal { return Principal{kind: "user", email: email} }

// NewGroupPrincipal returns the principal for a Google Group.
func NewGroupPrincipal(email Email) Principal { return Principal{kind: "group", email: email} }

// NewServiceAccountPrincipal returns the principal for a service account.
func NewServiceAccountPrincipal(email Email) Principal {
	return Principal{kind: "serviceAccount", email: email}
}

// Email returns the principal's email.
func (p Principal) Email() Email { return p.email }

// String returns the principal as IAM writes it, such as
// "serviceAccount:sa@project.iam.gserviceaccount.com".
func (p Principal) String() string { return p.kind + ":" + p.email.String() }
