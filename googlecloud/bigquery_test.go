package googlecloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// newFakeBigQueryServer fakes the BigQuery dataset endpoint that GetDataset
// relies on. It answers status for GET /projects/my-project/datasets/my_dataset,
// returning dataset when status is 200, and 404 for any other project or dataset
// ID.
func newFakeBigQueryServer(t *testing.T, status int, dataset *bigquery.Dataset) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/datasets/my_dataset", func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		json.NewEncoder(w).Encode(dataset)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestBigQueryClient(t *testing.T, srv *httptest.Server) *BigQueryClient {
	t.Helper()

	client, err := NewBigQueryClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewBigQueryClient: %v", err)
	}
	client.pollInterval = time.Millisecond
	return client
}

func TestBigQueryClient_GetDataset(t *testing.T) {
	t.Run("存在するデータセットのIDとロケーションを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusOK, &bigquery.Dataset{
			DatasetReference: &bigquery.DatasetReference{ProjectId: "my-project", DatasetId: "my_dataset"},
			Location:         "US",
		})
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err != nil {
			t.Fatalf("GetDataset() = %v, want nil error", err)
		}
		want := &Dataset{ID: "my_dataset", Location: "US"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetDataset() = %+v, want %+v", got, want)
		}
	})
	t.Run("存在しないデータセットはエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusNotFound, nil)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err == nil {
			t.Fatal("GetDataset() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "get dataset") {
			t.Errorf("GetDataset() error = %q, want it to contain %q", err, "get dataset")
		}
		if got != nil {
			t.Errorf("GetDataset() = %+v on error, want nil", got)
		}
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusForbidden, nil)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err == nil {
			t.Fatal("GetDataset() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "get dataset") {
			t.Errorf("GetDataset() error = %q, want it to contain %q", err, "get dataset")
		}
		if got != nil {
			t.Errorf("GetDataset() = %+v on error, want nil", got)
		}
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusNotFound, nil)
		client := newTestBigQueryClient(t, srv)

		_, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) {
			t.Fatalf("GetDataset() error = %v, want it to wrap a *googleapi.Error", err)
		}
		if gerr.Code != http.StatusNotFound {
			t.Errorf("googleapi.Error.Code = %d, want %d", gerr.Code, http.StatusNotFound)
		}
	})
}

