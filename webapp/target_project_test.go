package webapp

import (
	"context"
	"errors"
	"fmt"
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

func (f *fakeProgressStore) Create(ctx context.Context, progress domain.ProjectCreationProgress) (domain.ProjectCreationID, error) {
	f.creates++
	if f.err != nil {
		return "", f.err
	}
	if _, ok := f.recorded[progress.ID()]; ok {
		return "", fmt.Errorf("project creation %q is already recorded", progress.ID())
	}
	f.recorded[progress.ID()] = progress
	return progress.ID(), nil
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

func (f *fakeGoogleGroups) LookUp(ctx context.Context, token oauth2.TokenSource, email domain.Email) (domain.GoogleGroup, error) {
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

// fakeJobStarter records the IDs it was asked to start. Set err to make Start fail.
type fakeJobStarter struct {
	err     error
	started []domain.ProjectCreationID
}

func (f *fakeJobStarter) Start(ctx context.Context, id domain.ProjectCreationID) error {
	if f.err != nil {
		return f.err
	}
	f.started = append(f.started, id)
	return nil
}

// iamBinding is one AddBinding or RemoveBinding call.
type iamBinding struct {
	projectID string
	principal domain.Email
	role      domain.IAMRole
}

// fakeIAMClient records the bindings it was asked to add and remove. Set
// err to make AddBinding fail.
type fakeIAMClient struct {
	err            error
	added, removed []iamBinding
}

func (f *fakeIAMClient) AddBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Email, role domain.IAMRole) error {
	if f.err != nil {
		return f.err
	}
	f.added = append(f.added, iamBinding{projectID, principal, role})
	return nil
}

func (f *fakeIAMClient) RemoveBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Email, role domain.IAMRole) error {
	f.removed = append(f.removed, iamBinding{projectID, principal, role})
	return nil
}

// testConfig returns a config in the example.com organization. Its
// break-glass groups are served by fakeGoogleGroups with no members, so no
// developer is in them.
func testConfig(t *testing.T) config {
	t.Helper()
	organization, err := domain.NewOrganization("123456789012", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(domain.BreakGlassUltimateGroupEmailEnv, "break-glass-ultimate@example.com")
	t.Setenv(domain.BreakGlassEmergencyGroupEmailEnv, "break-glass-emergency@example.com")
	breakGlass, err := domain.NewBreakGlassRequirement()
	if err != nil {
		t.Fatal(err)
	}
	serviceAccount, err := domain.NewEmail("automation@example-project.iam.gserviceaccount.com")
	if err != nil {
		t.Fatal(err)
	}
	return config{Organization: organization, BreakGlass: breakGlass, ServiceAccount: serviceAccount}
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
		iam := &fakeIAMClient{}
		cfg := testConfig(t)
		svc := newProjectCreationService(cfg, store, creator, newFakeGoogleGroups(t), jobs, iam)

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
		// Recorded once at creating_project, then updated at
		// adding_service_account_iam_bindings and adding_iam_bindings.
		if store.creates != 1 || store.updates != 2 {
			t.Errorf("store.Create/Update were called %d/%d times, want 1/2", store.creates, store.updates)
		}

		// Call counts alone don't show what was saved: check that the last write is
		// at adding_iam_bindings, with the creator's project ID and the submitted request.
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
		// The service account gets projectIamAdmin on the new project, once.
		role, err := domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")
		if err != nil {
			t.Fatal(err)
		}
		want := iamBinding{projectID: "example-project", principal: cfg.ServiceAccount, role: role}
		if len(iam.added) != 1 || iam.added[0] != want {
			t.Errorf("IAM bindings added = %v, want [%v]", iam.added, want)
		}
		// The job continues this request, so it must be started with its ID.
		if len(jobs.started) != 1 || jobs.started[0] != domain.ProjectCreationID(request.ID) {
			t.Errorf("jobs started = %q, want [%q]", jobs.started, request.ID)
		}
	})

	t.Run("入力が不正ならプロジェクトを作成せずエラーを返す", func(t *testing.T) {
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := newProjectCreationService(testConfig(t), newFakeProgressStore(), creator, newFakeGoogleGroups(t), jobs, &fakeIAMClient{})

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

	t.Run("記録済みのIDで再試行するとエラーを返し、プロジェクトを作成しない", func(t *testing.T) {
		store := newFakeProgressStore()
		store.recorded["request-retried"] = testProgress(t, "request-retried")
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := newProjectCreationService(testConfig(t), store, creator, newFakeGoogleGroups(t), jobs, &fakeIAMClient{})

		request := InitiateProjectCreationRequest{
			ID:             "request-retried",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: an error, and nothing runs after store.Create rejects the ID.
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if err == nil {
			t.Error("InitiateProjectCreationAsUser(already recorded ID) = nil error, want an error")
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
		svc := newProjectCreationService(testConfig(t), store, creator, newFakeGoogleGroups(t), jobs, &fakeIAMClient{})

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

	t.Run("既知の問題があれば記録もプロジェクト作成もせずエラーを返す", func(t *testing.T) {
		// The developer group has one member, fewer than the minimum.
		groups := newFakeGoogleGroups(t)
		groupEmail, err := domain.NewEmail("developer-group@example.com")
		if err != nil {
			t.Fatal(err)
		}
		alice, err := domain.NewEmail("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		groups.groups["developer-group@example.com"] = domain.NewGoogleGroup(groupEmail, []domain.GoogleGroupMember{
			domain.NewGoogleGroupMember(alice, "MEMBER"),
		})
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		svc := newProjectCreationService(testConfig(t), store, creator, groups, jobs, &fakeIAMClient{})

		request := InitiateProjectCreationRequest{
			ID:             "request-has-problem",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: a ProjectCreationProblemsError, so the caller can show the
		// problems, and nothing recorded, since the request was never accepted.
		_, err = svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		var problemsErr *ProjectCreationProblemsError
		if !errors.As(err, &problemsErr) {
			t.Fatalf("InitiateProjectCreationAsUser(too few members) = %v, want a *ProjectCreationProblemsError", err)
		}
		if store.creates != 0 {
			t.Errorf("store.Create was called %d times, want 0", store.creates)
		}
		if creator.calls != 0 {
			t.Errorf("creator.Create was called %d times, want 0", creator.calls)
		}
		if len(jobs.started) != 0 {
			t.Errorf("jobs started = %q, want none", jobs.started)
		}
	})

	t.Run("権限付与が失敗したらerrorとして記録し、エラーを返す", func(t *testing.T) {
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{}
		iam := &fakeIAMClient{err: errors.New("permission denied")}
		svc := newProjectCreationService(testConfig(t), store, creator, newFakeGoogleGroups(t), jobs, iam)

		request := InitiateProjectCreationRequest{
			ID:             "request-binding-fails",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: the IAM error comes back, the failure is recorded at the
		// binding step, and no job is started.
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if !errors.Is(err, iam.err) {
			t.Errorf("InitiateProjectCreationAsUser(binding fails) = %v, want it to wrap %v", err, iam.err)
		}
		progress, ok := store.recorded[domain.ProjectCreationID(request.ID)]
		if !ok {
			t.Fatalf("no progress recorded for %q", request.ID)
		}
		if progress.State() != domain.StateAddingServiceAccountIAMBindings || progress.Status() != domain.StatusError {
			t.Errorf("recorded state/status = %q/%q, want %q/%q", progress.State(), progress.Status(), domain.StateAddingServiceAccountIAMBindings, domain.StatusError)
		}
		if len(jobs.started) != 0 {
			t.Errorf("jobs started = %q, want none", jobs.started)
		}
	})

	t.Run("worker起動が失敗したらerrorとして記録し、エラーを返す", func(t *testing.T) {
		store := newFakeProgressStore()
		creator := &fakeProjectCreator{projectID: "example-project"}
		jobs := &fakeJobStarter{err: errors.New("quota exceeded")}
		svc := newProjectCreationService(testConfig(t), store, creator, newFakeGoogleGroups(t), jobs, &fakeIAMClient{})

		request := InitiateProjectCreationRequest{
			ID:             "request-job-fails",
			DisplayName:    "Example Project",
			Requester:      "requester@example.com",
			DeveloperGroup: "developer-group@example.com",
		}
		// Success: the job error comes back and the failure is recorded at
		// adding_iam_bindings, the state the job would have continued from.
		_, err := svc.InitiateProjectCreationAsUser(context.Background(), request, nil)
		if !errors.Is(err, jobs.err) {
			t.Errorf("InitiateProjectCreationAsUser(job start fails) = %v, want it to wrap %v", err, jobs.err)
		}
		progress, ok := store.recorded[domain.ProjectCreationID(request.ID)]
		if !ok {
			t.Fatalf("no progress recorded for %q", request.ID)
		}
		if progress.State() != domain.StateAddingIAMBindings || progress.Status() != domain.StatusError {
			t.Errorf("recorded state/status = %q/%q, want %q/%q", progress.State(), progress.Status(), domain.StateAddingIAMBindings, domain.StatusError)
		}
	})
}

func TestConfigFromEnvironment(t *testing.T) {
	setEnv := func(t *testing.T) {
		t.Setenv(organizationIDEnv, "123456789012")
		t.Setenv(organizationDomainEnv, "example.com")
		t.Setenv(domain.BreakGlassUltimateGroupEmailEnv, "break-glass-ultimate@example.com")
		t.Setenv(domain.BreakGlassEmergencyGroupEmailEnv, "break-glass-emergency@example.com")
		t.Setenv(serviceAccountEnv, "automation@example-project.iam.gserviceaccount.com")
	}

	t.Run("環境変数からconfigを読み込む", func(t *testing.T) {
		setEnv(t)
		cfg, err := configFromEnvironment()
		if err != nil {
			t.Fatalf("configFromEnvironment() = %v", err)
		}
		if cfg.Organization.ID() != "123456789012" || cfg.Organization.Domain() != "example.com" {
			t.Errorf("Organization = %q/%q, want %q/%q", cfg.Organization.ID(), cfg.Organization.Domain(), "123456789012", "example.com")
		}
		if got := cfg.BreakGlass.UltimateGroupEmail().String(); got != "break-glass-ultimate@example.com" {
			t.Errorf("BreakGlass.UltimateGroupEmail() = %q, want %q", got, "break-glass-ultimate@example.com")
		}
		if got := cfg.ServiceAccount.String(); got != "automation@example-project.iam.gserviceaccount.com" {
			t.Errorf("ServiceAccount = %q, want %q", got, "automation@example-project.iam.gserviceaccount.com")
		}
	})

	for _, name := range []string{organizationIDEnv, organizationDomainEnv, domain.BreakGlassUltimateGroupEmailEnv, serviceAccountEnv} {
		t.Run(name+"が空だとエラー", func(t *testing.T) {
			setEnv(t)
			t.Setenv(name, "")
			if _, err := configFromEnvironment(); err == nil {
				t.Errorf("configFromEnvironment() with empty %s = nil error, want an error", name)
			}
		})
	}
}
