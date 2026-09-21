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
	StateInitiatingProjectCreation       State = "initiating_project_creation"
	StateCreatingProject                 State = "creating_project"
	StateAddingBreakGlassGroups          State = "adding_break_glass_groups"
	StateAddingServiceAccountPermissions State = "adding_service_account_permissions"
	StateAddingPAMEntitlements           State = "adding_pam_entitlements"
	StateInitialSetupDone                State = "initial_setup_done"
)

// states lists the states in the order the setup visits them.
var states = []State{
	StateInitiatingProjectCreation,
	StateCreatingProject,
	StateAddingBreakGlassGroups,
	StateAddingServiceAccountPermissions,
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

// SetupProgress is the persisted state of a project creation request. It is
// immutable: Advance, Fail and Retry return the next SetupProgress.
//
// The valid combinations of State and Status are:
//   - StateInitialSetupDone with StatusSuccess
//   - any other State with StatusInProgress or StatusError
type SetupProgress struct {
	id      ProjectCreationID
	request ProjectCreationRequest
	state   State
	status  Status
}

// NewSetupProgress returns a SetupProgress, or an error if state is unknown or
// status is not valid for state, or id is empty. Use it to restore a
// SetupProgress that was stored.
func NewSetupProgress(id ProjectCreationID, request ProjectCreationRequest, state State, status Status) (SetupProgress, error) {
	if id == "" {
		return SetupProgress{}, errors.New("project creation ID is empty")
	}
	if !slices.Contains(states, state) {
		return SetupProgress{}, fmt.Errorf("unknown state %q", state)
	}
	if state == StateInitialSetupDone {
		if status != StatusSuccess {
			return SetupProgress{}, fmt.Errorf("invalid status %q for state %q: must be %q", status, state, StatusSuccess)
		}
	} else if status != StatusInProgress && status != StatusError {
		return SetupProgress{}, fmt.Errorf("invalid status %q for state %q: must be %q or %q", status, state, StatusInProgress, StatusError)
	}
	return SetupProgress{id: id, request: cloneProjectCreationRequest(request), state: state, status: status}, nil
}

// ID returns the identifier of the project creation request.
func (p SetupProgress) ID() ProjectCreationID { return p.id }

// Request returns a copy of the project creation request being processed.
func (p SetupProgress) Request() ProjectCreationRequest {
	return cloneProjectCreationRequest(p.request)
}

func (p SetupProgress) State() State { return p.state }

func (p SetupProgress) Status() Status { return p.status }

// IsDone reports whether the initial setup has finished.
func (p SetupProgress) IsDone() bool { return p.state == StateInitialSetupDone }

// Advance returns the SetupProgress after the current operation succeeded: the
// next State with StatusInProgress, or StateInitialSetupDone with StatusSuccess
// after the last operation. It returns an error unless p is StatusInProgress.
func (p SetupProgress) Advance() (SetupProgress, error) {
	if p.status != StatusInProgress {
		return SetupProgress{}, fmt.Errorf("cannot advance from state %q with status %q: it is not in progress", p.state, p.status)
	}
	// StatusInProgress is never paired with StateInitialSetupDone, the last state,
	// so there is always a next one.
	next := states[slices.Index(states, p.state)+1]
	if next == StateInitialSetupDone {
		return SetupProgress{id: p.id, request: p.request, state: next, status: StatusSuccess}, nil
	}
	return SetupProgress{id: p.id, request: p.request, state: next, status: StatusInProgress}, nil
}

// Fail returns the SetupProgress after the current operation failed: the same
// State with StatusError. It returns an error unless p is StatusInProgress.
func (p SetupProgress) Fail() (SetupProgress, error) {
	if p.status != StatusInProgress {
		return SetupProgress{}, fmt.Errorf("cannot fail state %q with status %q: it is not in progress", p.state, p.status)
	}
	return SetupProgress{id: p.id, request: p.request, state: p.state, status: StatusError}, nil
}

// Retry returns the SetupProgress after the failed operation is tried again: the
// same State with StatusInProgress. It returns an error unless p is StatusError.
func (p SetupProgress) Retry() (SetupProgress, error) {
	if p.status != StatusError {
		return SetupProgress{}, fmt.Errorf("cannot retry state %q with status %q: it is not an error", p.state, p.status)
	}
	return SetupProgress{id: p.id, request: p.request, state: p.state, status: StatusInProgress}, nil
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
