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

	"cloud.google.com/go/privilegedaccessmanager/apiv1/privilegedaccessmanagerpb"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Unit tests for PAMClient.
//
// They fake the API with a server, like the tests of IAMClient. The client asks for
// enums as numbers ($alt=json;enum-encoding=int), so the fake answers with them.
//
// Every case below is skipped until it is written.

// fakePAMServer fakes one method and path of the API and records the requests it
// gets. Any other method or path fails the test: the API answers 404 to a path it
// does not know, which the client can take for an entitlement that does not exist.
type fakePAMServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
}

const (
	fakeEntitlementPath = "/v1/projects/my-project/locations/global/entitlements/my-entitlement"
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

func TestPAMClient_GetEntitlement(t *testing.T) {
	t.Run("エンタイトルメントの全項目を変換する", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("承認と理由が不要なエンタイトルメントは空の値になる", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("404はErrEntitlementNotFoundを返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("404以外のエラーはErrEntitlementNotFoundにしない", func(t *testing.T) {
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
