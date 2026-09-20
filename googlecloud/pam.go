package googlecloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	privilegedaccessmanager "cloud.google.com/go/privilegedaccessmanager/apiv1"
	"cloud.google.com/go/privilegedaccessmanager/apiv1/privilegedaccessmanagerpb"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// pamLocation is the location of the entitlements of a project. PAM keeps them
// in "global".
const pamLocation = "global"

// ErrEntitlementNotFound is returned when the entitlement does not exist.
var ErrEntitlementNotFound = errors.New("entitlement not found")

// Entitlement represents a PAM entitlement: a grant of roles on a resource that
// its eligible users can request for a limited time. Its fields are named after
// the ones of the Entitlement of the privilegedaccessmanager API, and the ones
// that a project does not use are left out.
type Entitlement struct {
	// ID is the last part of Name.
	ID string
	// Name is "projects/<project>/locations/global/entitlements/<ID>".
	Name string
	// Resource is the resource the roles are granted on, in the full form the API
	// uses, like "//cloudresourcemanager.googleapis.com/projects/my-project".
	Resource string
	// ResourceType is the type of Resource, like
	// "cloudresourcemanager.googleapis.com/Project".
	ResourceType string
	// Roles are granted to whoever has a grant of the entitlement.
	Roles []EntitlementRole
	// EligibleUsers can request a grant, in the form IAMClient uses for members,
	// like "user:alice@example.com" or "group:eng@example.com".
	EligibleUsers []string
	// ApprovalSteps are the approvals a grant needs, in order. It is empty if a
	// grant needs none.
	ApprovalSteps []ApprovalStep
	// RequireApproverJustification is whether an approver must give a reason. It
	// has no meaning without ApprovalSteps.
	RequireApproverJustification bool
	// MaxRequestDuration is the longest a grant can last.
	MaxRequestDuration time.Duration
	// RequireJustification is whether a requester must give a reason.
	RequireJustification bool
	// State is the name of the state of the entitlement, like "AVAILABLE".
	State string
}

// EntitlementRole represents a role of an entitlement, with the condition it is
// granted under, if it has one.
type EntitlementRole struct {
	Role string
	// ConditionExpression is an expression in the Common Expression Language, or
	// empty if the role has no condition. Unlike Binding in iam.go it has no title
	// or description: the API has none for an entitlement.
	ConditionExpression string
}

// ApprovalStep represents a step of approval: ApprovalsNeeded of Approvers must
// approve a grant before it goes to the next step.
type ApprovalStep struct {
	// Approvers are principals, in the same form as EligibleUsers.
	Approvers       []string
	ApprovalsNeeded int32
}

// PAMClient is a client for Privileged Access Manager, the API that grants roles
// for a limited time.
//
// It works with the entitlements of projects. It does not request or approve
// grants: they are made by the users the entitlement is for, not by a
// provisioning process.
type PAMClient struct {
	client *privilegedaccessmanager.Client
}

// NewPAMClient makes a PAMClient. It uses the REST client, because that is the one
// of the other clients, and so the tests can fake the API in the same way. The
// module is not part of google.golang.org/api, which the other clients use.
func NewPAMClient(ctx context.Context, opts ...option.ClientOption) (*PAMClient, error) {
	c, err := privilegedaccessmanager.NewRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new privilegedaccessmanager client: %w", err)
	}
	return &PAMClient{client: c}, nil
}

// Close releases what the client holds. The other clients have nothing to close.
func (c *PAMClient) Close() error {
	return c.client.Close()
}

func entitlementName(projectID, entitlementID string) string {
	return fmt.Sprintf("projects/%s/locations/%s/entitlements/%s", projectID, pamLocation, entitlementID)
}

// principals joins the principals of the entries into one list.
func principals(entries []*privilegedaccessmanagerpb.AccessControlEntry) []string {
	var all []string
	for _, e := range entries {
		all = append(all, e.GetPrincipals()...)
	}
	return all
}

func entitlementFromAPI(e *privilegedaccessmanagerpb.Entitlement) *Entitlement {
	ent := &Entitlement{
		ID:                   path.Base(e.GetName()),
		Name:                 e.GetName(),
		EligibleUsers:        principals(e.GetEligibleUsers()),
		MaxRequestDuration:   e.GetMaxRequestDuration().AsDuration(),
		RequireJustification: e.GetRequesterJustificationConfig().GetUnstructured() != nil,
		State:                e.GetState().String(),
	}
	if gcp := e.GetPrivilegedAccess().GetGcpIamAccess(); gcp != nil {
		ent.Resource = gcp.GetResource()
		ent.ResourceType = gcp.GetResourceType()
		for _, rb := range gcp.GetRoleBindings() {
			ent.Roles = append(ent.Roles, EntitlementRole{
				Role:                rb.GetRole(),
				ConditionExpression: rb.GetConditionExpression(),
			})
		}
	}
	if manual := e.GetApprovalWorkflow().GetManualApprovals(); manual != nil {
		ent.RequireApproverJustification = manual.GetRequireApproverJustification()
		for _, s := range manual.GetSteps() {
			ent.ApprovalSteps = append(ent.ApprovalSteps, ApprovalStep{
				Approvers:       principals(s.GetApprovers()),
				ApprovalsNeeded: s.GetApprovalsNeeded(),
			})
		}
	}
	return ent
}

// GetEntitlement returns an entitlement of a project. It returns
// ErrEntitlementNotFound if the project has none with that ID.
func (c *PAMClient) GetEntitlement(ctx context.Context, projectID, entitlementID string) (*Entitlement, error) {
	e, err := c.client.GetEntitlement(ctx, &privilegedaccessmanagerpb.GetEntitlementRequest{
		Name: entitlementName(projectID, entitlementID),
	})
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return nil, fmt.Errorf("get entitlement: %w: %w", ErrEntitlementNotFound, err)
		}
		return nil, fmt.Errorf("get entitlement: %w", err)
	}
	return entitlementFromAPI(e), nil
}

// isHTTPStatus reports whether err is an API error with the HTTP status code. It
// looks for the error of google.golang.org/api that the REST client wraps.
func isHTTPStatus(err error, code int) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == code
}
