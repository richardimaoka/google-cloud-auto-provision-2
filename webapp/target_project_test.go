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

// fakeGoogleGroups returns groups[email], or a group with no members if absent.
type fakeGoogleGroups struct {
	groups map[string]domain.GoogleGroup
}

func (f *fakeGoogleGroups) LookUp(ctx context.Context, email domain.Email) (domain.GoogleGroup, error) {
	if g, ok := f.groups[email.String()]; ok {
		return g, nil
	}
	return domain.NewGoogleGroup(email, nil), nil
}

// newFakeGoogleGroups returns a fakeGoogleGroups where
// developer-group@example.com has two members in example.com.
func newFakeGoogleGroups(t *testing.T) *fakeGoogleGroups {
	t.Helper()
	email := func(s string) domain.Email {
		e, err := domain.NewEmail(s)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	group := domain.NewGoogleGroup(email("developer-group@example.com"), []domain.GoogleGroupMember{
		domain.NewGoogleGroupMember(email("alice@example.com"), "MEMBER"),
		domain.NewGoogleGroupMember(email("bob@example.com"), "MEMBER"),
	})
	return &fakeGoogleGroups{groups: map[string]domain.GoogleGroup{"developer-group@example.com": group}}
}

// fakeJobStarter records the IDs it was asked to start.
type fakeJobStarter struct {
	started []domain.ProjectCreationID
}

func (f *fakeJobStarter) Start(ctx context.Context, id domain.ProjectCreationID) error {
	f.started = append(f.started, id)
	return nil
}

// testOrganization returns an organization whose domain is example.com.
func testOrganization(t *testing.T) domain.Organization {
	t.Helper()
	o, err := domain.NewOrganization("123456789012", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// testBreakGlassRequirement returns break-glass group addresses that
// fakeGoogleGroups serves with no members, so no developer is in them.
func testBreakGlassRequirement(t *testing.T) domain.BreakGlassRequirement {
	t.Helper()
	t.Setenv(domain.BreakGlassUltimateGroupEmailEnv, "break-glass-ultimate@example.com")
	t.Setenv(domain.BreakGlassEmergencyGroupEmailEnv, "break-glass-emergency@example.com")
	r, err := domain.NewBreakGlassRequirement()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// testProgress returns a valid ProjectCreationProgress at StateCreatingProject.
func testProgress(t *testing.T, id domain.ProjectCreationID) domain.ProjectCreationProgress {
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
		// All dependencies are fakes that succeed; the creator "creates" example-project.
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := NewProjectCreationService(store, creator, newFakeGoogleGroups(t), jobs, testOrganization(t), testBreakGlassRequirement(t))

		// A valid request whose developer group has enough members in example.com.
		request := InitiateProjectCreationRequest{
			ID:             "request-ok",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: the request is accepted (nil error, same ID), the project is
		// created once, and the progress is recorded and then advanced.
		// The token is nil because the fake creator ignores it.
		got, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err != nil {
			t.Fatalf("InitiateProjectCreationAsUser() = %v, want nil error", err)
		}
		if got.ID != domain.ProjectCreationID(request.ID) {
			t.Errorf("InitiateProjectCreationAsUser().ID = %q, want %q", got.ID, request.ID)
		}
		// The project is created (and access granted) exactly once.
		if creator.calls != 1 {
			t.Errorf("creator.Create was called %d times, want 1", creator.calls)
		}
		// Recorded once at creating_project, then updated once after AdvancePastCreation.
		if store.creates != 1 || store.updates != 1 {
			t.Errorf("store.Create/Update were called %d/%d times, want 1/1", store.creates, store.updates)
		}

		// Call counts alone don't show what was saved: check that the last write is
		// AdvancePastCreation's result, with the creator's project ID and the submitted request.
		progress, ok := store.recorded[domain.ProjectCreationID(request.ID)]
		if !ok {
			t.Fatalf("no progress recorded for %q", request.ID)
		}
		if progress.State() != domain.StateAddingIAMBindings || progress.Status() != domain.StatusInProgress {
			t.Errorf("recorded state/status = %q/%q, want %q/%q", progress.State(), progress.Status(), domain.StateAddingIAMBindings, domain.StatusInProgress)
		}
		if progress.ProjectID() != "example-project" {
			t.Errorf("recorded ProjectID = %q, want %q", progress.ProjectID(), "example-project")
		}
		if progress.Request().DisplayName != request.DisplayName {
			t.Errorf("recorded DisplayName = %q, want %q", progress.Request().DisplayName, request.DisplayName)
		}
		// The job continues this request, so it must be started with its ID.
		if len(jobs.started) != 1 || jobs.started[0] != domain.ProjectCreationID(request.ID) {
			t.Errorf("jobs started = %q, want [%q]", jobs.started, request.ID)
		}
	})

	t.Run("入力が不正ならプロジェクトを作成せずエラーを返す", func(t *testing.T) {
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := NewProjectCreationService(newFakeProgressStore(), creator, newFakeGoogleGroups(t), jobs, testOrganization(t), testBreakGlassRequirement(t))

		request := InitiateProjectCreationRequest{
			ID:             "request-invalid-email",
			DisplayName:    "Example Project",
			Requester:      "not-an-email", // invalid, so InitiateProjectCreationAsUser returns an error
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: an error and no project, because validation must stop a bad
		// request before anything reaches Google Cloud.
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(invalid requester email) = nil error, want an error")
		}
		if creator.calls != 0 {
			t.Errorf("creator.Create was called %d times, want 0", creator.calls)
		}
		if len(jobs.started) != 0 {
			t.Errorf("jobs started = %q, want none", jobs.started)
		}
	})

	t.Run("記録済みのIDで再試行すると二重にプロジェクトを作成しない", func(t *testing.T) {
		store := newFakeProgressStore()
		store.recorded["request-retried"] = testProgress(t, "request-retried")
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := NewProjectCreationService(store, creator, newFakeGoogleGroups(t), jobs, testOrganization(t), testBreakGlassRequirement(t))

		request := InitiateProjectCreationRequest{
			ID:             "request-retried",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: no error and the same ID, because a retry isn't a failure, and
		// nothing runs after store.Create reports the ID as recorded.
		got, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err != nil {
			t.Fatalf("InitiateProjectCreationAsUser(already recorded ID) = %v, want nil error", err)
		}
		if got.ID != domain.ProjectCreationID(request.ID) {
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
		if len(jobs.started) != 0 {
			t.Errorf("jobs started = %q, want none", jobs.started)
		}
	})

	t.Run("プロジェクト作成が失敗したらerrorとして記録し、エラーを返す", func(t *testing.T) {
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{err: errors.New("permission denied")}
		jobs := &fakeJobStarter{}
		svc := NewProjectCreationService(store, creator, newFakeGoogleGroups(t), jobs, testOrganization(t), testBreakGlassRequirement(t))

		request := InitiateProjectCreationRequest{
			ID:             "request-create-fails",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: an error, so the caller knows the request wasn't accepted.
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(project creation fails) = nil error, want an error")
		}
		// Confirms the error came from the creator, not from failing some
		// other way.
		if creator.calls != 1 {
			t.Errorf("creator.Create was called %d times, want 1", creator.calls)
		}
		if len(jobs.started) != 0 {
			t.Errorf("jobs started = %q, want none", jobs.started)
		}
		// The failure is recorded, so the request doesn't stay in_progress forever.
		progress, ok := store.recorded[domain.ProjectCreationID(request.ID)]
		if !ok {
			t.Fatalf("no progress recorded for %q", request.ID)
		}
		if progress.State() != domain.StateCreatingProject || progress.Status() != domain.StatusError {
			t.Errorf("recorded state/status = %q/%q, want %q/%q", progress.State(), progress.Status(), domain.StateCreatingProject, domain.StatusError)
		}
	})
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
