// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// InitiateProjectCreationRequest is the input for initiating target project
// creation.
type InitiateProjectCreationRequest struct {
	// ID is issued by the caller and doubles as an idempotency key: retrying
	// with the same ID never starts a second project creation.
	ID domain.ProjectCreationID
}

// InitiateProjectCreationResponse is the result of initiating target project
// creation.
type InitiateProjectCreationResponse struct {
	// ID identifies the accepted project creation. It is the ID of the request.
	ID domain.ProjectCreationID
}

// InitiateProjectCreation accepts a project creation request and returns once
// it is recorded. The rest of the initial setup runs in a Cloud Run job (see
// package worker), and its progress is read with GetProjectCreationProgress.
// The returned error means the request was not accepted.
func InitiateProjectCreation(ctx context.Context, request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	// step:
	//   引数からdomain.ProjectCreationRequestを構成し、バリデーション。
	//   構成に必要な情報はGoogle Cloudから取得。
	//   問題があれば、BigQueryには記録せず、問題と推奨解決法とともにエラーを返却。
	//   (ProjectCreationProgressはdomain.ProjectCreationRequestを必須で持つため、構成前には記録できない)

	// step:
	//   以下をBigQueryに記録。
	//     - ProjectCreationProgress, State = "creating_project", status = "in_progress"
	//   request.IDが記録済みなら重複申請なので、記録も後続処理の起動もせず、記録済みのIDで成功を返却。
	//   ただし記録済みのrequestと中身が違う場合はエラーを返却。

	// step:
	//   request.IDを渡して、Cloud Run Jobsのworker.RunProjectCreationを起動。

	return InitiateProjectCreationResponse{ID: request.ID}, nil
}

// GetProjectCreationProgressResponse is the result of looking up a project
// creation's progress.
type GetProjectCreationProgressResponse struct{}

// GetProjectCreationProgress returns the progress for a project creation request.
func GetProjectCreationProgress(id domain.ProjectCreationID) (GetProjectCreationProgressResponse, error) {
	return GetProjectCreationProgressResponse{}, nil
}

// RestartProjectCreationResponse is the result of restarting a project
// creation request.
type RestartProjectCreationResponse struct{}

// RestartProjectCreation resumes a project creation request after an error.
func RestartProjectCreation(id domain.ProjectCreationID) (RestartProjectCreationResponse, error) {
	return RestartProjectCreationResponse{}, nil
}
