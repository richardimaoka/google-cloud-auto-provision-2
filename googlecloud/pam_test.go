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

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"cloud.google.com/go/privilegedaccessmanager/apiv1/privilegedaccessmanagerpb"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Unit tests for PAMClient.
//
// They fake the API with a server, like the tests of IAMClient. The client asks for
// enums as numbers ($alt=json;enum-encoding=int), so the fake answers with them.
//
// The cases of GetEntitlement are skipped until they are written.

// fakePAMServer fakes one method and path of the API and records the requests it
// gets. Any other method or path fails the test: the API answers 404 to a path it
// does not know, which the client can take for an entitlement that does not exist.
type fakePAMServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
}

const (
	fakeEntitlementName = "projects/my-project/locations/global/entitlements/my-entitlement"
	fakeEntitlementPath = "/v1/" + fakeEntitlementName
	fakeOnboardingPath  = "/v1/projects/my-project/locations/global:checkOnboardingStatus"
)

// newFakePAMServer starts a fakePAMServer that answers every request for the method
// and the path with the status and the body.
func newFakePAMServer(t *testing.T, method, path string, status int, body string) *fakePAMServer {
	t.Helper()

	s := &fakePAMServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		s.mu.Unlock()

		if r.Method != method || r.URL.Path != path {
			t.Errorf("unexpected request %s %s, want %s %s", r.Method, r.URL.Path, method, path)
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

// protoJSON is the body of a 200 answer with m in it.
func protoJSON(t *testing.T, m proto.Message) string {
	t.Helper()

	b, err := protojson.MarshalOptions{UseEnumNumbers: true}.Marshal(m)
	if err != nil {
		t.Fatalf("marshal %T: %v", m, err)
	}
	return string(b)
}

// onboardingFinding is a finding of the type IAMAccessDenied.
func onboardingFinding(permissions ...string) *privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding {
	return &privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding{
		FindingType: &privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding_IamAccessDenied{
			IamAccessDenied: &privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding_IAMAccessDenied{
				MissingPermissions: permissions,
			},
		},
	}
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

// accessControlEntry is an entry of principals, like the ones of eligible users and
// approvers.
func accessControlEntry(principals ...string) *privilegedaccessmanagerpb.AccessControlEntry {
	return &privilegedaccessmanagerpb.AccessControlEntry{Principals: principals}
}

// The helpers below build the parts of an Entitlement that the API wraps in a
// oneof, so that a case shows the fields it is about and not the wrapper types.

// manualApprovals is an approval workflow of manual approvals.
func manualApprovals(requireApproverJustification bool, steps ...*privilegedaccessmanagerpb.ManualApprovals_Step) *privilegedaccessmanagerpb.ApprovalWorkflow {
	return &privilegedaccessmanagerpb.ApprovalWorkflow{
		ApprovalWorkflow: &privilegedaccessmanagerpb.ApprovalWorkflow_ManualApprovals{
			ManualApprovals: &privilegedaccessmanagerpb.ManualApprovals{
				RequireApproverJustification: requireApproverJustification,
				Steps:                        steps,
			},
		},
	}
}

// approvalStep is a step of manual approvals: approvalsNeeded of the approvers must
// approve.
func approvalStep(approvalsNeeded int32, approvers ...*privilegedaccessmanagerpb.AccessControlEntry) *privilegedaccessmanagerpb.ManualApprovals_Step {
	return &privilegedaccessmanagerpb.ManualApprovals_Step{Approvers: approvers, ApprovalsNeeded: approvalsNeeded}
}

// gcpIamAccess is a privileged access to grant roles on a resource.
func gcpIamAccess(resourceType, resource string, roles ...*privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_RoleBinding) *privilegedaccessmanagerpb.PrivilegedAccess {
	return &privilegedaccessmanagerpb.PrivilegedAccess{
		AccessType: &privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_{
			GcpIamAccess: &privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess{
				ResourceType: resourceType,
				Resource:     resource,
				RoleBindings: roles,
			},
		},
	}
}

// roleBinding is a role of gcpIamAccess. The condition expression is empty for a
// role without a condition.
func roleBinding(role, conditionExpression string) *privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_RoleBinding {
	return &privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_RoleBinding{Role: role, ConditionExpression: conditionExpression}
}

// requiredJustification is a config that makes a requester give a justification.
func requiredJustification() *privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig {
	return &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig{
		JustificationType: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_Unstructured_{
			Unstructured: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_Unstructured{},
		},
	}
}

// optionalJustification is a config that lets a requester give a justification or not.
func optionalJustification() *privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig {
	return &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig{
		JustificationType: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_NotMandatory_{
			NotMandatory: &privilegedaccessmanagerpb.Entitlement_RequesterJustificationConfig_NotMandatory{},
		},
	}
}

// getFakeEntitlement returns what GetEntitlement returns when the API answers with
// e. It sets the name of e, so a case has only the fields it is about.
func getFakeEntitlement(t *testing.T, e *privilegedaccessmanagerpb.Entitlement) *Entitlement {
	t.Helper()

	e.Name = fakeEntitlementName
	srv := newFakePAMServer(t, http.MethodGet, fakeEntitlementPath, http.StatusOK, protoJSON(t, e))
	client := newTestPAMClient(t, srv.URL)

	got, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
	if err != nil {
		t.Fatalf("GetEntitlement() = %v, want nil error", err)
	}
	return got
}

const fakeCreateEntitlementPath = "/v1/projects/my-project/locations/global/entitlements"
const fakeOperationName = "projects/my-project/locations/global/operations/op-1"
const fakeOperationPath = "/v1/" + fakeOperationName

type fakePAMResponse struct {
	status int
	body   string
}

// fakePAMCreateServer records a CreateEntitlement request and serves the create
// operation followed by its scripted polls.
type fakePAMCreateServer struct {
	*httptest.Server
	mu           sync.Mutex
	requests     []string
	query        []string
	entitlements []*privilegedaccessmanagerpb.Entitlement
	creates      []fakePAMResponse
	polls        []fakePAMResponse
}

func newFakePAMCreateServer(t *testing.T, creates, polls []fakePAMResponse) *fakePAMCreateServer {
	t.Helper()
	s := &fakePAMCreateServer{creates: creates, polls: polls}
	write := func(w http.ResponseWriter, responses []fakePAMResponse, n int) {
		if n >= len(responses) {
			t.Errorf("unexpected request %d, only %d scripted", n+1, len(responses))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := responses[n]
		w.WriteHeader(resp.status)
		io.WriteString(w, resp.body)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fakeCreateEntitlementPath:
			var e privilegedaccessmanagerpb.Entitlement
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(mustReadAll(t, r.Body), &e); err != nil {
				t.Errorf("decode create request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			s.entitlements = append(s.entitlements, &e)
			s.query = append(s.query, r.URL.Query().Get("entitlementId"))
			write(w, s.creates, len(s.entitlements)-1)
		case r.Method == http.MethodGet && r.URL.Path == fakeOperationPath:
			poll := 0
			for _, request := range s.requests {
				if request == http.MethodGet+" "+fakeOperationPath {
					poll++
				}
			}
			write(w, s.polls, poll-1)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// equalProtos compares messages with proto.Equal, because reflect.DeepEqual also
// compares the internal state of the messages.
func equalProtos[T proto.Message](got, want []T) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !proto.Equal(got[i], want[i]) {
			return false
		}
	}
	return true
}

func mustReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	return b
}

func (s *fakePAMCreateServer) entitlement(t *testing.T) *privilegedaccessmanagerpb.Entitlement {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entitlements) != 1 {
		t.Fatalf("create requests = %d, want 1", len(s.entitlements))
	}
	return s.entitlements[0]
}

func operationJSON(t *testing.T, done bool, entitlement *privilegedaccessmanagerpb.Entitlement, operationError *statuspb.Status) string {
	t.Helper()
	op := &longrunningpb.Operation{Name: fakeOperationName, Done: done}
	if entitlement != nil {
		response, err := anypb.New(entitlement)
		if err != nil {
			t.Fatal(err)
		}
		op.Result = &longrunningpb.Operation_Response{Response: response}
	}
	if operationError != nil {
		op.Result = &longrunningpb.Operation_Error{Error: operationError}
	}
	return protoJSON(t, op)
}

func successfulCreate(t *testing.T, input *Entitlement) (*Entitlement, *fakePAMCreateServer) {
	t.Helper()
	response := &privilegedaccessmanagerpb.Entitlement{Name: fakeEntitlementName}
	srv := newFakePAMCreateServer(t, []fakePAMResponse{{status: http.StatusOK, body: operationJSON(t, true, response, nil)}}, nil)
	client := newTestPAMClient(t, srv.URL)
	client.pollInterval = 0
	got, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", input)
	if err != nil {
		t.Fatalf("CreateEntitlement() = %v, want nil error", err)
	}
	return got, srv
}

func TestPAMClient_GetEntitlement(t *testing.T) {
	t.Run("globalリージョンのエンタイトルメントを1回のGETで取得する", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeEntitlementPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.Entitlement{
			Name: fakeEntitlementName,
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if err != nil {
			t.Fatalf("GetEntitlement() = %v, want nil error", err)
		}
		if got.Name != fakeEntitlementName {
			t.Errorf("Name = %q, want %q", got.Name, fakeEntitlementName)
		}
		if want := []string{"GET " + fakeEntitlementPath}; !reflect.DeepEqual(srv.Requests(), want) {
			t.Errorf("got requests %v, want %v", srv.Requests(), want)
		}
	})

	t.Run("リソースとリソースタイプと、条件式付きのロールを返す", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			PrivilegedAccess: gcpIamAccess(
				"cloudresourcemanager.googleapis.com/Project",
				"//cloudresourcemanager.googleapis.com/projects/my-project",
				roleBinding("roles/viewer", ""),
				roleBinding("roles/storage.admin", `resource.type == "storage.googleapis.com/Bucket"`),
			),
		})

		if want := "//cloudresourcemanager.googleapis.com/projects/my-project"; got.Resource != want {
			t.Errorf("Resource = %q, want %q", got.Resource, want)
		}
		if want := "cloudresourcemanager.googleapis.com/Project"; got.ResourceType != want {
			t.Errorf("ResourceType = %q, want %q", got.ResourceType, want)
		}
		// The role without a condition has an empty ConditionExpression.
		want := []EntitlementRole{
			{Role: "roles/viewer"},
			{Role: "roles/storage.admin", ConditionExpression: `resource.type == "storage.googleapis.com/Bucket"`},
		}
		if !reflect.DeepEqual(got.Roles, want) {
			t.Errorf("Roles = %+v, want %+v", got.Roles, want)
		}
	})

	t.Run("対象ユーザーは複数のエントリをまたいで1つのリストにまとめる", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			EligibleUsers: []*privilegedaccessmanagerpb.AccessControlEntry{
				accessControlEntry("user:alice@example.com", "group:group-1@example.com"),
				accessControlEntry("user:bob@example.com"),
			},
		})

		want := []string{"user:alice@example.com", "group:group-1@example.com", "user:bob@example.com"}
		if !reflect.DeepEqual(got.EligibleUsers, want) {
			t.Errorf("EligibleUsers = %v, want %v", got.EligibleUsers, want)
		}
	})

	t.Run("複数の承認ステップを、承認される順に、承認者と必要な承認数と一緒に返す", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			ApprovalWorkflow: manualApprovals(false,
				approvalStep(1, accessControlEntry("user:carol@example.com")),
				approvalStep(2, accessControlEntry("user:dave@example.com"), accessControlEntry("group:group-2@example.com")),
			),
		})

		// The first step must be approved before the second one, so the order matters.
		want := []ApprovalStep{
			{Approvers: []string{"user:carol@example.com"}, ApprovalsNeeded: 1},
			{Approvers: []string{"user:dave@example.com", "group:group-2@example.com"}, ApprovalsNeeded: 2},
		}
		if !reflect.DeepEqual(got.ApprovalSteps, want) {
			t.Errorf("ApprovalSteps = %+v, want %+v", got.ApprovalSteps, want)
		}
	})

	t.Run("承認者の理由の入力が必須ならRequireApproverJustificationはtrueになる", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			ApprovalWorkflow: manualApprovals(true),
		})

		if !got.RequireApproverJustification {
			t.Errorf("RequireApproverJustification = false, want true")
		}
	})

	t.Run("承認が設定されていなければ承認ステップは空になる", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{})

		if len(got.ApprovalSteps) != 0 {
			t.Errorf("ApprovalSteps = %+v, want none", got.ApprovalSteps)
		}
		if got.RequireApproverJustification {
			t.Errorf("RequireApproverJustification = true, want false without approvals")
		}
	})

	t.Run("理由の入力が必須ならRequireJustificationはtrueになる", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			RequesterJustificationConfig: requiredJustification(),
		})

		if !got.RequireJustification {
			t.Errorf("RequireJustification = false, want true")
		}
	})

	t.Run("理由の入力が任意ならRequireJustificationはfalseになる", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			RequesterJustificationConfig: optionalJustification(),
		})

		if got.RequireJustification {
			t.Errorf("RequireJustification = true, want false")
		}
	})

	t.Run("最大リクエスト時間を返す", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			MaxRequestDuration: durationpb.New(2 * time.Hour),
		})

		if want := 2 * time.Hour; got.MaxRequestDuration != want {
			t.Errorf("MaxRequestDuration = %v, want %v", got.MaxRequestDuration, want)
		}
	})

	t.Run("状態を名前の文字列で返す", func(t *testing.T) {
		got := getFakeEntitlement(t, &privilegedaccessmanagerpb.Entitlement{
			State: privilegedaccessmanagerpb.Entitlement_AVAILABLE,
		})

		if want := "AVAILABLE"; got.State != want {
			t.Errorf("State = %q, want %q", got.State, want)
		}
	})

	t.Run("404はErrEntitlementNotFoundを返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeEntitlementPath, http.StatusNotFound, `{"error":{"code":404,"message":"Entitlement not found","status":"NOT_FOUND"}}`)
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
		srv := newFakePAMServer(t, http.MethodGet, fakeEntitlementPath, http.StatusForbidden, `{"error":{"code":403,"message":"Permission denied","status":"PERMISSION_DENIED"}}`)
		client := newTestPAMClient(t, srv.URL)

		got, err := client.GetEntitlement(context.Background(), "my-project", "my-entitlement")
		if err == nil {
			t.Fatalf("GetEntitlement() = %+v, want an error", got)
		}
		if errors.Is(err, ErrEntitlementNotFound) {
			t.Errorf("GetEntitlement() = %v, want an error that is not ErrEntitlementNotFound", err)
		}
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusForbidden {
			t.Errorf("errors.As(googleapi.Error) = %v, want code 403", gerr)
		}
	})
}

