//go:build integration

package googlecloud

// These tests call the real BigQuery API and write to the real
// project_creation_progress table (see ../terraform). Each run adds one row
// under a request ID of its own and deletes it at the end, even if the test
// fails. The table is in the control project, GOOGLECLOUD_TEST_CONTROL_PROJECT.
// It shares the environment of the other integration tests, see
// groups_integration_test.go, plus:
//
//	GOOGLECLOUD_TEST_BIGQUERY_DATASET  dataset that holds project_creation_progress
//
//	(set -a; . ./.env.integration; set +a; go test -tags integration -count=1 -v ./googlecloud -run BigQuery)

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
)

func TestBigQueryClient_GetTable_Integration(t *testing.T) {
	t.Run("作成したテーブルを取得できる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

// TestBigQueryClient_Progress_Integration checks what the fake can't: that
// MERGE inserts once, that the changed-row counts are what CreateProgress and
// UpdateProgress rely on, and that empty strings are stored as NULL. Later
// steps build on earlier ones, so they are steps of one test.
func TestBigQueryClient_Progress_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// The control project both holds the table and is billed for the queries.
	projectID := requireEnv(t, "GOOGLECLOUD_TEST_CONTROL_PROJECT")
	datasetID := requireEnv(t, "GOOGLECLOUD_TEST_BIGQUERY_DATASET")
	client, err := NewBigQueryClient(ctx, option.WithQuotaProject(projectID))
	if err != nil {
		t.Fatalf("NewBigQueryClient: %v", err)
	}

	// Unique per run, so runs never touch each other's rows.
	current := ProjectCreationProgressRow{
		RequestID:      fmt.Sprintf("it-%d", time.Now().UnixNano()),
		Revision:       1,
		State:          "creating_project",
		Status:         "in_progress",
		DisplayName:    "Integration Test",
		Requester:      "requester@example.com",
		DeveloperGroup: "developer-group@example.com",
	}
	next := current
	next.State = "adding_service_account_iam_bindings"
	next.ProjectID = "it-project-123456"

	t.Cleanup(func() {
		// A fresh context: ctx may have ended by now.
		if err := deleteProgress(context.Background(), client, projectID, datasetID, current.RequestID); err != nil {
			t.Errorf("clean up row %q: %v", current.RequestID, err)
		}
	})

	if err := client.CreateProgress(ctx, projectID, datasetID, current); err != nil {
		t.Fatalf("CreateProgress() = %v, want nil error", err)
	}

	t.Run("挿入した行を読み戻せ、空のproject_idとerror_messageはNULLになる", func(t *testing.T) {
		got := readProgress(t, ctx, client, projectID, datasetID, current.RequestID)
		want := progressCells{state: current.State, status: current.Status, projectID: nil, errorMessage: nil, timestampsSet: true}
		if got != want {
			t.Errorf("row = %+v, want %+v", got, want)
		}
	})

	t.Run("同じリクエストIDでもう一度作るとErrProgressExistsで、行は変わらない", func(t *testing.T) {
		again := current
		again.DisplayName = "Should Not Be Written"
		if err := client.CreateProgress(ctx, projectID, datasetID, again); !errors.Is(err, ErrProgressExists) {
			t.Errorf("CreateProgress(same ID) = %v, want ErrProgressExists", err)
		}
		if got := readProgress(t, ctx, client, projectID, datasetID, current.RequestID); got.state != current.State {
			t.Errorf("state = %q after the second CreateProgress, want %q", got.state, current.State)
		}
	})

	t.Run("currentの状態から、nextに更新できる", func(t *testing.T) {
		if err := client.UpdateProgress(ctx, projectID, datasetID, current, next); err != nil {
			t.Fatalf("UpdateProgress() = %v, want nil error", err)
		}
		got := readProgress(t, ctx, client, projectID, datasetID, current.RequestID)
		if got.state != next.State || got.projectID == nil || *got.projectID != next.ProjectID {
			t.Errorf("row = %+v, want state %q and project_id %q", got, next.State, next.ProjectID)
		}
	})

	t.Run("行がもうcurrentの状態でなければErrProgressChangedで、行は変わらない", func(t *testing.T) {
		// The row is at next now, so another update from current is stale.
		stale := next
		stale.State = "adding_iam_bindings"
		if err := client.UpdateProgress(ctx, projectID, datasetID, current, stale); !errors.Is(err, ErrProgressChanged) {
			t.Errorf("UpdateProgress(stale current) = %v, want ErrProgressChanged", err)
		}
		if got := readProgress(t, ctx, client, projectID, datasetID, current.RequestID); got.state != next.State {
			t.Errorf("state = %q after the stale update, want %q", got.state, next.State)
		}
	})
}

// progressCells is what readProgress reads back. Nil pointers are NULL.
type progressCells struct {
	state, status           string
	projectID, errorMessage *string
	timestampsSet           bool
}

// readProgress reads the row of requestID, failing the test unless there is
// exactly one.
func readProgress(t *testing.T, ctx context.Context, c *BigQueryClient, projectID, datasetID, requestID string) progressCells {
	t.Helper()
	sql := fmt.Sprintf("SELECT state, status, project_id, error_message, created_at IS NOT NULL AND updated_at IS NOT NULL "+
		"FROM `%s.%s.project_creation_progress` WHERE request_id = @request_id", projectID, datasetID)
	_, rows, err := c.query(ctx, projectID, sql, []*bigquery.QueryParameter{stringParam("request_id", requestID)})
	if err != nil {
		t.Fatalf("read row %q: %v", requestID, err)
	}
	if len(rows) != 1 {
		t.Fatalf("read row %q: got %d rows, want 1", requestID, len(rows))
	}
	f := rows[0].F
	nullable := func(v any) *string {
		if s, ok := v.(string); ok {
			return &s
		}
		return nil
	}
	state, _ := f[0].V.(string)
	status, _ := f[1].V.(string)
	return progressCells{
		state:         state,
		status:        status,
		projectID:     nullable(f[2].V),
		errorMessage:  nullable(f[3].V),
		timestampsSet: f[4].V == "true",
	}
}

// deleteProgress removes the row of requestID. Only tests delete rows.
func deleteProgress(ctx context.Context, c *BigQueryClient, projectID, datasetID, requestID string) error {
	sql := fmt.Sprintf("DELETE FROM `%s.%s.project_creation_progress` WHERE request_id = @request_id", projectID, datasetID)
	_, err := c.exec(ctx, projectID, sql, []*bigquery.QueryParameter{stringParam("request_id", requestID)})
	return err
}