// newFakeBigQueryTablesServer fakes the two BigQuery endpoints that GetTable
// relies on. It answers tableStatus for
// GET /projects/my-project/datasets/my_dataset/tables/my_table, returning table
// when tableStatus is 200, and datasetStatus for
// GET /projects/my-project/datasets/my_dataset, which GetTable asks after a 404
// of the table. Any other ID gets a 404.
func newFakeBigQueryTablesServer(t *testing.T, tableStatus int, table *bigquery.Table, datasetStatus int) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/datasets/my_dataset/tables/my_table", func(w http.ResponseWriter, r *http.Request) {
		if tableStatus != http.StatusOK {
			w.WriteHeader(tableStatus)
			return
		}
		json.NewEncoder(w).Encode(table)
	})
	mux.HandleFunc("/projects/my-project/datasets/my_dataset", func(w http.ResponseWriter, r *http.Request) {
		if datasetStatus != http.StatusOK {
			w.WriteHeader(datasetStatus)
			return
		}
		json.NewEncoder(w).Encode(&bigquery.Dataset{
			DatasetReference: &bigquery.DatasetReference{ProjectId: "my-project", DatasetId: "my_dataset"},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestBigQueryClient_GetTable(t *testing.T) {
	t.Run("存在するテーブルのIDを返す", func(t *testing.T) {
		srv := newFakeBigQueryTablesServer(t, http.StatusOK, &bigquery.Table{
			TableReference: &bigquery.TableReference{ProjectId: "my-project", DatasetId: "my_dataset", TableId: "my_table"},
		}, http.StatusOK)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetTable(context.Background(), "my-project", "my_dataset", "my_table")
		if err != nil {
			t.Fatalf("GetTable() = %v, want nil error", err)
		}
		want := &Table{ID: "my_table"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetTable() = %+v, want %+v", got, want)
		}
	})
	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryTablesServer(t, http.StatusNotFound, nil, http.StatusOK)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetTable(context.Background(), "my-project", "my_dataset", "my_table")
		if !errors.Is(err, ErrTableNotFound) {
			t.Fatalf("GetTable() error = %v, want ErrTableNotFound", err)
		}
		if got != nil {
			t.Errorf("GetTable() = %+v on error, want nil", got)
		}
	})
	t.Run("存在しないデータセットのテーブルは、テーブルが無い場合と区別できるエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryTablesServer(t, http.StatusNotFound, nil, http.StatusNotFound)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetTable(context.Background(), "my-project", "my_dataset", "my_table")
		if !errors.Is(err, ErrDatasetNotFound) {
			t.Fatalf("GetTable() error = %v, want ErrDatasetNotFound", err)
		}
		if errors.Is(err, ErrTableNotFound) {
			t.Errorf("GetTable() error = %v, want it not to be ErrTableNotFound", err)
		}
		if got != nil {
			t.Errorf("GetTable() = %+v on error, want nil", got)
		}
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryTablesServer(t, http.StatusForbidden, nil, http.StatusOK)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetTable(context.Background(), "my-project", "my_dataset", "my_table")
		if err == nil {
			t.Fatal("GetTable() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "get table") {
			t.Errorf("GetTable() error = %q, want it to contain %q", err, "get table")
		}
		if errors.Is(err, ErrTableNotFound) || errors.Is(err, ErrDatasetNotFound) {
			t.Errorf("GetTable() error = %v, want it not to be a not-found error", err)
		}
		if got != nil {
			t.Errorf("GetTable() = %+v on error, want nil", got)
		}
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		srv := newFakeBigQueryTablesServer(t, http.StatusNotFound, nil, http.StatusOK)
		client := newTestBigQueryClient(t, srv)

		_, err := client.GetTable(context.Background(), "my-project", "my_dataset", "my_table")
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) {
			t.Fatalf("GetTable() error = %v, want it to wrap a *googleapi.Error", err)
		}
		if gerr.Code != http.StatusNotFound {
			t.Errorf("googleapi.Error.Code = %d, want %d", gerr.Code, http.StatusNotFound)
		}
	})
}

// newFakeBigQueryJobsServer fakes the two BigQuery endpoints that query relies
// on: POST /projects/my-project/queries, which starts the job, and
// GET /projects/my-project/queries/<job ID>, which returns the results of the
// job. A nil handler fails the test if the endpoint is called.
func newFakeBigQueryJobsServer(t *testing.T, query, results http.HandlerFunc) *httptest.Server {
	t.Helper()

	unexpected := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request to the %s endpoint: %s %s", name, r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	if query == nil {
		query = unexpected("query")
	}
	if results == nil {
		results = unexpected("results")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/my-project/queries", query)
	mux.HandleFunc("GET /projects/my-project/queries/{jobID}", results)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// respondJSON answers with v as JSON.
func respondJSON(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}

func TestBigQueryClient_query(t *testing.T) {
	job := &bigquery.JobReference{ProjectId: "my-project", JobId: "job1", Location: "US"}
	schema := &bigquery.TableSchema{Fields: []*bigquery.TableFieldSchema{
		{Name: "name", Type: "STRING"},
		{Name: "n", Type: "INTEGER"},
	}}
	row := func(name, n string) *bigquery.TableRow {
		return &bigquery.TableRow{F: []*bigquery.TableCell{{V: name}, {V: n}}}
	}

	t.Run("ジョブが完了していれば、待たずにスキーマと行を返す", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{
				JobComplete:  true,
				JobReference: job,
				Schema:       schema,
				Rows:         []*bigquery.TableRow{row("a", "1")},
			})
		}, nil)
		client := newTestBigQueryClient(t, srv)

		gotSchema, gotRows, err := client.query(context.Background(), "my-project", "SELECT 1", nil)
		if err != nil {
			t.Fatalf("query() = %v, want nil error", err)
		}
		if !reflect.DeepEqual(gotSchema, schema) {
			t.Errorf("query() schema = %+v, want %+v", gotSchema, schema)
		}
		if want := []*bigquery.TableRow{row("a", "1")}; !reflect.DeepEqual(gotRows, want) {
			t.Errorf("query() rows = %+v, want %+v", gotRows, want)
		}
	})
	t.Run("ジョブが未完了なら、完了まで待って結果を返す", func(t *testing.T) {
		var polls atomic.Int32
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{JobReference: job})
		}, func(w http.ResponseWriter, r *http.Request) {
			// The job is done at the third poll.
			if polls.Add(1) < 3 {
				respondJSON(w, &bigquery.GetQueryResultsResponse{JobReference: job})
				return
			}
			respondJSON(w, &bigquery.GetQueryResultsResponse{
				JobComplete:  true,
				JobReference: job,
				Schema:       schema,
				Rows:         []*bigquery.TableRow{row("a", "1")},
			})
		})
		client := newTestBigQueryClient(t, srv)

		gotSchema, gotRows, err := client.query(context.Background(), "my-project", "SELECT 1", nil)
		if err != nil {
			t.Fatalf("query() = %v, want nil error", err)
		}
		if got := polls.Load(); got != 3 {
			t.Errorf("polls = %d, want 3", got)
		}
		if !reflect.DeepEqual(gotSchema, schema) {
			t.Errorf("query() schema = %+v, want %+v", gotSchema, schema)
		}
		if want := []*bigquery.TableRow{row("a", "1")}; !reflect.DeepEqual(gotRows, want) {
			t.Errorf("query() rows = %+v, want %+v", gotRows, want)
		}
	})
	t.Run("ジョブが失敗したら、原因が分かるエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{JobReference: job})
		}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			respondJSON(w, map[string]any{"error": map[string]any{
				"code":    400,
				"message": "Syntax error: Unexpected keyword FROM at [1:8]",
				"errors":  []map[string]any{{"reason": "invalidQuery"}},
			}})
		})
		client := newTestBigQueryClient(t, srv)

		gotSchema, gotRows, err := client.query(context.Background(), "my-project", "SELECT FROM", nil)
		if err == nil {
			t.Fatal("query() = nil error, want error")
		}
		if want := "Syntax error: Unexpected keyword FROM at [1:8]"; !strings.Contains(err.Error(), want) {
			t.Errorf("query() error = %q, want it to contain %q", err, want)
		}
		if gotSchema != nil || gotRows != nil {
			t.Errorf("query() = %+v, %+v on error, want nil", gotSchema, gotRows)
		}
	})
	t.Run("結果が複数ページでも、全行を返す", func(t *testing.T) {
		// The first page comes with the response of the query, and the later ones
		// with the results of the job, each asked with the token of the page before.
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{
				JobComplete:  true,
				JobReference: job,
				Schema:       schema,
				Rows:         []*bigquery.TableRow{row("a", "1")},
				PageToken:    "page2",
			})
		}, func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("location"); got != "US" {
				t.Errorf("location = %q, want %q", got, "US")
			}
			switch token := r.URL.Query().Get("pageToken"); token {
			case "page2":
				respondJSON(w, &bigquery.GetQueryResultsResponse{
					JobComplete: true,
					Rows:        []*bigquery.TableRow{row("b", "2")},
					PageToken:   "page3",
				})
			case "page3":
				respondJSON(w, &bigquery.GetQueryResultsResponse{
					JobComplete: true,
					Rows:        []*bigquery.TableRow{row("c", "3")},
				})
			default:
				t.Errorf("pageToken = %q, want page2 or page3", token)
				w.WriteHeader(http.StatusBadRequest)
			}
		})
		client := newTestBigQueryClient(t, srv)

		_, gotRows, err := client.query(context.Background(), "my-project", "SELECT 1", nil)
		if err != nil {
			t.Fatalf("query() = %v, want nil error", err)
		}
		want := []*bigquery.TableRow{row("a", "1"), row("b", "2"), row("c", "3")}
		if !reflect.DeepEqual(gotRows, want) {
			t.Errorf("query() rows = %+v, want %+v", gotRows, want)
		}
	})
	t.Run("待っている間にコンテキストがキャンセルされたら戻る", func(t *testing.T) {
		// The job never gets done. The interval is long, so query is waiting for
		// it, not asking the API, when the context is canceled.
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{JobReference: job})
		}, nil)
		client := newTestBigQueryClient(t, srv)
		client.pollInterval = time.Hour

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		time.AfterFunc(50*time.Millisecond, cancel)

		_, _, err := client.query(ctx, "my-project", "SELECT 1", nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("query() error = %v, want context.Canceled", err)
		}
	})
	t.Run("GoogleSQL で実行する", func(t *testing.T) {
		// The API runs a query in legacy SQL unless useLegacySql is false, and a
		// false that is left out of the request is not sent.
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			bodies <- body
			respondJSON(w, &bigquery.QueryResponse{JobComplete: true, JobReference: job})
		}, nil)
		client := newTestBigQueryClient(t, srv)

		if _, _, err := client.query(context.Background(), "my-project", "SELECT 1", nil); err != nil {
			t.Fatalf("query() = %v, want nil error", err)
		}
		body := <-bodies
		if got, ok := body["useLegacySql"]; !ok || got != false {
			t.Errorf("useLegacySql = %v (sent: %t), want false", got, ok)
		}
		if got := body["query"]; got != "SELECT 1" {
			t.Errorf("query = %v, want %q", got, "SELECT 1")
		}
	})
	t.Run("パラメータは名前付きパラメータとして送る", func(t *testing.T) {
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			bodies <- body
			respondJSON(w, &bigquery.QueryResponse{JobComplete: true, JobReference: job})
		}, nil)
		client := newTestBigQueryClient(t, srv)

		params := []*bigquery.QueryParameter{{
			Name:           "id",
			ParameterType:  &bigquery.QueryParameterType{Type: "STRING"},
			ParameterValue: &bigquery.QueryParameterValue{Value: "x"},
		}}
		if _, _, err := client.query(context.Background(), "my-project", "SELECT @id", params); err != nil {
			t.Fatalf("query() = %v, want nil error", err)
		}
		body := <-bodies
		if got := body["parameterMode"]; got != "NAMED" {
			t.Errorf("parameterMode = %v, want NAMED", got)
		}
		if got := body["query"]; got != "SELECT @id" {
			t.Errorf("query = %v, want %q", got, "SELECT @id")
		}
		sent, _ := body["queryParameters"].([]any)
		if len(sent) != 1 {
			t.Fatalf("queryParameters = %v, want 1 parameter", body["queryParameters"])
		}
		p, _ := sent[0].(map[string]any)
		value, _ := p["parameterValue"].(map[string]any)
		if p["name"] != "id" || value["value"] != "x" {
			t.Errorf("queryParameters[0] = %v, want name id and value x", p)
		}
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}, nil)
		client := newTestBigQueryClient(t, srv)

		gotSchema, gotRows, err := client.query(context.Background(), "my-project", "SELECT 1", nil)
		if err == nil {
			t.Fatal("query() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "query") {
			t.Errorf("query() error = %q, want it to contain %q", err, "query")
		}
		if gotSchema != nil || gotRows != nil {
			t.Errorf("query() = %+v, %+v on error, want nil", gotSchema, gotRows)
		}
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}, nil)
		client := newTestBigQueryClient(t, srv)

		_, _, err := client.query(context.Background(), "my-project", "SELECT 1", nil)
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) {
			t.Fatalf("query() error = %v, want it to wrap a *googleapi.Error", err)
		}
		if gerr.Code != http.StatusForbidden {
			t.Errorf("googleapi.Error.Code = %d, want %d", gerr.Code, http.StatusForbidden)
		}
	})
}

