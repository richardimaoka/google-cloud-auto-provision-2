package googlecloud

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"cloud.google.com/go/privilegedaccessmanager/apiv1/privilegedaccessmanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
)

// Unit tests for PAMClient.
//
// They fake the API with a server, like the tests of IAMClient. The client asks for
// enums as numbers ($alt=json;enum-encoding=int), so the fake answers with them.
//
// Every case below is skipped until it is written.

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
	t.Run("globalリージョンでサービスエージェントを返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("問題がなければ指摘は空になる", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("IAMに拒否された権限を返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("1つの指摘に複数の権限があれば全て返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("指摘が複数あれば全て返す", func(t *testing.T) {
		t.Skip("not written yet")
	})

	t.Run("エラーはそのまま返す", func(t *testing.T) {
		t.Skip("not written yet")
	})
}
