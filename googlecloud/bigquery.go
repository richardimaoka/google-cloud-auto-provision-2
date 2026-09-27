package googlecloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

var (
	// ErrDatasetNotFound is returned when the dataset does not exist.
	ErrDatasetNotFound = errors.New("dataset not found")
	// ErrTableNotFound is returned when the dataset exists but the table does
	// not.
	ErrTableNotFound = errors.New("table not found")
	// ErrProgressExists means CreateProgress found the request ID recorded.
	ErrProgressExists = errors.New("project creation progress already exists")
	// ErrProgressChanged means UpdateProgress found the row no longer at current.
	ErrProgressChanged = errors.New("project creation progress changed")
)

// Dataset represents a BigQuery dataset.
type Dataset struct {
	// ID is the dataset ID, like "my_dataset".
	ID string
	// Location is where the dataset's data is stored, like "US".
	Location string
}

// Table represents a BigQuery table.
type Table struct {
	// ID is the table ID, like "my_table".
	ID string
}

// BigQueryClient is a client for BigQuery.
type BigQueryClient struct {
	service *bigquery.Service
	// pollInterval is how long query waits before it asks again for the results
	// of a job that is not done. Tests replace it.
	pollInterval time.Duration
}

func NewBigQueryClient(ctx context.Context, opts ...option.ClientOption) (*BigQueryClient, error) {
	svc, err := bigquery.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new bigquery service: %w", err)
	}
	return &BigQueryClient{service: svc, pollInterval: defaultPollInterval}, nil
}

// GetDataset returns the dataset with the given ID in the given project.
func (c *BigQueryClient) GetDataset(ctx context.Context, projectID, datasetID string) (*Dataset, error) {
	ds, err := c.service.Datasets.Get(projectID, datasetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get dataset: %w", err)
	}
	return &Dataset{
		ID:       ds.DatasetReference.DatasetId,
		Location: ds.Location,
	}, nil
}

