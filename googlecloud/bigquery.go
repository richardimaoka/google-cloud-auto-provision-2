package googlecloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
		return nil, nil, fmt.Errorf("query: %w", err)
	}

	// The response of the query has the first page if the job is done in time.
	// If not, the results of the job have it once the job is done.
	job := resp.JobReference
	schema, rows, token := resp.Schema, resp.Rows, resp.PageToken
	for done := resp.JobComplete; !done; {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("query: %w", ctx.Err())
		case <-time.After(c.pollInterval):
		}

		res, err := c.service.Jobs.GetQueryResults(projectID, job.JobId).Location(job.Location).Context(ctx).Do()
		if err != nil {
			return nil, nil, fmt.Errorf("query: get results: %w", err)
		}
		done = res.JobComplete
		schema, rows, token = res.Schema, res.Rows, res.PageToken
	}

	for token != "" {
		res, err := c.service.Jobs.GetQueryResults(projectID, job.JobId).Location(job.Location).PageToken(token).Context(ctx).Do()
		if err != nil {
			return nil, nil, fmt.Errorf("query: get results: %w", err)
		}
		rows = append(rows, res.Rows...)
		token = res.PageToken
	}
	return schema, rows, nil
}
