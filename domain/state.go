package domain

import (
	"errors"
	"fmt"
	"slices"
)

// State is a step of the initial setup that follows an accepted
// ProjectCreationRequest.
//
// Every state but the last is named in the present participle, such as
// "creating_project", and means that operation is running right now. Each lasts
// only a few minutes. There is deliberately no state like "created_project" for
// the moment between two operations: it would last an instant, and recording it
// would cost an update for nothing. The last state, "initial_setup_done", is a
// past participle because nothing is left to do.
type State string

const (
	StateCreatingProject       State = "creating_project"
	StateAddingIAMBindings     State = "adding_iam_bindings"
	StateAddingPAMEntitlements State = "adding_pam_entitlements"
	StateInitialSetupDone      State = "initial_setup_done"
)

// states lists the states in the order the setup visits them.
var states = []State{
	StateCreatingProject,
	StateAddingIAMBindings,
	StateAddingPAMEntitlements,
	StateInitialSetupDone,
}

// Status is how a State is going.
type Status string

const (
	// StatusInProgress is the normal status of a State other than StateInitialSetupDone.
	StatusInProgress Status = "in_progress"
	// StatusError is the status of a State other than StateInitialSetupDone whose
	// operation failed.
	StatusError Status = "error"
	// StatusSuccess is the only status of StateInitialSetupDone.
	StatusSuccess Status = "success"
)

// ProjectCreationID identifies a project creation request.
type ProjectCreationID string

// ProjectCreationProgress is the persisted state of a project creation request.
// It is immutable: Advance, AdvancePastCreation, Fail, Retry and Amend return
// the next ProjectCreationProgress.
//
// The valid combinations of State and Status are:
//   - StateInitialSetupDone with StatusSuccess
//   - any other State with StatusInProgress or StatusError
//
// ProjectID is "" while State is StateCreatingProject — the Google Cloud
// project doesn't exist yet — and non-empty for every other State.
type ProjectCreationProgress struct {
	revision  int
	id        ProjectCreationID
	request   ProjectCreationRequest
	state     State
	status    Status
	projectID string
}

// NewProjectCreationProgress returns a ProjectCreationProgress, or an error if state is unknown,
// status is not valid for state, id is empty, revision is less than one, or projectID doesn't
// match state as described on ProjectCreationProgress. Use it to restore a ProjectCreationProgress
// that was stored.
func NewProjectCreationProgress(id ProjectCreationID, revision int, request ProjectCreationRequest, state State, status Status, projectID string) (ProjectCreationProgress, error) {
	if id == "" {
		return ProjectCreationProgress{}, errors.New("project creation ID is empty")
	}
	if revision < 1 {
		return ProjectCreationProgress{}, fmt.Errorf("project creation revision is %d, want at least 1", revision)
	}
	if !slices.Contains(states, state) {
		return ProjectCreationProgress{}, fmt.Errorf("unknown state %q", state)
	}
	if state == StateInitialSetupDone {
		if status != StatusSuccess {
			return ProjectCreationProgress{}, fmt.Errorf("invalid status %q for state %q: must be %q", status, state, StatusSuccess)
		}
	} else if status != StatusInProgress && status != StatusError {
		return ProjectCreationProgress{}, fmt.Errorf("invalid status %q for state %q: must be %q or %q", status, state, StatusInProgress, StatusError)
	}
	if state == StateCreatingProject {
		if projectID != "" {
			return ProjectCreationProgress{}, fmt.Errorf("project ID %q given for state %q: the project doesn't exist yet at this state", projectID, state)
		}
	} else if projectID == "" {
		return ProjectCreationProgress{}, fmt.Errorf("project ID is empty for state %q: it must be known by this state", state)
	}
	return ProjectCreationProgress{revision: revision, id: id, request: cloneProjectCreationRequest(request), state: state, status: status, projectID: projectID}, nil
}

// ID returns the identifier of the project creation request.
func (p ProjectCreationProgress) ID() ProjectCreationID { return p.id }

// Revision returns the one-based revision of the project creation request.
func (p ProjectCreationProgress) Revision() int { return p.revision }

// Request returns a copy of the project creation request being processed.
func (p ProjectCreationProgress) Request() ProjectCreationRequest {
	return cloneProjectCreationRequest(p.request)
}

func (p ProjectCreationProgress) State() State { return p.state }

func (p ProjectCreationProgress) Status() Status { return p.status }

// ProjectID returns the created Google Cloud project's ID, or "" while State
// is StateCreatingProject.
func (p ProjectCreationProgress) ProjectID() string { return p.projectID }

// IsDone reports whether the initial setup has finished.
func (p ProjectCreationProgress) IsDone() bool { return p.state == StateInitialSetupDone }

