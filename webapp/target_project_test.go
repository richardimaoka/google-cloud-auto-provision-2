package webapp

import (
	"context"
	"errors"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
)

// fakeProgressStore is an in-memory progressStore. Set err to
// make every call fail.
type fakeProgressStore struct {
	err      error
	recorded map[domain.ProjectCreationID]domain.ProjectCreationProgress
	creates  int
	updates  int
}

func newFakeProgressStore() *fakeProgressStore {
	return &fakeProgressStore{recorded: map[domain.ProjectCreationID]domain.ProjectCreationProgress{}}
}

func (f *fakeProgressStore) Create(ctx context.Context, progress domain.ProjectCreationProgress) (bool, error) {
	f.creates++
	if f.err != nil {
		return false, f.err
	}
	if _, ok := f.recorded[progress.ID()]; ok {
		return true, nil
	}
	f.recorded[progress.ID()] = progress
	return false, nil
}

func (f *fakeProgressStore) Update(ctx context.Context, progress domain.ProjectCreationProgress) error {
	f.updates++
	if f.err != nil {
		return f.err
	}
	f.recorded[progress.ID()] = progress
	return nil
}

// fakeProjectCreator is a projectCreator that returns projectID, or err if set.
type fakeProjectCreator struct {
	projectID string
	err       error
	calls     int
}

func (f *fakeProjectCreator) Create(ctx context.Context, token oauth2.TokenSource, request domain.ProjectCreationRequest) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.projectID, nil
}

// recordedProgress returns a ProjectCreationProgress to pre-populate a
// fakeProgressStore with, as if an earlier request had been recorded.
func recordedProgress(t *testing.T, id domain.ProjectCreationID) domain.ProjectCreationProgress {
	t.Helper()
	requester, err := domain.NewEmail("requester@example.com")
	if err != nil {
		t.Fatal(err)
	}
	groupEmail, err := domain.NewEmail("developer-group@example.com")
	if err != nil {
		t.Fatal(err)
	}
	request, err := domain.NewProjectCreationRequest("Example Project", requester, domain.NewGoogleGroup(groupEmail, nil))
	if err != nil {
		t.Fatal(err)
	}
	progress, err := domain.NewProjectCreationProgress(id, 1, request, domain.StateCreatingProject, domain.StatusInProgress, "")
	if err != nil {
		t.Fatal(err)
	}
	return progress
}

// TestInitiateProjectCreationAsUser checks recording, project creation, and
// access-granting happen in order, and a failure anywhere is returned.
func TestInitiateProjectCreationAsUser(t *testing.T) {
	t.Run("正常系: 記録・プロジェクト作成・権限付与・AdvancePastCreation・worker起動まで行う", func(t *testing.T) {
		t.Skip("blocked: no seam yet to fake the Cloud Run Jobs start")
	})

	t.Run("recordProjectCreationRequestがエラーならプロジェクトを作成せずエラーを返す", func(t *testing.T) {
		creator := &fakeProjectCreator{projectID: "example-project"}
		svc := &ProjectCreationService{store: newFakeProgressStore(), creator: creator}

		request := InitiateProjectCreationRequest{
			ID:             "request-invalid-email",
			DisplayName:    "Example Project",
			Requester:      "not-an-email",
			DeveloperGroup: "developer-group@example.com",
		}
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(invalid requester email) = nil error, want an error")
		}
		if creator.calls != 0 {
			t.Errorf("creator.Create was called %d times, want 0", creator.calls)
		}
	})

	t.Run("記録済みのIDで再試行すると二重にプロジェクトを作成しない", func(t *testing.T) {
		store := newFakeProgressStore()
		store.recorded["request-retried"] = recordedProgress(t, "request-retried")
		creator := &fakeProjectCreator{projectID: "example-project"}
		svc := &ProjectCreationService{store: store, creator: creator}

		request := InitiateProjectCreationRequest{
			ID:             "request-retried",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		got, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err != nil {
			t.Fatalf("InitiateProjectCreationAsUser(already recorded ID) = %v, want nil error", err)
		}
		if got.ID != request.ID {
			t.Errorf("InitiateProjectCreationAsUser(already recorded ID).ID = %q, want %q", got.ID, request.ID)
		}
		// Confirms the duplicate was detected through the store, not skipped
		// some other way.
		if store.creates != 1 {
			t.Errorf("store.Create was called %d times, want 1", store.creates)
		}
		if store.updates != 0 {
			t.Errorf("store.Update was called %d times, want 0", store.updates)
		}
		if creator.calls != 0 {
			t.Errorf("creator.Create was called %d times, want 0", creator.calls)
		}
	})

	t.Run("createProjectAndGrantAutomationAccessがエラーならworkerを起動せずエラーを返す", func(t *testing.T) {
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{err: errors.New("permission denied")}
		svc := &ProjectCreationService{store: store, creator: creator}

		request := InitiateProjectCreationRequest{
			ID:             "request-create-fails",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(project creation fails) = nil error, want an error")
		}
		// Confirms the error came from the creator, not from failing some
		// other way.
		if creator.calls != 1 {
			t.Errorf("creator.Create was called %d times, want 1", creator.calls)
		}
	})
}

// TestRecordProjectCreationRequest_StoreFails checks that a failing
// store.Create is returned as an error.
func TestRecordProjectCreationRequest_StoreFails(t *testing.T) {
	store := newFakeProgressStore()
	store.err = errors.New("create failed")
	svc := &ProjectCreationService{store: store}

	_, err := svc.recordProjectCreationRequest(context.Background(), InitiateProjectCreationRequest{
		ID:             "request-1",
		DisplayName:    "Example Project",
		Requester:      "requester@example.com",
		DeveloperGroup: "developer-group@example.com",
	})
	if err == nil {
		t.Error("recordProjectCreationRequest with a failing store.Create = nil error, want an error")
	}
	if store.creates != 1 {
		t.Errorf("store.Create was called %d times, want 1", store.creates)
	}
}

// TestCreateProjectAndGrantAutomationAccess is a scaffold:
// createProjectAndGrantAutomationAccess is still a stub, so every case is
// skipped until it has a real body to test.
func TestCreateProjectAndGrantAutomationAccess(t *testing.T) {
	t.Run("callerCredentialsでプロジェクトを作成し、自動化Service Accountへ権限を付与する", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("プロジェクト作成が失敗したら権限付与はせずエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("権限付与が失敗したらエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
}
