package webapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"
)

// TestInitiateProjectCreationAsUser checks that initiating a project
// creation validates and records the request before touching Google Cloud,
// creates the project and grants the automation service account access
// under the caller's own credentials, and reports a failure at either step
// as an error rather than proceeding.
func TestInitiateProjectCreationAsUser(t *testing.T) {
	t.Run("正常系: 記録・プロジェクト作成・権限付与・AdvancePastCreation・worker起動まで行う", func(t *testing.T) {
		t.Skip("blocked: no BigQuery client seam yet to record or verify ProjectCreationProgress against")
	})

	t.Run("recordProjectCreationRequestがエラーならプロジェクトを作成せずエラーを返す", func(t *testing.T) {
		// Validation should reject this before it ever reaches Google Cloud.
		mux := http.NewServeMux()
		mux.HandleFunc("POST /v3/projects", func(w http.ResponseWriter, r *http.Request) {
			t.Error("googlecloud.ProjectsClient.Create was called despite an invalid request")
			w.WriteHeader(http.StatusInternalServerError)
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		request := InitiateProjectCreationRequest{
			ID:             "request-invalid-email",
			DisplayName:    "Example Project",
			Requester:      "not-an-email",
			DeveloperGroup: "developer-group@example.com",
		}
		_, err := InitiateProjectCreationAsUser(context.Background(), request,
			option.WithEndpoint(srv.URL), option.WithoutAuthentication())
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(invalid requester email) = nil error, want an error")
		}
	})

	t.Run("記録済みのIDで再試行すると二重にプロジェクトを作成しない", func(t *testing.T) {
		t.Skip("blocked: idempotency needs the same persisted-state seam as the happy path above")
	})

	t.Run("createProjectAndGrantAutomationAccessがエラーならworkerを起動せずエラーを返す", func(t *testing.T) {
		calls := 0
		mux := http.NewServeMux()
		mux.HandleFunc("POST /v3/projects", func(w http.ResponseWriter, r *http.Request) {
			calls++
			http.Error(w, `{"error":{"code":403,"message":"permission denied","status":"PERMISSION_DENIED"}}`, http.StatusForbidden)
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		request := InitiateProjectCreationRequest{
			ID:             "request-create-fails",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		_, err := InitiateProjectCreationAsUser(context.Background(), request,
			option.WithEndpoint(srv.URL), option.WithoutAuthentication())
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(project creation fails) = nil error, want an error")
		}
		// Confirms the error came from this fake server, not from ignoring
		// callerCredentials and failing some other way.
		if calls != 1 {
			t.Errorf("fake create-project endpoint was called %d times, want 1", calls)
		}
	})
}

// TestCreateProjectAndGrantAutomationAccess is a scaffold:
// createProjectAndGrantAutomationAccess is still a stub, so every case is
// skipped until it has a real body to test.
func TestCreateProjectAndGrantAutomationAccess(t *testing.T) {
	t.Run("callerCredentialsでプロジェクトを作成し、自動化Service Accountへ権限を付与する", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("プロジェクト作成が失敗したら権限付与はせずエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("権限付与が失敗したらエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
}
