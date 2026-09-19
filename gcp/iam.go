package gcp

import (
	"context"

	"google.golang.org/api/option"
)

// Policy represents an IAM policy. Its fields are named after the ones of the
// Policy of the cloudresourcemanager API, without Etag and AuditConfigs.
type Policy struct {
	// Bindings associate members with roles. One role can be in more than one
	// binding, for example one without a Condition and one with.
	Bindings []Binding
	// Version is the policy format the API answered with: 3 if a binding has a
	// Condition, and 1 if none has, although the client always asks for version
	// 3, so a Condition is never left out.
	Version int64
}

// Binding represents an IAM role binding: it grants Role to Members, and only
// while Condition holds, if there is one.
type Binding struct {
	Role string
	// Members are the principals, in the form the API uses: "user:a@example.com",
	// "serviceAccount:...", "group:...", "domain:example.com", and so on.
	Members []string
	// Condition is nil if the role is granted without a condition.
	Condition *Expr
}

// Expr represents the condition of a binding: an expression in the Common
// Expression Language, with a title and a description.
type Expr struct {
	Title       string
	Description string
	Expression  string
}

// IAMClient is a client for IAM policies.
type IAMClient struct{}

func NewIAMClient(ctx context.Context, opts ...option.ClientOption) (*IAMClient, error) {
	return nil, nil
}

// GetPolicy returns the IAM policy of a resource, like "projects/my-project",
// "folders/123" or "organizations/123".
func (c *IAMClient) GetPolicy(ctx context.Context, resource string) (*Policy, error) {
	return nil, nil
}