func TestSchemaMatchesTerraform(t *testing.T) {
	t.Run("Terraform の project_creation_progress テーブル定義と Go の ProjectCreationProgressRow が一致する", func(t *testing.T) {
		// Terraform のテーブル定義をパースして、ProjectCreationProgressRow と
		// CreateProgress・UpdateProgress が書く列の名前、型、NULL 可否と照合する。
		t.Skip("not implemented")
	})
}

// recordDML answers every query with a finished job that changed affected
// rows, and sends each request body to bodies.
func recordDML(job *bigquery.JobReference, affected int64, bodies chan<- map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		bodies <- body
		respondJSON(w, &bigquery.QueryResponse{JobComplete: true, JobReference: job, NumDmlAffectedRows: affected})
	}
}

// sentParams returns the named parameters of a query request body as name to value.
func sentParams(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	if got := body["parameterMode"]; got != "NAMED" {
		t.Errorf("parameterMode = %v, want NAMED", got)
	}
	params := map[string]any{}
	list, _ := body["queryParameters"].([]any)
	for _, p := range list {
		p, _ := p.(map[string]any)
		value, _ := p["parameterValue"].(map[string]any)
		params[p["name"].(string)] = value["value"]
	}
	return params
}

func testProgressRow() ProjectCreationProgressRow {
	return ProjectCreationProgressRow{
		RequestID:      "request-1",
		Revision:       1,
		State:          "creating_project",
		Status:         "in_progress",
		DisplayName:    "Example Project",
		Requester:      "requester@example.com",
		DeveloperGroup: "developer-group@example.com",
	}
}