// GetTable returns the table with the given ID in the given dataset.
//
// If the API answers that the table is not found, GetTable asks the API for the
// dataset to tell the two cases apart: it returns ErrDatasetNotFound if the
// dataset is missing, and ErrTableNotFound if the dataset is there.
func (c *BigQueryClient) GetTable(ctx context.Context, projectID, datasetID, tableID string) (*Table, error) {
	t, err := c.service.Tables.Get(projectID, datasetID, tableID).Context(ctx).Do()
	if isHTTPStatus(err, http.StatusNotFound) {
		// If the dataset lookup fails for another reason, the cause of the 404
		// is not known, and the error of the table stays as it is.
		_, derr := c.service.Datasets.Get(projectID, datasetID).Context(ctx).Do()
		switch {
		case isHTTPStatus(derr, http.StatusNotFound):
			return nil, fmt.Errorf("get table: %w: %w", ErrDatasetNotFound, err)
		case derr == nil:
			return nil, fmt.Errorf("get table: %w: %w", ErrTableNotFound, err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("get table: %w", err)
	}
	return &Table{ID: t.TableReference.TableId}, nil
}

// query runs a GoogleSQL statement in the project, waits until the job is done,
// and returns the schema and all the rows of the result. A statement that
// returns no rows, like an INSERT or a MERGE, returns no rows.
//
// params are sent as named query parameters, so the values of the statement
// are @name in sql and never part of the text.
func (c *BigQueryClient) query(ctx context.Context, projectID, sql string, params []*bigquery.QueryParameter) (*bigquery.TableSchema, []*bigquery.TableRow, error) {
	schema, rows, _, err := c.run(ctx, projectID, sql, params)
	return schema, rows, err
}

// exec runs a DML statement and returns how many rows it changed.
func (c *BigQueryClient) exec(ctx context.Context, projectID, sql string, params []*bigquery.QueryParameter) (int64, error) {
	_, _, affected, err := c.run(ctx, projectID, sql, params)
	return affected, err
}

// run backs query and exec.
func (c *BigQueryClient) run(ctx context.Context, projectID, sql string, params []*bigquery.QueryParameter) (*bigquery.TableSchema, []*bigquery.TableRow, int64, error) {
	// The API runs a query in legacy SQL unless useLegacySql is false.
	req := &bigquery.QueryRequest{
		Query:        sql,
		UseLegacySql: googleapi.Bool(false),
	}
	if len(params) > 0 {
		req.QueryParameters = params
		req.ParameterMode = "NAMED"
	}
	resp, err := c.service.Jobs.Query(projectID, req).Context(ctx).Do()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("query: %w", err)
	}

	// The response of the query has the first page if the job is done in time.
	// If not, the results of the job have it once the job is done.
	job := resp.JobReference
	schema, rows, token, affected := resp.Schema, resp.Rows, resp.PageToken, resp.NumDmlAffectedRows
	for done := resp.JobComplete; !done; {
		select {
		case <-ctx.Done():
			return nil, nil, 0, fmt.Errorf("query: %w", ctx.Err())
		case <-time.After(c.pollInterval):
		}

		res, err := c.service.Jobs.GetQueryResults(projectID, job.JobId).Location(job.Location).Context(ctx).Do()
		if err != nil {
			return nil, nil, 0, fmt.Errorf("query: get results: %w", err)
		}
		done = res.JobComplete
		schema, rows, token, affected = res.Schema, res.Rows, res.PageToken, res.NumDmlAffectedRows
	}

	for token != "" {
		res, err := c.service.Jobs.GetQueryResults(projectID, job.JobId).Location(job.Location).PageToken(token).Context(ctx).Do()
		if err != nil {
			return nil, nil, 0, fmt.Errorf("query: get results: %w", err)
		}
		rows = append(rows, res.Rows...)
		token = res.PageToken
	}
	return schema, rows, affected, nil
}

// ProgressDatasetID is the dataset that holds project_creation_progress, in
// every control project. It matches google_bigquery_dataset.app in
// terraform/modules/progress_table.
const ProgressDatasetID = "auto_provision"

// ProjectCreationProgressRow is one row of project_creation_progress. Empty
// ProjectID and ErrorMessage are written as NULL.
type ProjectCreationProgressRow struct {
	RequestID      string
	Revision       int64
	State          string
	Status         string
	ProjectID      string
	DisplayName    string
	Requester      string
	DeveloperGroup string
	ErrorMessage   string
}

// CreateProgress inserts row, or returns ErrProgressExists if its request ID
// is recorded. MERGE, unlike INSERT, is checked for concurrent conflicts.
func (c *BigQueryClient) CreateProgress(ctx context.Context, projectID, datasetID string, row ProjectCreationProgressRow) error {
	if !validBigQueryIdentifier(projectID) || !validBigQueryIdentifier(datasetID) {
		return fmt.Errorf("create progress: invalid project or dataset ID")
	}
	sql := fmt.Sprintf("MERGE `%s.%s.project_creation_progress` T\n"+
		"USING (SELECT @request_id AS request_id) S\n"+
		"ON T.request_id = S.request_id\n"+
		"WHEN NOT MATCHED THEN INSERT (request_id, revision, state, status, project_id, display_name, requester, developer_group, error_message, created_at, updated_at)\n"+
		"VALUES (@request_id, @revision, @state, @status, NULLIF(@project_id, ''), @display_name, @requester, @developer_group, NULLIF(@error_message, ''), CURRENT_TIMESTAMP(), CURRENT_TIMESTAMP())",
		projectID, datasetID)
	affected, err := c.exec(ctx, projectID, sql, progressParams(row))
	if err != nil {
		return fmt.Errorf("create progress: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("create progress %q: %w", row.RequestID, ErrProgressExists)
	}
	return nil
}

// UpdateProgress replaces current with next, or returns ErrProgressChanged if
// the row is no longer at current's state and status.
func (c *BigQueryClient) UpdateProgress(ctx context.Context, projectID, datasetID string, current, next ProjectCreationProgressRow) error {
	if !validBigQueryIdentifier(projectID) || !validBigQueryIdentifier(datasetID) {
		return fmt.Errorf("update progress: invalid project or dataset ID")
	}
	if current.RequestID != next.RequestID {
		return fmt.Errorf("update progress: current request ID %q and next %q differ", current.RequestID, next.RequestID)
	}
	sql := fmt.Sprintf("UPDATE `%s.%s.project_creation_progress`\n"+
		"SET revision = @revision, state = @state, status = @status, project_id = NULLIF(@project_id, ''), display_name = @display_name, requester = @requester, developer_group = @developer_group, error_message = NULLIF(@error_message, ''), updated_at = CURRENT_TIMESTAMP()\n"+
		"WHERE request_id = @request_id AND state = @current_state AND status = @current_status",
		projectID, datasetID)
	params := append(progressParams(next), stringParam("current_state", current.State), stringParam("current_status", current.Status))
	affected, err := c.exec(ctx, projectID, sql, params)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("update progress %q from %s/%s: %w", current.RequestID, current.State, current.Status, ErrProgressChanged)
	}
	return nil
}

// progressParams returns row as named query parameters.
func progressParams(row ProjectCreationProgressRow) []*bigquery.QueryParameter {
	return []*bigquery.QueryParameter{
		stringParam("request_id", row.RequestID),
		{
			Name:           "revision",
			ParameterType:  &bigquery.QueryParameterType{Type: "INT64"},
			ParameterValue: &bigquery.QueryParameterValue{Value: strconv.FormatInt(row.Revision, 10)},
		},
		stringParam("state", row.State),
		stringParam("status", row.Status),
		stringParam("project_id", row.ProjectID),
		stringParam("display_name", row.DisplayName),
		stringParam("requester", row.Requester),
		stringParam("developer_group", row.DeveloperGroup),
		stringParam("error_message", row.ErrorMessage),
	}
}

// stringParam returns a STRING parameter, sending "" explicitly rather than as NULL.
func stringParam(name, value string) *bigquery.QueryParameter {
	return &bigquery.QueryParameter{
		Name:          name,
		ParameterType: &bigquery.QueryParameterType{Type: "STRING"},
		ParameterValue: &bigquery.QueryParameterValue{
			Value:           value,
			ForceSendFields: []string{"Value"},
		},
	}
}

func validBigQueryIdentifier(id string) bool {
	return id != "" && !strings.ContainsAny(id, "`.")
}
