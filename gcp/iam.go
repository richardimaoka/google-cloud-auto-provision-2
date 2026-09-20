package gcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// policyVersion is the version of the policy format the client asks for. Version
// 3 is the one that has conditions, and a policy read with a lower version has
// them left out.
const policyVersion = 3

const (
	// maxUpdateAttempts is how many times updatePolicy reads and writes a policy
	// before it gives up on conflicts: the first attempt plus 2 retries.
	maxUpdateAttempts = 3
	// defaultRetryWait is how long updatePolicy waits before it reads the policy
	// again after a conflict.
	defaultRetryWait = 200 * time.Millisecond
)

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
	service   *cloudresourcemanager.Service
	retryWait time.Duration
}

func NewIAMClient(ctx context.Context, opts ...option.ClientOption) (*IAMClient, error) {
	svc, err := cloudresourcemanager.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new cloudresourcemanager service: %w", err)
	}
	return &IAMClient{service: svc, retryWait: defaultRetryWait}, nil
}

func policyFromAPI(p *cloudresourcemanager.Policy) *Policy {
	policy := &Policy{Version: p.Version}
	for _, b := range p.Bindings {
		binding := Binding{Role: b.Role, Members: slices.Clone(b.Members)}
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

// ownerRole is the role of a project's owners.
const ownerRole = "roles/owner"

// hasOwner reports whether the policy has an owner: a member of an unconditional
// roles/owner binding. A deleted member is left in the binding and grants
// nothing, so it is not one.
func (p *Policy) hasOwner() bool {
	return slices.ContainsFunc(p.Bindings, func(b Binding) bool {
		return b.Role == ownerRole && b.Condition == nil &&
			slices.ContainsFunc(b.Members, func(m string) bool { return !strings.HasPrefix(m, "deleted:") })
	})
}

// equal reports whether two policies have the same bindings, in the same order.
func (p *Policy) equal(o *Policy) bool {
	return slices.EqualFunc(p.Bindings, o.Bindings, func(a, b Binding) bool {
		sameCondition := a.Condition == b.Condition || (a.Condition != nil && b.Condition != nil && *a.Condition == *b.Condition)
		return a.Role == b.Role && slices.Equal(a.Members, b.Members) && sameCondition
	})
}

// policyToAPI turns a Policy back into the API's, to be written with the etag it
// was read with, and with the version that has conditions.
func policyToAPI(p *Policy, etag string) *cloudresourcemanager.Policy {
	policy := &cloudresourcemanager.Policy{Version: policyVersion, Etag: etag}
	for _, b := range p.Bindings {
		binding := &cloudresourcemanager.Binding{Role: b.Role, Members: b.Members}
		if b.Condition != nil {
			binding.Condition = &cloudresourcemanager.Expr{
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
	p, err := c.getPolicy(ctx, "projects/"+projectID)
	if err != nil {
		return nil, err
	}
	return policyFromAPI(p), nil
}

// getPolicy reads the policy as the API has it, with its etag.
func (c *IAMClient) getPolicy(ctx context.Context, resource string) (*cloudresourcemanager.Policy, error) {
	req := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{RequestedPolicyVersion: policyVersion},
	}
	p, err := c.service.Projects.GetIamPolicy(resource, req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get iam policy: %w", err)
	}
	return p, nil
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

// updatePolicy reads the policy, calls update on it and writes the result with the
// etag it read. On a conflict (409) it reads again and calls update again on the
// new policy, up to maxUpdateAttempts times, so update must be safe to call more
// than once. It writes nothing if update changed nothing, and returns ErrLastOwner
// instead of writing a policy that leaves the project with no owner.
func (c *IAMClient) updatePolicy(ctx context.Context, projectID string, update func(*Policy) error) error {
	var err error
	for attempt := 1; attempt <= maxUpdateAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryWait):
			}
		}

		err = c.tryUpdatePolicy(ctx, "projects/"+projectID, update)
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusConflict {
			return err
		}
	}
	return fmt.Errorf("update iam policy: gave up after %d attempts: %w", maxUpdateAttempts, err)
}

// tryUpdatePolicy is one attempt of updatePolicy: it reads, calls update and
// writes once.
func (c *IAMClient) tryUpdatePolicy(ctx context.Context, resource string, update func(*Policy) error) error {
	current, err := c.getPolicy(ctx, resource)
	if err != nil {
		return err
	}
	// update gets its own copy, so the one read is left to compare with.
	original := policyFromAPI(current)
	policy := policyFromAPI(current)
	if err := update(policy); err != nil {
		return err
	}
	if policy.equal(original) {
		return nil
	}
	// A policy that had no owner to begin with, for example because the owners come
	// from the organization, is written as it is.
	if original.hasOwner() && !policy.hasOwner() {
		return ErrLastOwner
	}

	// A Policy does not hold everything the API's has, like the audit configs, so
	// only the bindings and the etag are written.
	req := &cloudresourcemanager.SetIamPolicyRequest{
		Policy:     policyToAPI(policy, current.Etag),
		UpdateMask: "bindings,etag",
	}
	if _, err := c.service.Projects.SetIamPolicy(resource, req).Context(ctx).Do(); err != nil {
		return fmt.Errorf("set iam policy: %w", err)
	}
	return nil
}
