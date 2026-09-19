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
// reports that the ID is already taken, Create retries with name, a hyphen and
// a random suffix of 6 digits, using a new suffix each time, for at most 5
// retries (6 attempts in total). Any other failure is returned at once without
// retrying, and so is the last conflict once the attempts run out.
//
// A project ID is at most 30 characters, so the suffix is shortened to fit. If
// name is so long that not even a one-digit suffix fits, Create makes only the
// first attempt, and returns its error if it fails.
func (c *ProjectsClient) Create(ctx context.Context, name string) (*Project, error) {
	return nil, nil
}

// nextProjectID returns the next project ID for Create to try, given the IDs
// already tried. The first candidate is name itself. Later ones are name, a
// hyphen and a random suffix of 6 digits, shortened so the whole ID fits in 30
// characters, and never equal to an ID in tried. It returns false when there is
// no later candidate, which is the case when not even a one-digit suffix fits.
func nextProjectID(name string, tried []string) (string, bool) {
	return "", false
}

// Delete marks the project for deletion, which moves it to the DELETE_REQUESTED
// state. It blocks until the long-running delete operation finishes.
func (c *ProjectsClient) Delete(ctx context.Context, id string) error {
	return nil
}
