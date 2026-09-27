package domain

import (
	"reflect"
	"testing"
)

func TestNewProjectCreationProgress(t *testing.T) {
	const projectID = "auto-provision-123456"

	tests := []struct {
		name      string
		id        ProjectCreationID
		revision  int
		state     State
		status    Status
		projectID string
		wantErr   bool
	}{
		{name: "-ingの状態はin_progressで作れる", id: "request-123", revision: 1, state: StateCreatingProject, status: StatusInProgress},
		{name: "-ingの状態はerrorでも作れる", id: "request-123", revision: 1, state: StateAddingPAMEntitlements, status: StatusError, projectID: projectID},
		{name: "initial_setup_doneはsuccessで作れる", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusSuccess, projectID: projectID},
		{name: "-ingの状態をsuccessにするとエラー", id: "request-123", revision: 1, state: StateCreatingProject, status: StatusSuccess, wantErr: true},
		{name: "initial_setup_doneをin_progressにするとエラー", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusInProgress, projectID: projectID, wantErr: true},
		{name: "initial_setup_doneをerrorにするとエラー", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusError, projectID: projectID, wantErr: true},
		{name: "知らない状態だとエラー", id: "request-123", revision: 1, state: State("created_project"), status: StatusInProgress, wantErr: true},
		{name: "知らないステータスだとエラー", id: "request-123", revision: 1, state: StateCreatingProject, status: Status("failed"), wantErr: true},
		{name: "状態が空だとエラー", id: "request-123", revision: 1, state: "", status: StatusInProgress, wantErr: true},
		{name: "IDが空だとエラー", revision: 1, state: StateCreatingProject, status: StatusInProgress, wantErr: true},
		{name: "revisionが0だとエラー", id: "request-123", state: StateCreatingProject, status: StatusInProgress, wantErr: true},
		{name: "creating_projectなのにprojectIDがあるとエラー", id: "request-123", revision: 1, state: StateCreatingProject, status: StatusInProgress, projectID: projectID, wantErr: true},
		{name: "creating_project以外でprojectIDが空だとエラー", id: "request-123", revision: 1, state: StateAddingIAMBindings, status: StatusInProgress, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProjectCreationProgress(tt.id, tt.revision, projectCreationRequest(t), tt.state, tt.status, tt.projectID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewProjectCreationProgress(%q, %d, %q, %q, %q) = nil error, want an error", tt.id, tt.revision, tt.state, tt.status, tt.projectID)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProjectCreationProgress(%q, %d, %q, %q, %q): %v", tt.id, tt.revision, tt.state, tt.status, tt.projectID, err)
			}
			if p.ID() != tt.id || p.Revision() != tt.revision || p.State() != tt.state || p.Status() != tt.status || p.ProjectID() != tt.projectID {
				t.Errorf("got %q/%d/%q/%q/%q, want %q/%d/%q/%q/%q", p.ID(), p.Revision(), p.State(), p.Status(), p.ProjectID(), tt.id, tt.revision, tt.state, tt.status, tt.projectID)
			}
		})
	}
}