// Advance returns the ProjectCreationProgress after the current operation succeeded: the
// next State with StatusInProgress, or StateInitialSetupDone with StatusSuccess
// after the last operation. It returns an error unless p is StatusInProgress.
//
// It also returns an error from StateCreatingProject: leaving that state
// means the Google Cloud project now exists, and its ID has to be recorded in
// the same step, so use AdvancePastCreation there instead.
func (p ProjectCreationProgress) Advance() (ProjectCreationProgress, error) {
	if p.status != StatusInProgress {
		return ProjectCreationProgress{}, fmt.Errorf("cannot advance from state %q with status %q: it is not in progress", p.state, p.status)
	}
	if p.state == StateCreatingProject {
		return ProjectCreationProgress{}, fmt.Errorf("cannot advance from state %q: use AdvancePastCreation, which also records the created project's ID", p.state)
	}
	// StatusInProgress is never paired with StateInitialSetupDone, the last state,
	// so there is always a next one.
	next := states[slices.Index(states, p.state)+1]
	if next == StateInitialSetupDone {
		return ProjectCreationProgress{revision: p.revision, id: p.id, request: p.request, state: next, status: StatusSuccess, projectID: p.projectID}, nil
	}
	return ProjectCreationProgress{revision: p.revision, id: p.id, request: p.request, state: next, status: StatusInProgress, projectID: p.projectID}, nil
}

// AdvancePastCreation returns the ProjectCreationProgress after the Google
// Cloud project was created: StateAddingIAMBindings with StatusInProgress,
// carrying projectID from here on. It returns an error unless p is
// StateCreatingProject with StatusInProgress, or if projectID is empty.
//
// This is the only place a ProjectCreationProgress's ProjectID is set. Every
// other transition uses Advance, which deliberately takes no project ID and
// rejects StateCreatingProject for that reason — a ProjectCreationProgress
// should never leave StateCreatingProject without recording what got created.
func (p ProjectCreationProgress) AdvancePastCreation(projectID string) (ProjectCreationProgress, error) {
	if p.state != StateCreatingProject || p.status != StatusInProgress {
		return ProjectCreationProgress{}, fmt.Errorf("cannot advance past creation from state %q with status %q: it is not %q/%q", p.state, p.status, StateCreatingProject, StatusInProgress)
	}
	if projectID == "" {
		return ProjectCreationProgress{}, errors.New("project ID is empty")
	}
	return ProjectCreationProgress{revision: p.revision, id: p.id, request: p.request, state: StateAddingIAMBindings, status: StatusInProgress, projectID: projectID}, nil
}

// Fail returns the ProjectCreationProgress after the current operation failed: the same
// State with StatusError. It returns an error unless p is StatusInProgress.
func (p ProjectCreationProgress) Fail() (ProjectCreationProgress, error) {
	if p.status != StatusInProgress {
		return ProjectCreationProgress{}, fmt.Errorf("cannot fail state %q with status %q: it is not in progress", p.state, p.status)
	}
	return ProjectCreationProgress{revision: p.revision, id: p.id, request: p.request, state: p.state, status: StatusError, projectID: p.projectID}, nil
}

// Retry returns the ProjectCreationProgress after the failed operation is tried again: the
// same State with StatusInProgress. It returns an error unless p is StatusError.
func (p ProjectCreationProgress) Retry() (ProjectCreationProgress, error) {
	if p.status != StatusError {
		return ProjectCreationProgress{}, fmt.Errorf("cannot retry state %q with status %q: it is not an error", p.state, p.status)
	}
	return ProjectCreationProgress{revision: p.revision, id: p.id, request: p.request, state: p.state, status: StatusInProgress, projectID: p.projectID}, nil
}

// Amend returns the ProjectCreationProgress with request applied. It returns an
// error unless p is StatusError. The amended progress retains its ID, State and
// Status, and has the next Revision.
func (p ProjectCreationProgress) Amend(request ProjectCreationRequest) (ProjectCreationProgress, error) {
	if p.status != StatusError {
		return ProjectCreationProgress{}, fmt.Errorf("cannot amend state %q with status %q: it is not an error", p.state, p.status)
	}
	return ProjectCreationProgress{
		revision:  p.revision + 1,
		id:        p.id,
		request:   cloneProjectCreationRequest(request),
		state:     p.state,
		status:    p.status,
		projectID: p.projectID,
	}, nil
}

func cloneProjectCreationRequest(request ProjectCreationRequest) ProjectCreationRequest {
	return ProjectCreationRequest{
		DisplayName: request.DisplayName,
		Requester:   request.Requester,
		DeveloperGroup: NewGoogleGroup(
			request.DeveloperGroup.Email(),
			request.DeveloperGroup.Members(),
		),
	}
}