func TestBigQueryClient_CreateProgress(t *testing.T) {
	job := &bigquery.JobReference{ProjectId: "my-project", JobId: "job1", Location: "US"}

	t.Run("MERGEで、値をクエリパラメータとして送って1行挿入する", func(t *testing.T) {
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, recordDML(job, 1, bodies), nil)

		if err := newTestBigQueryClient(t, srv).CreateProgress(context.Background(), "my-project", testProgressRow()); err != nil {
			t.Fatalf("CreateProgress() = %v, want nil error", err)
		}
		body := <-bodies
		sql, _ := body["query"].(string)
		// MERGE, not INSERT: BigQuery checks concurrent MERGEs for conflicts.
		if !strings.HasPrefix(sql, "MERGE `my-project.auto_provision.project_creation_progress`") {
			t.Errorf("query = %q, want a MERGE into my-project.auto_provision.project_creation_progress", sql)
		}
		if strings.Contains(sql, "request-1") || strings.Contains(sql, "Example Project") {
			t.Errorf("query = %q, want the values sent as parameters, not in the SQL", sql)
		}
		params := sentParams(t, body)
		want := map[string]any{
			"request_id": "request-1", "revision": "1", "state": "creating_project", "status": "in_progress",
			"project_id": "", "display_name": "Example Project", "requester": "requester@example.com",
			"developer_group": "developer-group@example.com", "error_message": "",
		}
		if !reflect.DeepEqual(params, want) {
			t.Errorf("parameters = %v, want %v", params, want)
		}
	})

	t.Run("ジョブが最初の応答で終わらなくても、結果の変更行数で判定する", func(t *testing.T) {
		// The first response has no count yet. Only the finished results say 1
		// row was inserted, so reading the first response's 0 would wrongly
		// report ErrProgressExists.
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.QueryResponse{JobComplete: false, JobReference: job})
		}, func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, &bigquery.GetQueryResultsResponse{JobComplete: true, JobReference: job, NumDmlAffectedRows: 1})
		})

		if err := newTestBigQueryClient(t, srv).CreateProgress(context.Background(), "my-project", testProgressRow()); err != nil {
			t.Errorf("CreateProgress(slow job) = %v, want nil error", err)
		}
	})

	t.Run("変更された行が0ならErrProgressExistsを返す", func(t *testing.T) {
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, recordDML(job, 0, bodies), nil)

		err := newTestBigQueryClient(t, srv).CreateProgress(context.Background(), "my-project", testProgressRow())
		if !errors.Is(err, ErrProgressExists) {
			t.Errorf("CreateProgress(recorded ID) = %v, want ErrProgressExists", err)
		}
	})

	t.Run("不正なプロジェクトIDは、ジョブを送らずに拒否する", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, nil, nil)

		if err := newTestBigQueryClient(t, srv).CreateProgress(context.Background(), "my-project`; DROP TABLE x; --", testProgressRow()); err == nil {
			t.Error("CreateProgress(invalid project ID) = nil error, want an error")
		}
	})

	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}, nil)

		err := newTestBigQueryClient(t, srv).CreateProgress(context.Background(), "my-project", testProgressRow())
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) || gerr.Code != http.StatusNotFound {
			t.Errorf("CreateProgress() = %v, want a wrapped 404", err)
		}
	})
}