func projectCreationRequest(t *testing.T) ProjectCreationRequest {
	t.Helper()
	request, err := NewProjectCreationRequest(
		"Example Project",
		email(t, "requester@example.com"),
		NewGoogleGroup(email(t, "developer-group@example.com"), nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

// testProjectID is the placeholder project ID for progress helper calls whose
// state is past StateCreatingProject, where a ProjectCreationProgress must
// carry one.
const testProjectID = "auto-provision-123456"

// progress returns a ProjectCreationProgress, failing the test if it is not valid.
// It fills in testProjectID unless state is StateCreatingProject, where a
// ProjectCreationProgress must not have one yet.
func progress(t *testing.T, state State, status Status) ProjectCreationProgress {
	t.Helper()
	projectID := testProjectID
	if state == StateCreatingProject {
		projectID = ""
	}
	p, err := NewProjectCreationProgress("request-123", 1, projectCreationRequest(t), state, status, projectID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProjectCreationProgress_Advance(t *testing.T) {
	// 正常系では、creating_projectをAdvancePastCreationで抜けた後、最後まで順番に進む。
	want := []struct {
		state  State
		status Status
	}{
		{StateAddingServiceAccountIAMBindings, StatusInProgress},
		{StateAddingIAMBindings, StatusInProgress},
		{StateAddingPAMEntitlements, StatusInProgress},
		{StateInitialSetupDone, StatusSuccess},
	}

	p, err := progress(t, StateCreatingProject, StatusInProgress).AdvancePastCreation(testProjectID)
	if err != nil {
		t.Fatalf("AdvancePastCreation(%q): %v", testProjectID, err)
	}
	if p.State() != want[0].state || p.Status() != want[0].status || p.ProjectID() != testProjectID {
		t.Fatalf("AdvancePastCreation(%q) = %q/%q/%q, want %q/%q/%q", testProjectID, p.State(), p.Status(), p.ProjectID(), want[0].state, want[0].status, testProjectID)
	}
	for i, w := range want[1:] {
		next, err := p.Advance()
		if err != nil {
			t.Fatalf("Advance() from %q/%q: %v", p.State(), p.Status(), err)
		}
		if next.State() != w.state || next.Status() != w.status {
			t.Fatalf("step %d: got %q/%q, want %q/%q", i+1, next.State(), next.Status(), w.state, w.status)
		}
		if next.ID() != p.ID() {
			t.Fatalf("step %d: ID = %q, want %q", i+1, next.ID(), p.ID())
		}
		if next.Revision() != p.Revision() {
			t.Fatalf("step %d: Revision = %d, want %d", i+1, next.Revision(), p.Revision())
		}
		if next.Request().DisplayName != p.Request().DisplayName {
			t.Fatalf("step %d: DisplayName = %q, want %q", i+1, next.Request().DisplayName, p.Request().DisplayName)
		}
		if next.ProjectID() != p.ProjectID() {
			t.Fatalf("step %d: ProjectID = %q, want %q", i+1, next.ProjectID(), p.ProjectID())
		}
		p = next
	}
	if !p.IsDone() {
		t.Errorf("IsDone() = false at the end, want true")
	}

	t.Run("進んでも元のProjectCreationProgressは変わらない", func(t *testing.T) {
		before := progress(t, StateAddingIAMBindings, StatusInProgress)
		if _, err := before.Advance(); err != nil {
			t.Fatal(err)
		}
		if before.State() != StateAddingIAMBindings || before.Status() != StatusInProgress {
			t.Errorf("Advance changed the receiver to %q/%q", before.State(), before.Status())
		}
	})

	t.Run("creating_projectからはAdvanceで進めない", func(t *testing.T) {
		if _, err := progress(t, StateCreatingProject, StatusInProgress).Advance(); err == nil {
			t.Error("Advance() from creating_project = nil error, want an error")
		}
	})

	t.Run("errorからは進めない", func(t *testing.T) {
		if _, err := progress(t, StateAddingIAMBindings, StatusError).Advance(); err == nil {
			t.Error("Advance() from an error = nil error, want an error")
		}
	})

	t.Run("initial_setup_doneからは進めない", func(t *testing.T) {
		if _, err := progress(t, StateInitialSetupDone, StatusSuccess).Advance(); err == nil {
			t.Error("Advance() from the done state = nil error, want an error")
		}
	})
}

func TestProjectCreationProgress_AdvancePastCreation(t *testing.T) {
	t.Run("creating_project/in_progressから、指定したprojectIDでadding_service_account_iam_bindingsに進む", func(t *testing.T) {
		before := progress(t, StateCreatingProject, StatusInProgress)
		after, err := before.AdvancePastCreation(testProjectID)
		if err != nil {
			t.Fatalf("AdvancePastCreation(%q): %v", testProjectID, err)
		}
		if after.State() != StateAddingServiceAccountIAMBindings || after.Status() != StatusInProgress || after.ProjectID() != testProjectID {
			t.Errorf("got %q/%q/%q, want %q/%q/%q", after.State(), after.Status(), after.ProjectID(), StateAddingServiceAccountIAMBindings, StatusInProgress, testProjectID)
		}
		if before.State() != StateCreatingProject || before.ProjectID() != "" {
			t.Errorf("AdvancePastCreation changed the receiver to %q/%q", before.State(), before.ProjectID())
		}
	})

	t.Run("creating_project以外からは進めない", func(t *testing.T) {
		if _, err := progress(t, StateAddingIAMBindings, StatusInProgress).AdvancePastCreation(testProjectID); err == nil {
			t.Error("AdvancePastCreation() from adding_iam_bindings = nil error, want an error")
		}
	})

	t.Run("errorからは進めない", func(t *testing.T) {
		if _, err := progress(t, StateCreatingProject, StatusError).AdvancePastCreation(testProjectID); err == nil {
			t.Error("AdvancePastCreation() from an error = nil error, want an error")
		}
	})

	t.Run("projectIDが空だとエラー", func(t *testing.T) {
		if _, err := progress(t, StateCreatingProject, StatusInProgress).AdvancePastCreation(""); err == nil {
			t.Error("AdvancePastCreation(\"\") = nil error, want an error")
		}
	})
}

func TestProjectCreationProgress_Fail(t *testing.T) {
	t.Run("in_progressの状態は同じ状態のままerrorになる", func(t *testing.T) {
		for _, state := range states[:len(states)-1] {
			p, err := progress(t, state, StatusInProgress).Fail()
			if err != nil {
				t.Fatalf("Fail() from %q: %v", state, err)
			}
			if p.State() != state || p.Status() != StatusError {
				t.Errorf("Fail() from %q gave %q/%q, want %q/%q", state, p.State(), p.Status(), state, StatusError)
			}
		}
	})

	t.Run("すでにerrorだと失敗にできない", func(t *testing.T) {
		if _, err := progress(t, StateCreatingProject, StatusError).Fail(); err == nil {
			t.Error("Fail() from an error = nil error, want an error")
		}
	})

	t.Run("initial_setup_doneは失敗にできない", func(t *testing.T) {
		if _, err := progress(t, StateInitialSetupDone, StatusSuccess).Fail(); err == nil {
			t.Error("Fail() from the done state = nil error, want an error")
		}
	})
}

func TestProjectCreationProgress_Retry(t *testing.T) {
	t.Run("errorの状態は同じ状態のままin_progressに戻る", func(t *testing.T) {
		for _, state := range states[:len(states)-1] {
			p, err := progress(t, state, StatusError).Retry()
			if err != nil {
				t.Fatalf("Retry() from %q: %v", state, err)
			}
			if p.State() != state || p.Status() != StatusInProgress {
				t.Errorf("Retry() from %q gave %q/%q, want %q/%q", state, p.State(), p.Status(), state, StatusInProgress)
			}
		}
	})

	t.Run("in_progressだとリトライできない", func(t *testing.T) {
		if _, err := progress(t, StateCreatingProject, StatusInProgress).Retry(); err == nil {
			t.Error("Retry() from in_progress = nil error, want an error")
		}
	})

	t.Run("initial_setup_doneはリトライできない", func(t *testing.T) {
		if _, err := progress(t, StateInitialSetupDone, StatusSuccess).Retry(); err == nil {
			t.Error("Retry() from the done state = nil error, want an error")
		}
	})

	t.Run("失敗してからリトライすると、同じ状態からまた進める", func(t *testing.T) {
		p := progress(t, StateAddingIAMBindings, StatusInProgress)
		failed, err := p.Fail()
		if err != nil {
			t.Fatal(err)
		}
		retried, err := failed.Retry()
		if err != nil {
			t.Fatal(err)
		}
		next, err := retried.Advance()
		if err != nil {
			t.Fatal(err)
		}
		if next.State() != StateAddingPAMEntitlements || next.Status() != StatusInProgress {
			t.Errorf("got %q/%q, want %q/%q", next.State(), next.Status(), StateAddingPAMEntitlements, StatusInProgress)
		}
	})
}

func TestProjectCreationProgress_Amend(t *testing.T) {
	t.Run("errorのリクエストを修正するとrevisionを増やしてerrorのまま返す", func(t *testing.T) {
		before := progress(t, StateAddingIAMBindings, StatusError)
		amendedRequest, err := NewProjectCreationRequest(
			"Amended Project",
			email(t, "amended-requester@example.com"),
			NewGoogleGroup(email(t, "amended-developer-group@example.com"), nil),
		)
		if err != nil {
			t.Fatal(err)
		}

		after, err := before.Amend(amendedRequest)
		if err != nil {
			t.Fatalf("Amend(): %v", err)
		}
		if after.ID() != before.ID() {
			t.Errorf("ID() = %q, want %q", after.ID(), before.ID())
		}
		if after.Revision() != before.Revision()+1 {
			t.Errorf("Revision() = %d, want %d", after.Revision(), before.Revision()+1)
		}
		if !reflect.DeepEqual(after.Request(), amendedRequest) {
			t.Errorf("Request() = %#v, want %#v", after.Request(), amendedRequest)
		}
		if after.State() != before.State() || after.Status() != StatusError {
			t.Errorf("State()/Status() = %q/%q, want %q/%q", after.State(), after.Status(), before.State(), StatusError)
		}
		if reflect.DeepEqual(before.Request(), amendedRequest) || before.Revision() != 1 || before.Status() != StatusError {
			t.Error("Amend changed the receiver")
		}
	})

	for _, test := range []struct {
		name   string
		state  State
		status Status
	}{
		{name: "in_progressでは修正できない", state: StateCreatingProject, status: StatusInProgress},
		{name: "完了済みでは修正できない", state: StateInitialSetupDone, status: StatusSuccess},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := progress(t, test.state, test.status).Amend(projectCreationRequest(t)); err == nil {
				t.Error("Amend() = nil error, want an error")
			}
		})
	}
}
