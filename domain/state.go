package domain

import (
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
	StateCreatingProject                 State = "creating_project"
	StateAddingBreakGlassGroups          State = "adding_break_glass_groups"
	StateAddingServiceAccountPermissions State = "adding_service_account_permissions"
	StateAddingPAMEntitlements           State = "adding_pam_entitlements"
	StateInitialSetupDone                State = "initial_setup_done"
)

// states lists the states in the order the setup visits them.
var states = []State{
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

// SetupProgress is where the initial setup of a project is. It is immutable:
// Advance, Fail and Retry return the next SetupProgress.
//
// The valid combinations of State and Status are:
//   - StateInitialSetupDone with StatusSuccess
//   - any other State with StatusInProgress or StatusError
type SetupProgress struct {
	state  State
	status Status
}

// NewSetupProgress returns a SetupProgress, or an error if state is unknown or
// status is not valid for state. Use it to restore a SetupProgress that was
// stored.
func NewSetupProgress(state State, status Status) (SetupProgress, error) {
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
	return SetupProgress{state: state, status: status}, nil
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
		return SetupProgress{state: next, status: StatusSuccess}, nil
	}
	return SetupProgress{state: next, status: StatusInProgress}, nil
}

// Fail returns the SetupProgress after the current operation failed: the same
// State with StatusError. It returns an error unless p is StatusInProgress.
func (p SetupProgress) Fail() (SetupProgress, error) {
	if p.status != StatusInProgress {
		return SetupProgress{}, fmt.Errorf("cannot fail state %q with status %q: it is not in progress", p.state, p.status)
	}
	return SetupProgress{state: p.state, status: StatusError}, nil
}

// Retry returns the SetupProgress after the failed operation is tried again: the
// same State with StatusInProgress. It returns an error unless p is StatusError.
func (p SetupProgress) Retry() (SetupProgress, error) {
	if p.status != StatusError {
		return SetupProgress{}, fmt.Errorf("cannot retry state %q with status %q: it is not an error", p.state, p.status)
	}
	return SetupProgress{state: p.state, status: StatusInProgress}, nil
}
