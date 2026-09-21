// Package orchestrator coordinates Google Cloud project provisioning.
package orchestrator

import "github.com/yusni/google-cloud-auto-provision-2/domain"

// InitiateProjectCreationResponse is the result of initiating target project
// creation.
type InitiateProjectCreationResponse struct{}

// InitiateProjectCreation starts the initial setup for a target project.
func InitiateProjectCreation(request domain.ProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	return InitiateProjectCreationResponse{}, nil
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
