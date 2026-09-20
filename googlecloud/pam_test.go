package googlecloud

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/privilegedaccessmanager/apiv1/privilegedaccessmanagerpb"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Unit tests for PAMClient.
//
// They fake the API with a server, like the tests of IAMClient. The client asks for
// enums as numbers ($alt=json;enum-encoding=int), so the fake answers with them.

// fakePAMServer fakes GET /v1/projects/my-project/locations/global/entitlements/my-entitlement
// and records the requests it gets. Any other method or path fails the test: the
// API answers 404 to a path it does not know, which the client takes for an
// entitlement that does not exist.
type fakePAMServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
}

const fakeEntitlementPath = "/v1/projects/my-project/locations/global/entitlements/my-entitlement"

// newFakePAMServer starts a fakePAMServer that answers every request with the
// status and the body.
func newFakePAMServer(t *testing.T, status int, body string) *fakePAMServer {
	t.Helper()

	s := &fakePAMServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		s.mu.Unlock()

		if r.Method != http.MethodGet || r.URL.Path != fakeEntitlementPath {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

// Requests returns the requests received so far, as "METHOD /path", oldest first.
func (s *fakePAMServer) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

// entitlementJSON is the body of a 200 answer with e in it.
func entitlementJSON(t *testing.T, e *privilegedaccessmanagerpb.Entitlement) string {
	t.Helper()

	b, err := protojson.MarshalOptions{UseEnumNumbers: true}.Marshal(e)
	if err != nil {
		t.Fatalf("marshal entitlement: %v", err)
	}
	return string(b)
}

func newTestPAMClient(t *testing.T, endpoint string) *PAMClient {
	t.Helper()

	client, err := NewPAMClient(context.Background(), option.WithEndpoint(endpoint), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewPAMClient: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestPAMClient_GetEntitlement(t *testing.T) {
	t.Run("エンタイトルメントの全項目を変換する", func(t *testing.T) {
		srv := newFakePAMServer(t, http.StatusOK, entitlementJSON(t, &privilegedaccessmanagerpb.Entitlement{
			Name: "projects/my-project/locations/global/entitlements/my-entitlement",
			EligibleUsers: []*privilegedaccessmanagerpb.AccessControlEntry{
				{Principals: []string{"user:alice@example.com", "group:group-1@example.com"}},
				{Principals: []string{"user:bob@example.com"}},
			},
			ApprovalWorkflow: &privilegedaccessmanagerpb.ApprovalWorkflow{
				ApprovalWorkflow: &privilegedaccessmanagerpb.ApprovalWorkflow_ManualApprovals{
					ManualApprovals: &privilegedaccessmanagerpb.ManualApprovals{
						RequireApproverJustification: true,
						Steps: []*privilegedaccessmanagerpb.ManualApprovals_Step{
							{
								Approvers:       []*privilegedaccessmanagerpb.AccessControlEntry{{Principals: []string{"user:carol@example.com"}}},
								ApprovalsNeeded: 1,
							},
						},
					},
				},
			},
			PrivilegedAccess: &privilegedaccessmanagerpb.PrivilegedAccess{
				AccessType: &privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_{
					GcpIamAccess: &privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess{
						ResourceType: "cloudresourcemanager.googleapis.com/Project",
						Resource:     "//cloudresourcemanager.googleapis.com/projects/my-project",
						RoleBindings: []*privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_RoleBinding{
							{Role: "roles/viewer"},
							{Role: "roles/storage.admin", ConditionExpression: `resource.type == "storage.googleapis.com/Bucket"`},
						},
					},
				},
			},
			MaxRequestDuration: durationpb.New(2 * time.Hour),
			RequesterJustificationConfig: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig{
				JustificationType: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_Unstructured_{
					Unstructured: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_Unstructured{},
				},
			},
			State: privilegedaccessmanagerpb.Entitlement_AVAILABLE,
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if err != nil {
			t.Fatalf("GetEntitlement() = %v, want nil error", err)
		}

		want := &Entitlement{
			ID:           "my-entitlement",
			Name:         "projects/my-project/locations/global/entitlements/my-entitlement",
			Resource:     "//cloudresourcemanager.googleapis.com/projects/my-project",
			ResourceType: "cloudresourcemanager.googleapis.com/Project",
			Roles: []EntitlementRole{
				{Role: "roles/viewer"},
				{Role: "roles/storage.admin", ConditionExpression: `resource.type == "storage.googleapis.com/Bucket"`},
			},
			EligibleUsers: []string{"user:alice@example.com", "group:group-1@example.com", "user:bob@example.com"},
			ApprovalSteps: []ApprovalStep{
				{Approvers: []string{"user:carol@example.com"}, ApprovalsNeeded: 1},
			},
			RequireApproverJustification: true,
			MaxRequestDuration:           2 * time.Hour,
			RequireJustification:         true,
			State:                        "AVAILABLE",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetEntitlement() = %+v, want %+v", got, want)
		}
		if reqs := srv.Requests(); len(reqs) != 1 {
			t.Errorf("got requests %v, want 1", reqs)
		}
	})

	t.Run("承認と理由が不要なエンタイトルメントは空の値になる", func(t *testing.T) {
		srv := newFakePAMServer(t, http.StatusOK, entitlementJSON(t, &privilegedaccessmanagerpb.Entitlement{
			Name:  "projects/my-project/locations/global/entitlements/my-entitlement",
			State: privilegedaccessmanagerpb.Entitlement_CREATING,
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if err != nil {
			t.Fatalf("GetEntitlement() = %v, want nil error", err)
		}
		if len(got.ApprovalSteps) != 0 || got.RequireApproverJustification || got.RequireJustification || len(got.Roles) != 0 {
			t.Errorf("GetEntitlement() = %+v, want no approvals, no justification and no roles", got)
		}
		if got.State != "CREATING" {
			t.Errorf("State = %q, want CREATING", got.State)
		}
	})

	t.Run("404はErrEntitlementNotFoundを返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.StatusNotFound, `{"error":{"code":404,"message":"Entitlement not found","status":"NOT_FOUND"}}`)
		client := newTestPAMClient(t, srv.URL)

		_, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if !errors.Is(err, ErrEntitlementNotFound) {
			t.Fatalf("GetEntitlement() = %v, want ErrEntitlementNotFound", err)
		}
		// The error of the API stays in the chain, so the message of the API is not lost.
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusNotFound {
			t.Errorf("errors.As(googleapi.Error) = %v, want code 404", gerr)
		}
	})

	t.Run("404以外のエラーはErrEntitlementNotFoundにしない", func(t *testing.T) {
		srv := newFakePAMServer(t, http.StatusForbidden, `{"error":{"code":403,"message":"Permission denied","status":"PERMISSION_DENIED"}}`)
		client := newTestPAMClient(t, srv.URL)

		_, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if err == nil || errors.Is(err, ErrEntitlementNotFound) {
			t.Fatalf("GetEntitlement() = %v, want an error that is not ErrEntitlementNotFound", err)
		}
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusForbidden {
			t.Errorf("errors.As(googleapi.Error) = %v, want code 403", gerr)
		}
	})
}
