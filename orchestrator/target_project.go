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

// GetSetupProgressResponse is the result of looking up a project creation's
// setup progress.
type GetSetupProgressResponse struct{}

// GetSetupProgress returns the setup progress for a project creation request.
func GetSetupProgress(id domain.ProjectCreationID) (GetSetupProgressResponse, error) {
	return GetSetupProgressResponse{}, nil
}

// RestartProjectCreationResponse is the result of restarting a project
// creation request.
type RestartProjectCreationResponse struct{}

// RestartProjectCreation resumes a project creation request after an error.
func RestartProjectCreation(id domain.ProjectCreationID) (RestartProjectCreationResponse, error) {
	return RestartProjectCreationResponse{}, nil
}
