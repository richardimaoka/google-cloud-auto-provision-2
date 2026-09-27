package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
)

// newFakeBigQueryServer answers every query in my-project with a finished job
// that changed affected rows, and returns the sent parameters by name, one map
// per query.
func newFakeBigQueryServer(t *testing.T, affected int64) (*httptest.Server, *[]map[string]string) {
	t.Helper()
	var sent []map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/my-project/queries", func(w http.ResponseWriter, r *http.Request) {
		var req bigquery.QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode query request: %v", err)
		}
		params := map[string]string{}
		for _, p := range req.QueryParameters {
			params[p.Name] = p.ParameterValue.Value
		}
		sent = append(sent, params)
		json.NewEncoder(w).Encode(&bigquery.QueryResponse{
			JobComplete:        true,
			JobReference:       &bigquery.JobReference{ProjectId: "my-project", JobId: "job1"},
			NumDmlAffectedRows: affected,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &sent
}

func newTestProgressStore(t *testing.T, srv *httptest.Server) *bigQueryProgressStore {
	t.Helper()
	client, err := googlecloud.NewBigQueryClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	return &bigQueryProgressStore{client: client, projectID: "my-project", datasetID: "my_dataset"}
}

func TestBigQueryProgressStore(t *testing.T) {
	current := testProgress(t, "request-1")
	next, err := current.AdvancePastCreation("example-project")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Createは、progressを行の値に変換して記録し、IDを返す", func(t *testing.T) {
		srv, sent := newFakeBigQueryServer(t, 1)

		id, err := newTestProgressStore(t, srv).Create(context.Background(), current)
		if err != nil {
			t.Fatalf("Create() = %v", err)
		}
		if id != current.ID() {
			t.Errorf("Create() = %q, want %q", id, current.ID())
		}
		if len(*sent) != 1 {
			t.Fatalf("sent %d queries, want 1", len(*sent))
		}
		want := map[string]string{
			"request_id": "request-1", "revision": "1", "state": "creating_project", "status": "in_progress",
			"project_id": "", "display_name": "Example Project", "requester": "requester@example.com",
			"developer_group": "developer-group@example.com", "error_message": "",
		}
		for name, value := range want {
			if got := (*sent)[0][name]; got != value {
				t.Errorf("parameter %s = %q, want %q", name, got, value)
			}
		}
	})

	t.Run("Createは、記録済みのIDならエラーを返す", func(t *testing.T) {
		srv, _ := newFakeBigQueryServer(t, 0)

		if _, err := newTestProgressStore(t, srv).Create(context.Background(), current); err == nil {
			t.Error("Create(recorded ID) = nil error, want an error")
		}
	})

	t.Run("Updateは、currentの状態のときだけnextの値で更新する", func(t *testing.T) {
		srv, sent := newFakeBigQueryServer(t, 1)

		if err := newTestProgressStore(t, srv).Update(context.Background(), current, next); err != nil {
			t.Fatalf("Update() = %v", err)
		}
		if len(*sent) != 1 {
			t.Fatalf("sent %d queries, want 1", len(*sent))
		}
		params := (*sent)[0]
		if params["state"] != string(domain.StateAddingServiceAccountIAMBindings) || params["project_id"] != "example-project" {
			t.Errorf("parameters state/project_id = %q/%q, want next's %q/%q", params["state"], params["project_id"], domain.StateAddingServiceAccountIAMBindings, "example-project")
		}
		if params["current_state"] != string(domain.StateCreatingProject) || params["current_status"] != string(domain.StatusInProgress) {
			t.Errorf("parameters current_state/current_status = %q/%q, want current's %q/%q", params["current_state"], params["current_status"], domain.StateCreatingProject, domain.StatusInProgress)
		}
	})

	t.Run("Updateは、行がcurrentの状態でなくなっていればエラーを返す", func(t *testing.T) {
		srv, _ := newFakeBigQueryServer(t, 0)

		if err := newTestProgressStore(t, srv).Update(context.Background(), current, next); err == nil {
			t.Error("Update(row moved on) = nil error, want an error")
		}
	})
}
