package domain

import "testing"

func TestNewProjectCreationProgress(t *testing.T) {
	tests := []struct {
		name     string
		id       ProjectCreationID
		revision int
		state    State
		status   Status
		wantErr  bool
	}{
		{name: "-ingの状態はin_progressで作れる", id: "request-123", revision: 1, state: StateCreatingProject, status: StatusInProgress},
		{name: "-ingの状態はerrorでも作れる", id: "request-123", revision: 1, state: StateAddingPAMEntitlements, status: StatusError},
		{name: "initial_setup_doneはsuccessで作れる", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusSuccess},
		{name: "-ingの状態をsuccessにするとエラー", id: "request-123", revision: 1, state: StateCreatingProject, status: StatusSuccess, wantErr: true},
		{name: "initial_setup_doneをin_progressにするとエラー", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusInProgress, wantErr: true},
		{name: "initial_setup_doneをerrorにするとエラー", id: "request-123", revision: 1, state: StateInitialSetupDone, status: StatusError, wantErr: true},
		{name: "知らない状態だとエラー", id: "request-123", revision: 1, state: State("created_project"), status: StatusInProgress, wantErr: true},
		{name: "知らないステータスだとエラー", id: "request-123", revision: 1, state: StateCreatingProject, status: Status("failed"), wantErr: true},
		{name: "状態が空だとエラー", id: "request-123", revision: 1, state: "", status: StatusInProgress, wantErr: true},
		{name: "IDが空だとエラー", revision: 1, state: StateInitiatingProjectCreation, status: StatusInProgress, wantErr: true},
		{name: "revisionが0だとエラー", id: "request-123", state: StateInitiatingProjectCreation, status: StatusInProgress, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProjectCreationProgress(tt.id, tt.revision, projectCreationRequest(t), tt.state, tt.status)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewProjectCreationProgress(%q, %d, %q, %q) = nil error, want an error", tt.id, tt.revision, tt.state, tt.status)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProjectCreationProgress(%q, %d, %q, %q): %v", tt.id, tt.revision, tt.state, tt.status, err)
			}
			if p.ID() != tt.id || p.Revision() != tt.revision || p.State() != tt.state || p.Status() != tt.status {
				t.Errorf("got %q/%d/%q/%q, want %q/%d/%q/%q", p.ID(), p.Revision(), p.State(), p.Status(), tt.id, tt.revision, tt.state, tt.status)
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

// progress returns a ProjectCreationProgress, failing the test if it is not valid.
func progress(t *testing.T, state State, status Status) ProjectCreationProgress {
	t.Helper()
	p, err := NewProjectCreationProgress("request-123", 1, projectCreationRequest(t), state, status)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProjectCreationProgress_Advance(t *testing.T) {
	// 正常系では、最初の状態から最後まで順番に進む。
	want := []struct {
		state  State
		status Status
	}{
		{StateInitiatingProjectCreation, StatusInProgress},
		{StateCreatingProject, StatusInProgress},
		{StateAddingBreakGlassGroups, StatusInProgress},
		{StateAddingServiceAccountPermissions, StatusInProgress},
		{StateAddingPAMEntitlements, StatusInProgress},
		{StateInitialSetupDone, StatusSuccess},
	}

	p := progress(t, want[0].state, want[0].status)
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
		p = next
	}
	if !p.IsDone() {
		t.Errorf("IsDone() = false at the end, want true")
	}

	t.Run("進んでも元のProjectCreationProgressは変わらない", func(t *testing.T) {
		before := progress(t, StateCreatingProject, StatusInProgress)
		if _, err := before.Advance(); err != nil {
			t.Fatal(err)
		}
		if before.State() != StateCreatingProject || before.Status() != StatusInProgress {
			t.Errorf("Advance changed the receiver to %q/%q", before.State(), before.Status())
		}
	})

	t.Run("errorからは進めない", func(t *testing.T) {
		if _, err := progress(t, StateAddingBreakGlassGroups, StatusError).Advance(); err == nil {
			t.Error("Advance() from an error = nil error, want an error")
		}
	})

	t.Run("initial_setup_doneからは進めない", func(t *testing.T) {
		if _, err := progress(t, StateInitialSetupDone, StatusSuccess).Advance(); err == nil {
			t.Error("Advance() from the done state = nil error, want an error")
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
		p := progress(t, StateAddingBreakGlassGroups, StatusInProgress)
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
		if next.State() != StateAddingServiceAccountPermissions || next.Status() != StatusInProgress {
			t.Errorf("got %q/%q, want %q/%q", next.State(), next.Status(), StateAddingServiceAccountPermissions, StatusInProgress)
		}
	})
}
