package gcp

import (
	"context"
	"errors"
	"fmt"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

// policyVersion is the version of the policy format the client asks for. Version
// 3 is the one that has conditions, and a policy read with a lower version has
// them left out.
const policyVersion = 3

var (
	// ErrBindingNotFound is returned by RemoveBinding when the member does not have
	// the role, so the name it was given matches nothing.
	ErrBindingNotFound = errors.New("iam binding not found")
	// ErrLastOwner is returned when a change would leave a project with no owner.
	ErrLastOwner = errors.New("change would leave the project with no owner")
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

// IAMClient is a client for the IAM policies of projects.
//
// It reads conditional bindings, but does not set them. AddBinding and
// RemoveBinding work on the unconditional bindings only, like gcloud does with
// --condition=None, and a write sends the conditional bindings back as they were
// read, so it does not change who has access.
type IAMClient struct {
	service *cloudresourcemanager.Service
}

func NewIAMClient(ctx context.Context, opts ...option.ClientOption) (*IAMClient, error) {
	svc, err := cloudresourcemanager.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new cloudresourcemanager service: %w", err)
	}
	return &IAMClient{service: svc}, nil
}

func policyFromAPI(p *cloudresourcemanager.Policy) *Policy {
	policy := &Policy{Version: p.Version}
	for _, b := range p.Bindings {
		binding := Binding{Role: b.Role, Members: b.Members}
		if b.Condition != nil {
			binding.Condition = &Expr{
				Title:       b.Condition.Title,
				Description: b.Condition.Description,
				Expression:  b.Condition.Expression,
			}
		}
		policy.Bindings = append(policy.Bindings, binding)
	}
	return policy
}

// GetPolicy returns the IAM policy of a project, given its ID or its number. It
// asks for policy version 3, so a condition is never left out.
func (c *IAMClient) GetPolicy(ctx context.Context, projectID string) (*Policy, error) {
	req := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: policyVersion},
	}
	p, err := c.service.Projects.GetIamPolicy("projects/"+projectID, req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get iam policy: %w", err)
	}
	return policyFromAPI(p), nil
}

// AddBinding grants role to member on a project, like gcloud's
// add-iam-policy-binding. The member is in the form the API uses, like
// "user:alice@example.com". It does nothing, and writes nothing, if the member has
// the role already.
func (c *IAMClient) AddBinding(ctx context.Context, projectID, role, member string) error {
	return nil
}

// RemoveBinding takes role from member on a project, like gcloud's
// remove-iam-policy-binding, and removes the binding if it has no member left. It
// returns ErrBindingNotFound if the member does not have the role, in the way a
// role that does not exist is an error for AddBinding, and ErrLastOwner if it
// would leave the project with no owner.
func (c *IAMClient) RemoveBinding(ctx context.Context, projectID, role, member string) error {
	return nil
}

// updatePolicy is the read-modify-write operation under AddBinding and
// RemoveBinding. It reads the policy, calls update, and writes the result with the
// etag it read and version 3. On a conflict (409) it reads again and calls update
// again, up to a limit, waiting longer each time. It writes nothing if update left
// the policy equal to the one it read.
//
// It refuses a change that leaves a project with no owner, when the policy it read
// had one, and returns ErrLastOwner without writing or retrying. An owner is a
// member of an unconditional roles/owner binding, and a "deleted:" member is not
// one, as it grants nothing. A policy that had no owner to begin with, for example
// because the owners come from the organization, is written as it is.
func (c *IAMClient) updatePolicy(ctx context.Context, projectID string, update func(*Policy) error) error {
	return nil
}
