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
