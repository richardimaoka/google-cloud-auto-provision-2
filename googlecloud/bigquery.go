package googlecloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	bigquery "google.golang.org/api/bigquery/v2"
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
}

func NewBigQueryClient(ctx context.Context, opts ...option.ClientOption) (*BigQueryClient, error) {
	svc, err := bigquery.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new bigquery service: %w", err)
	}
	return &BigQueryClient{service: svc}, nil
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