func TestPAMClient_CreateEntitlement(t *testing.T) {
	t.Run("globalリージョンにエンタイトルメントIDを指定してPOSTする", func(t *testing.T) {
		got, srv := successfulCreate(t, &Entitlement{})
		if got.Name != fakeEntitlementName {
			t.Errorf("Name = %q, want %q", got.Name, fakeEntitlementName)
		}
		if want := []string{"POST " + fakeCreateEntitlementPath}; !reflect.DeepEqual(srv.requests, want) {
			t.Errorf("requests = %v, want %v", srv.requests, want)
		}
		if want := []string{"my-entitlement"}; !reflect.DeepEqual(srv.query, want) {
			t.Errorf("entitlementId = %v, want %v", srv.query, want)
		}
	})

	t.Run("リソースとリソースタイプと、条件式付きのロールをリクエストに載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{ResourceType: "cloudresourcemanager.googleapis.com/Project", Resource: "//cloudresourcemanager.googleapis.com/projects/my-project", Roles: []EntitlementRole{{Role: "roles/viewer"}, {Role: "roles/storage.admin", ConditionExpression: `resource.type == "storage.googleapis.com/Bucket"`}}})
		got := srv.entitlement(t).GetPrivilegedAccess().GetGcpIamAccess()
		if got.GetResource() != "//cloudresourcemanager.googleapis.com/projects/my-project" || got.GetResourceType() != "cloudresourcemanager.googleapis.com/Project" {
			t.Errorf("GcpIamAccess = %+v, want resource and type", got)
		}
		want := []*privilegedaccessmanagerpb.PrivilegedAccess_GcpIamAccess_RoleBinding{roleBinding("roles/viewer", ""), roleBinding("roles/storage.admin", `resource.type == "storage.googleapis.com/Bucket"`)}
		if !equalProtos(got.GetRoleBindings(), want) {
			t.Errorf("RoleBindings = %+v, want %+v", got.GetRoleBindings(), want)
		}
	})

	t.Run("対象ユーザーを1つのエントリにまとめてリクエストに載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{EligibleUsers: []string{"user:alice@example.com", "group:group-1@example.com", "user:bob@example.com"}})
		want := []*privilegedaccessmanagerpb.AccessControlEntry{accessControlEntry("user:alice@example.com", "group:group-1@example.com", "user:bob@example.com")}
		if got := srv.entitlement(t).GetEligibleUsers(); !equalProtos(got, want) {
			t.Errorf("EligibleUsers = %+v, want %+v", got, want)
		}
	})

	t.Run("複数の承認ステップを、承認される順に、承認者と必要な承認数と一緒にリクエストに載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{ApprovalSteps: []ApprovalStep{{Approvers: []string{"user:carol@example.com"}, ApprovalsNeeded: 1}, {Approvers: []string{"user:dave@example.com", "group:group-2@example.com"}, ApprovalsNeeded: 2}}})
		got := srv.entitlement(t).GetApprovalWorkflow().GetManualApprovals().GetSteps()
		want := []*privilegedaccessmanagerpb.ManualApprovals_Step{approvalStep(1, accessControlEntry("user:carol@example.com")), approvalStep(2, accessControlEntry("user:dave@example.com", "group:group-2@example.com"))}
		if !equalProtos(got, want) {
			t.Errorf("Steps = %+v, want %+v", got, want)
		}
	})

	t.Run("承認者の理由の入力が必須ならリクエストにそう載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{ApprovalSteps: []ApprovalStep{{}}, RequireApproverJustification: true})
		if !srv.entitlement(t).GetApprovalWorkflow().GetManualApprovals().GetRequireApproverJustification() {
			t.Error("RequireApproverJustification = false, want true")
		}
	})

	t.Run("承認ステップがなければ承認の設定を載せない", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{RequireApproverJustification: true})
		if got := srv.entitlement(t).GetApprovalWorkflow(); got != nil {
			t.Errorf("ApprovalWorkflow = %+v, want nil", got)
		}
	})

	t.Run("理由の入力が必須ならリクエストにそう載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{RequireJustification: true})
		if srv.entitlement(t).GetRequesterJustificationConfig().GetUnstructured() == nil {
			t.Error("RequesterJustificationConfig = not unstructured, want unstructured")
		}
	})

	t.Run("理由の入力が任意ならリクエストにそう載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{})
		if srv.entitlement(t).GetRequesterJustificationConfig().GetNotMandatory() == nil {
			t.Error("RequesterJustificationConfig = not notMandatory, want notMandatory")
		}
	})

	t.Run("最大リクエスト時間をリクエストに載せる", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{MaxRequestDuration: 2 * time.Hour})
		want := 2 * time.Hour
		if got := srv.entitlement(t).GetMaxRequestDuration().AsDuration(); got != want {
			t.Errorf("MaxRequestDuration = %v, want %v", got, want)
		}
	})

	t.Run("オペレーションが最初のレスポンスで完了している", func(t *testing.T) {
		_, srv := successfulCreate(t, &Entitlement{})
		if len(srv.requests) != 1 {
			t.Errorf("requests = %v, want no poll", srv.requests)
		}
	})

	t.Run("オペレーションが数回のポーリング後に完了する", func(t *testing.T) {
		running := fakePAMResponse{status: http.StatusOK, body: operationJSON(t, false, nil, nil)}
		done := fakePAMResponse{status: http.StatusOK, body: operationJSON(t, true, &privilegedaccessmanagerpb.Entitlement{Name: fakeEntitlementName}, nil)}
		srv := newFakePAMCreateServer(t, []fakePAMResponse{running}, []fakePAMResponse{running, done})
		client := newTestPAMClient(t, srv.URL)
		client.pollInterval = 0
		if _, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{}); err != nil {
			t.Fatalf("CreateEntitlement() = %v, want nil error", err)
		}
		if want := []string{"POST " + fakeCreateEntitlementPath, "GET " + fakeOperationPath, "GET " + fakeOperationPath}; !reflect.DeepEqual(srv.requests, want) {
			t.Errorf("requests = %v, want %v", srv.requests, want)
		}
	})

	t.Run("完了したオペレーションのレスポンスがEntitlementに変換される", func(t *testing.T) {
		response := &privilegedaccessmanagerpb.Entitlement{Name: fakeEntitlementName, State: privilegedaccessmanagerpb.Entitlement_AVAILABLE, EligibleUsers: []*privilegedaccessmanagerpb.AccessControlEntry{accessControlEntry("user:alice@example.com")}}
		srv := newFakePAMCreateServer(t, []fakePAMResponse{{status: http.StatusOK, body: operationJSON(t, true, response, nil)}}, nil)
		client := newTestPAMClient(t, srv.URL)
		got, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{})
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "AVAILABLE" || !reflect.DeepEqual(got.EligibleUsers, []string{"user:alice@example.com"}) {
			t.Errorf("Entitlement = %+v, want converted response", got)
		}
	})

	t.Run("同じIDのエンタイトルメントが既にあればErrEntitlementAlreadyExistsを返す", func(t *testing.T) {
		srv := newFakePAMCreateServer(t, []fakePAMResponse{{status: http.StatusConflict, body: `{"error":{"code":409,"message":"already exists","status":"ALREADY_EXISTS"}}`}}, nil)
		client := newTestPAMClient(t, srv.URL)
		_, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{})
		if !errors.Is(err, ErrEntitlementAlreadyExists) {
			t.Errorf("error = %v, want ErrEntitlementAlreadyExists", err)
		}
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusConflict {
			t.Errorf("error = %v, want wrapped 409", err)
		}
	})

	t.Run("作成リクエストが409以外で拒否されてもErrEntitlementAlreadyExistsにしない", func(t *testing.T) {
		srv := newFakePAMCreateServer(t, []fakePAMResponse{{status: http.StatusForbidden, body: `{"error":{"code":403,"message":"denied","status":"PERMISSION_DENIED"}}`}}, nil)
		client := newTestPAMClient(t, srv.URL)
		_, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{})
		if err == nil || errors.Is(err, ErrEntitlementAlreadyExists) {
			t.Errorf("error = %v, want non-conflict error", err)
		}
	})

	t.Run("オペレーションがエラーで完了する", func(t *testing.T) {
		// err != nilだけ確認している。オペレーションのエラー内容(code 8, "quota
		// exceeded")がerrに含まれることまでは見ていないので、別の理由でerrが
		// nilでなくなっても、このテストは気づけない。
		srv := newFakePAMCreateServer(t, []fakePAMResponse{{status: http.StatusOK, body: operationJSON(t, true, nil, &statuspb.Status{Code: 8, Message: "quota exceeded"})}}, nil)
		client := newTestPAMClient(t, srv.URL)
		if got, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{}); err == nil || got != nil {
			t.Errorf("CreateEntitlement() = %+v, %v, want error", got, err)
		}
	})

	t.Run("ポーリング中に500", func(t *testing.T) {
		running := fakePAMResponse{status: http.StatusOK, body: operationJSON(t, false, nil, nil)}
		srv := newFakePAMCreateServer(t, []fakePAMResponse{running}, []fakePAMResponse{{status: http.StatusInternalServerError, body: `{"error":{"code":500,"message":"server error","status":"INTERNAL"}}`}})
		client := newTestPAMClient(t, srv.URL)
		client.pollInterval = 0
		if got, err := client.CreateEntitlement(context.Background(), "my-project", "my-entitlement", &Entitlement{}); err == nil || got != nil {
			t.Errorf("CreateEntitlement() = %+v, %v, want error", got, err)
		}
	})
}