func TestBigQueryClient_UpdateProgress(t *testing.T) {
	job := &bigquery.JobReference{ProjectId: "my-project", JobId: "job1", Location: "US"}
	current := testProgressRow()
	next := current
	next.State = "adding_service_account_iam_bindings"
	next.ProjectID = "example-project"

	t.Run("currentの状態のときだけ、nextの値で更新する", func(t *testing.T) {
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, recordDML(job, 1, bodies), nil)

		if err := newTestBigQueryClient(t, srv).UpdateProgress(context.Background(), "my-project", current, next); err != nil {
			t.Fatalf("UpdateProgress() = %v, want nil error", err)
		}
		body := <-bodies
		sql, _ := body["query"].(string)
		if !strings.HasPrefix(sql, "UPDATE `my-project.auto_provision.project_creation_progress`") {
			t.Errorf("query = %q, want an UPDATE of my-project.auto_provision.project_creation_progress", sql)
		}
		// The WHERE must compare with current, or a concurrent change would be overwritten.
		if !strings.Contains(sql, "state = @current_state AND status = @current_status") {
			t.Errorf("query = %q, want it to require the current state and status", sql)
		}
		params := sentParams(t, body)
		if params["state"] != next.State || params["project_id"] != next.ProjectID {
			t.Errorf("parameters state/project_id = %v/%v, want next's %q/%q", params["state"], params["project_id"], next.State, next.ProjectID)
		}
		if params["current_state"] != current.State || params["current_status"] != current.Status {
			t.Errorf("parameters current_state/current_status = %v/%v, want %q/%q", params["current_state"], params["current_status"], current.State, current.Status)
		}
	})

	t.Run("変更された行が0ならErrProgressChangedを返す", func(t *testing.T) {
		bodies := make(chan map[string]any, 1)
		srv := newFakeBigQueryJobsServer(t, recordDML(job, 0, bodies), nil)

		err := newTestBigQueryClient(t, srv).UpdateProgress(context.Background(), "my-project", current, next)
		if !errors.Is(err, ErrProgressChanged) {
			t.Errorf("UpdateProgress(row moved on) = %v, want ErrProgressChanged", err)
		}
	})

	t.Run("currentとnextのリクエストIDが違えば、ジョブを送らずに拒否する", func(t *testing.T) {
		srv := newFakeBigQueryJobsServer(t, nil, nil)
		other := next
		other.RequestID = "request-2"

		if err := newTestBigQueryClient(t, srv).UpdateProgress(context.Background(), "my-project", current, other); err == nil {
			t.Error("UpdateProgress(different request IDs) = nil error, want an error")
		}
	})
}
