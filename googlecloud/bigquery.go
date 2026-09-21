package googlecloud

import (
	"context"
	"fmt"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
)

// Dataset represents a BigQuery dataset.
type Dataset struct {
	// ID is the dataset ID, like "my_dataset".
	ID string
	// Location is where the dataset's data is stored, like "US".
	Location string
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
