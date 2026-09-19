package gcp

import (
	"context"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/option"
)

// Project represents a Google Cloud project.
type Project struct {
	// ID is the globally unique project ID, like "my-project-123".
	ID string
	// Name is the display name.
	Name string
	// Parent is "organizations/<id>" or "folders/<id>", or empty if the project
	// has no parent.
	Parent string
	// State is "ACTIVE" or "DELETE_REQUESTED".
	State string
}

// ProjectsClient is a client for Google Cloud projects.
type ProjectsClient struct {
	service *cloudresourcemanager.Service
	parent  string
}

// NewProjectsClient returns a client whose Create puts new projects under
// parent, which is "organizations/<id>" or "folders/<id>". An empty parent
// creates projects with no parent.
func NewProjectsClient(ctx context.Context, parent string, opts ...option.ClientOption) (*ProjectsClient, error) {
	return nil, nil
}

// Get returns the project with the given project ID.
func (c *ProjectsClient) Get(ctx context.Context, id string) (*Project, error) {
	return nil, nil
}

// Create creates a project with the given display name under the client's
// parent. It blocks until the long-running create operation finishes, and
// returns the created project.
//
// The API requires a project ID and doesn't generate one. The first attempt
// uses name as the ID, so name must be valid as a project ID. If the API
// reports that the ID is already taken, Create retries with name plus a random
// suffix, using a new suffix each time, for at most 5 retries (6 attempts in
// total). Any other failure is returned at once without retrying, and so is the
// last conflict once the attempts run out.
func (c *ProjectsClient) Create(ctx context.Context, name string) (*Project, error) {
	return nil, nil
}

// generateProjectID derives a valid project ID from a display name: the name
// lowercased, with runs of other characters turned into hyphens, followed by a
// random suffix so the globally unique ID is unlikely to collide. A valid ID is
// 6 to 30 lowercase letters, digits or hyphens, starts with a letter, and has
// no trailing hyphen.
func generateProjectID(name string) string {
	return ""
}

// Delete marks the project for deletion, which moves it to the DELETE_REQUESTED
// state. It blocks until the long-running delete operation finishes.
func (c *ProjectsClient) Delete(ctx context.Context, id string) error {
	return nil
}
