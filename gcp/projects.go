package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	// maxProjectIDLen is the longest project ID the API accepts.
	maxProjectIDLen = 30
	// suffixDigits is the length of the random suffix, before it is shortened to
	// fit maxProjectIDLen.
	suffixDigits = 6
	// defaultPollInterval is how long to wait between polls of a long-running
	// operation. Creating a project usually takes a few seconds.
	defaultPollInterval = time.Second
	// maxCreateAttempts is the initial attempt to create a project plus 5
	// retries with a different project ID.
	maxCreateAttempts = 6
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
	service      *cloudresourcemanager.Service
	pollInterval time.Duration
	// nextID picks the project ID for each attempt of Create. Tests replace it
	// to script the IDs.
	nextID func(name string, tried []string) (string, bool)
}

func NewProjectsClient(ctx context.Context, opts ...option.ClientOption) (*ProjectsClient, error) {
	svc, err := cloudresourcemanager.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new cloudresourcemanager service: %w", err)
	}
	return &ProjectsClient{
		service:      svc,
		pollInterval: defaultPollInterval,
		nextID:       nextProjectID,
	}, nil
}

func projectFromAPI(p *cloudresourcemanager.Project) *Project {
	return &Project{
		ID:     p.ProjectId,
		Name:   p.DisplayName,
		Parent: p.Parent,
		State:  p.State,
	}
}

// Get returns the project with the given project ID.
func (c *ProjectsClient) Get(ctx context.Context, id string) (*Project, error) {
	p, err := c.service.Projects.Get("projects/" + id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return projectFromAPI(p), nil
}

// Create creates a project with the given display name. It blocks until the
// long-running create operation finishes, and returns the created project.
//
// Create doesn't set a parent, so the API picks one. The docs say a parent "is
// selected automatically if applicable based on the user account's domain", so
// where the project ends up depends on who calls Create.
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
	var tried []string
	var conflict error
	for range maxCreateAttempts {
		id, ok := c.nextID(name, tried)
		if !ok {
			break
		}

		op, err := c.service.Projects.Create(&cloudresourcemanager.Project{
			ProjectId:   id,
			DisplayName: name,
		}).Context(ctx).Do()
		if isConflict(err) {
			tried = append(tried, id)
			conflict = err
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create project: %w", err)
		}

		done, err := c.waitOperation(ctx, op)
		if err != nil {
			return nil, fmt.Errorf("create project: %w", err)
		}
		var p cloudresourcemanager.Project
		if err := json.Unmarshal(done.Response, &p); err != nil {
			return nil, fmt.Errorf("create project: parse operation response: %w", err)
		}
		return projectFromAPI(&p), nil
	}
	return nil, fmt.Errorf("create project: no free project ID for %q after %d attempts: %w", name, len(tried), conflict)
}

// isConflict reports whether err is the API's answer that the project ID is
// already taken: an immediate 409 ALREADY_EXISTS on the create request.
func isConflict(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusConflict
}

// nextProjectID returns the next project ID for Create to try, given the IDs
// already tried. The first candidate is name itself. Later ones are name, a
// hyphen and a random suffix of 6 digits, shortened so the whole ID fits in 30
// characters, and never equal to an ID in tried. It returns false when there is
// no later candidate, which is the case when not even a one-digit suffix fits.
func nextProjectID(name string, tried []string) (string, bool) {
	if len(tried) == 0 {
		return name, true
	}

	// The suffix needs a hyphen and at least one digit.
	digits := min(suffixDigits, maxProjectIDLen-len(name)-1)
	if digits < 1 {
		return "", false
	}

	// Start at a random suffix and move on only if it was already tried, so a
	// short suffix with few values left still finds the free ones, and the loop
	// ends once every value has been tried.
	limit := 1
	for range digits {
		limit *= 10
	}
	start := rand.IntN(limit)
	for i := range limit {
		id := fmt.Sprintf("%s-%0*d", name, digits, (start+i)%limit)
		if !slices.Contains(tried, id) {
			return id, true
		}
	}
	return "", false
}

// Delete marks the project for deletion, which moves it to the DELETE_REQUESTED
// state. It blocks until the long-running delete operation finishes.
func (c *ProjectsClient) Delete(ctx context.Context, id string) error {
	op, err := c.service.Projects.Delete("projects/" + id).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if _, err := c.waitOperation(ctx, op); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// waitOperation polls op until it is done and returns the finished operation.
// It returns an error if a poll fails, if ctx ends first, or if the operation
// finishes with an error.
func (c *ProjectsClient) waitOperation(ctx context.Context, op *cloudresourcemanager.Operation) (*cloudresourcemanager.Operation, error) {
	for !op.Done {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.pollInterval):
		}

		var err error
		op, err = c.service.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("poll operation: %w", err)
		}
	}
	if op.Error != nil {
		return nil, fmt.Errorf("operation failed: %s (code %d)", op.Error.Message, op.Error.Code)
	}
	return op, nil
}
