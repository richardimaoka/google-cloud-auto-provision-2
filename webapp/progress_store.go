package webapp

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
)

// bigQueryProgressStore is the progressStore backed by the BigQuery
// project_creation_progress table.
type bigQueryProgressStore struct {
	client    *googlecloud.BigQueryClient
	projectID string
	datasetID string
}

func (s *bigQueryProgressStore) Create(ctx context.Context, progress domain.ProjectCreationProgress) (domain.ProjectCreationID, error) {
	if err := s.client.CreateProgress(ctx, s.projectID, s.datasetID, progressRow(progress)); err != nil {
		return "", err
	}
	return progress.ID(), nil
}

func (s *bigQueryProgressStore) Update(ctx context.Context, current, next domain.ProjectCreationProgress) error {
	return s.client.UpdateProgress(ctx, s.projectID, s.datasetID, progressRow(current), progressRow(next))
}

// progressRow converts progress to a table row.
func progressRow(progress domain.ProjectCreationProgress) googlecloud.ProjectCreationProgressRow {
	request := progress.Request()
	return googlecloud.ProjectCreationProgressRow{
		RequestID:      string(progress.ID()),
		Revision:       int64(progress.Revision()),
		State:          string(progress.State()),
		Status:         string(progress.Status()),
		ProjectID:      progress.ProjectID(),
		DisplayName:    request.DisplayName,
		Requester:      request.Requester.String(),
		DeveloperGroup: request.DeveloperGroup.Email().String(),
	}
}