func TestPAMClient_ListEntitlements(t *testing.T) {
	t.Run("globalリージョンのエンタイトルメントを1回のGETで一覧する", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("各エンタイトルメントをGetEntitlementと同じ形に変換する", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エンタイトルメントが1件もなければ空のリストを返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エンタイトルメントが複数ページにまたがる", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("並びはAPIの並びのまま、ページをまたいでも保つ", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エンタイトルメントの一覧の取得が1ページ目で失敗する", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エンタイトルメントの一覧の取得が2ページ目で失敗する", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エラーはそのまま返す", func(t *testing.T) {
		t.Skip("not written yet")
	})
}

func TestPAMClient_CheckOnboarding(t *testing.T) {
	const serviceAccount = "service-123@gcp-sa-pam.iam.gserviceaccount.com"

	t.Run("globalリージョンでサービスアカウントを返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.CheckOnboardingStatusResponse{
			ServiceAccount: serviceAccount,
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err != nil {
			t.Fatalf("CheckOnboarding() = %v, want nil error", err)
		}
		if got.ServiceAccount != serviceAccount {
			t.Errorf("ServiceAccount = %q, want %q", got.ServiceAccount, serviceAccount)
		}
		if want := []string{"GET " + fakeOnboardingPath}; !reflect.DeepEqual(srv.Requests(), want) {
			t.Errorf("got requests %v, want %v", srv.Requests(), want)
		}
	})

	t.Run("問題がなければ指摘は空になる", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.CheckOnboardingStatusResponse{
			ServiceAccount: serviceAccount,
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err != nil {
			t.Fatalf("CheckOnboarding() = %v, want nil error", err)
		}
		if len(got.Findings) != 0 {
			t.Errorf("Findings = %+v, want none", got.Findings)
		}
	})

	t.Run("IAMに拒否された権限を返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.CheckOnboardingStatusResponse{
			ServiceAccount: serviceAccount,
			Findings:       []*privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding{onboardingFinding("resourcemanager.projects.getIamPolicy")},
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err != nil {
			t.Fatalf("CheckOnboarding() = %v, want nil error", err)
		}
		want := []OnboardingFinding{{MissingPermissions: []string{"resourcemanager.projects.getIamPolicy"}}}
		if !reflect.DeepEqual(got.Findings, want) {
			t.Errorf("Findings = %+v, want %+v", got.Findings, want)
		}
	})

	t.Run("1つの指摘に複数の権限があれば全て返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.CheckOnboardingStatusResponse{
			ServiceAccount: serviceAccount,
			Findings: []*privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding{
				onboardingFinding("resourcemanager.projects.getIamPolicy", "resourcemanager.projects.setIamPolicy"),
			},
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err != nil {
			t.Fatalf("CheckOnboarding() = %v, want nil error", err)
		}
		// The order of the API is kept.
		want := []OnboardingFinding{{MissingPermissions: []string{"resourcemanager.projects.getIamPolicy", "resourcemanager.projects.setIamPolicy"}}}
		if !reflect.DeepEqual(got.Findings, want) {
			t.Errorf("Findings = %+v, want %+v", got.Findings, want)
		}
	})

	t.Run("指摘が複数あれば全て返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusOK, protoJSON(t, &privilegedaccessmanagerpb.CheckOnboardingStatusResponse{
			ServiceAccount: serviceAccount,
			Findings: []*privilegedaccessmanagerpb.CheckOnboardingStatusResponse_Finding{
				onboardingFinding("resourcemanager.projects.getIamPolicy"),
				onboardingFinding("resourcemanager.projects.setIamPolicy", "iam.roles.get"),
			},
		}))
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err != nil {
			t.Fatalf("CheckOnboarding() = %v, want nil error", err)
		}
		// The findings stay apart, in the order of the API.
		want := []OnboardingFinding{
			{MissingPermissions: []string{"resourcemanager.projects.getIamPolicy"}},
			{MissingPermissions: []string{"resourcemanager.projects.setIamPolicy", "iam.roles.get"}},
		}
		if !reflect.DeepEqual(got.Findings, want) {
			t.Errorf("Findings = %+v, want %+v", got.Findings, want)
		}
	})

	t.Run("エラーはそのまま返す", func(t *testing.T) {
		srv := newFakePAMServer(t, http.MethodGet, fakeOnboardingPath, http.StatusForbidden, `{"error":{"code":403,"message":"Permission denied","status":"PERMISSION_DENIED"}}`)
		client := newTestPAMClient(t, srv.URL)

		got, err := client.CheckOnboarding(context.Background(), "my-project")
		if err == nil {
			t.Fatalf("CheckOnboarding() = %+v, want an error", got)
		}
		if got != nil {
			t.Errorf("CheckOnboarding() status = %+v, want nil with an error", got)
		}
		// The error of the API stays in the chain, so its code and message are not lost.
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusForbidden {
			t.Errorf("errors.As(googleapi.Error) = %v, want code 403", gerr)
		}
	})
}

func TestOnboardingStatus_IsReady(t *testing.T) {
	t.Run("サービスアカウントがあって指摘がなければtrueになる", func(t *testing.T) {
		// Findings is nil in one and empty in the other: both mean no findings.
		for name, s := range map[string]*OnboardingStatus{
			"nil":   {ServiceAccount: "service-123@gcp-sa-pam.iam.gserviceaccount.com"},
			"empty": {ServiceAccount: "service-123@gcp-sa-pam.iam.gserviceaccount.com", Findings: []OnboardingFinding{}},
		} {
			if !s.IsReady() {
				t.Errorf("IsReady() = false with %s Findings, want true", name)
			}
		}
	})

	t.Run("指摘があればfalseになる", func(t *testing.T) {
		s := &OnboardingStatus{
			ServiceAccount: "service-123@gcp-sa-pam.iam.gserviceaccount.com",
			Findings:       []OnboardingFinding{{MissingPermissions: []string{"resourcemanager.projects.getIamPolicy"}}},
		}
		if s.IsReady() {
			t.Errorf("IsReady() = true with %d findings, want false", len(s.Findings))
		}
	})

	t.Run("サービスアカウントが空ならfalseになる", func(t *testing.T) {
		// It is false even with no findings: PAM may not report every issue.
		s := &OnboardingStatus{}
		if s.IsReady() {
			t.Errorf("IsReady() = true without a service account, want false")
		}
	})
}
