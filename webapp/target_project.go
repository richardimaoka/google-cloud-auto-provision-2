// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	"golang.org/x/oauth2"
)

// InitiateProjectCreationRequest is the input for initiating target project
// creation. DisplayName, Requester and DeveloperGroup are as submitted on the
// form (see webapp/server.go): raw, unvalidated strings, not yet the domain
// types they become inside InitiateProjectCreationAsUser. DeveloperGroup's
// members aren't included here; they're looked up from Google Cloud.
type InitiateProjectCreationRequest struct {
	// ID is issued by the caller. A request with an already recorded ID is
	// rejected with an error.
	ID string
	// DisplayName is the new project's requested display name.
	DisplayName string
	// Requester is the requester's email address.
	Requester string
	// DeveloperGroup is the developer group's email address.
	DeveloperGroup string
}

// InitiateProjectCreationResponse is the result of initiating target project
// creation.
type InitiateProjectCreationResponse struct {
	// ID identifies the accepted project creation. It is the ID of the request.
	ID domain.ProjectCreationID
}

// progressStore records ProjectCreationProgress.
type progressStore interface {
	// Create returns an error, recording nothing, if progress.ID exists.
	Create(ctx context.Context, progress domain.ProjectCreationProgress) (domain.ProjectCreationID, error)
	// Update replaces current with next. It returns an error, recording nothing,
	// if the recorded progress is no longer at current's state and status.
	Update(ctx context.Context, current, next domain.ProjectCreationProgress) error
}

// projectCreator creates a project and grants the automation service account access to it.
type projectCreator interface {
	// Create acts as the user who owns token and returns the new project's ID.
	Create(ctx context.Context, token oauth2.TokenSource, request domain.ProjectCreationRequest) (projectID string, err error)
}

// googleGroups looks up Google Groups with their members, acting as the user who owns token.
type googleGroups interface {
	LookUp(ctx context.Context, token oauth2.TokenSource, email domain.Email) (domain.GoogleGroup, error)
}

// jobStarter starts the Cloud Run job that continues a project creation.
type jobStarter interface {
	Start(ctx context.Context, id domain.ProjectCreationID) error
}

// iamClient changes a project's IAM bindings, acting as the user who owns token.
type iamClient interface {
	AddBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Principal, role domain.IAMRole) error
	// RemoveBinding succeeds if principal doesn't have role, like AddBinding if it does.
	RemoveBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Principal, role domain.IAMRole) error
}

// Environment variables that configFromEnvironment reads.
const (
	organizationIDEnv     = "ORGANIZATION_ID"
	organizationDomainEnv = "ORGANIZATION_DOMAIN"
	serviceAccountEnv     = "AUTOMATION_SA_EMAIL"
	googleCloudProjectEnv = "GOOGLE_CLOUD_PROJECT_ID"
)

// bigQueryDataset holds the project_creation_progress table. It matches
// google_bigquery_dataset.app in terraform/modules/progress_table.
const bigQueryDataset = "auto_provision"

// config is what ProjectCreationService needs besides its dependencies.
type config struct {
	Organization   domain.Organization
	BreakGlass     domain.BreakGlassRequirement
	ServiceAccount domain.Email
	// BigQueryProject holds the bigQueryDataset dataset.
	BigQueryProject string
}

// ProjectCreationService starts and follows project provisioning.
type ProjectCreationService struct {
	progressStore  progressStore
	projectCreator projectCreator
	googleGroups   googleGroups
	jobStarter     jobStarter
	iamClient      iamClient
	organization   domain.Organization
	breakGlass     domain.BreakGlassRequirement
	serviceAccount domain.Email
}

// NewProjectCreationService returns a ProjectCreationService backed by Google
// Cloud, configured from the environment.
func NewProjectCreationService(ctx context.Context) (*ProjectCreationService, error) {
	cfg, err := configFromEnvironment()
	if err != nil {
		return nil, err
	}
	// The progress table is the app's own data, so it's accessed with the app's
	// credentials, unlike the other dependencies, which act as the user.
	bigQuery, err := googlecloud.NewBigQueryClient(ctx)
	if err != nil {
		return nil, err
	}
	store := &bigQueryProgressStore{client: bigQuery, projectID: cfg.BigQueryProject, datasetID: bigQueryDataset}
	// TODO: build jobStarter once it exists.
	return newProjectCreationService(cfg, store, &resourceManagerProjects{}, &cloudIdentityGroups{}, nil, &resourceManagerIAM{}), nil
}

// configFromEnvironment reads config from the environment.
func configFromEnvironment() (config, error) {
	organization, err := domain.NewOrganization(os.Getenv(organizationIDEnv), os.Getenv(organizationDomainEnv))
	if err != nil {
		return config{}, fmt.Errorf("%s/%s: %w", organizationIDEnv, organizationDomainEnv, err)
	}
	breakGlass, err := domain.NewBreakGlassRequirement()
	if err != nil {
		return config{}, err
	}
	serviceAccount, err := domain.NewEmail(os.Getenv(serviceAccountEnv))
	if err != nil {
		return config{}, fmt.Errorf("%s: %w", serviceAccountEnv, err)
	}
	bigQueryProject, err := requiredEnv(googleCloudProjectEnv)
	if err != nil {
		return config{}, err
	}
	return config{
		Organization:    organization,
		BreakGlass:      breakGlass,
		ServiceAccount:  serviceAccount,
		BigQueryProject: bigQueryProject,
	}, nil
}

// requiredEnv returns the environment variable name, or an error if it's unset or empty.
func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return value, nil
}

// newProjectCreationService takes the dependencies directly, so tests can pass fakes.
func newProjectCreationService(cfg config, store progressStore, creator projectCreator, groups googleGroups, jobs jobStarter, iam iamClient) *ProjectCreationService {
	return &ProjectCreationService{
		progressStore:  store,
		projectCreator: creator,
		googleGroups:   groups,
		jobStarter:     jobs,
		iamClient:      iam,
		organization:   cfg.Organization,
		breakGlass:     cfg.BreakGlass,
		serviceAccount: cfg.ServiceAccount,
	}
}

// InitiateProjectCreationAsUser records the request, creates the project with
// the caller's token, and hands the rest of the setup to a Cloud Run job.
// Use it until the automation service account can create projects itself.
func (s *ProjectCreationService) InitiateProjectCreationAsUser(ctx context.Context, request InitiateProjectCreationRequest, token oauth2.TokenSource) (InitiateProjectCreationResponse, error) {
	projectRequest, err := s.checkPrerequisites(ctx, request, token)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	id := domain.ProjectCreationID(request.ID)
	progress, projectID, err := s.createProject(ctx, id, projectRequest, token)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	progress, err = s.addServiceAccountIAMBindings(ctx, progress, projectID, token)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	if err := s.startSetupJob(ctx, progress); err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	return InitiateProjectCreationResponse{ID: id}, nil
}

// checkPrerequisites builds a domain.ProjectCreationRequest from request and
// checks it for problems. Nothing is recorded.
func (s *ProjectCreationService) checkPrerequisites(ctx context.Context, request InitiateProjectCreationRequest, token oauth2.TokenSource) (domain.ProjectCreationRequest, error) {
	// フォームの文字列からdomain.ProjectCreationRequestを構成する。
	// DeveloperGroupのメンバーはs.googleGroupsで取得。不正な値ならエラーを返す。
	requester, err := domain.NewEmail(request.Requester)
	if err != nil {
		return domain.ProjectCreationRequest{}, fmt.Errorf("requester: %w", err)
	}
	developerGroupEmail, err := domain.NewEmail(request.DeveloperGroup)
	if err != nil {
		return domain.ProjectCreationRequest{}, fmt.Errorf("developer group: %w", err)
	}
	developerGroup, err := s.googleGroups.LookUp(ctx, token, developerGroupEmail)
	if err != nil {
		return domain.ProjectCreationRequest{}, fmt.Errorf("look up developer group: %w", err)
	}
	projectRequest, err := domain.NewProjectCreationRequest(request.DisplayName, requester, developerGroup)
	if err != nil {
		return domain.ProjectCreationRequest{}, err
	}

	// break-glassグループの現在のメンバーを取得し、domainの判定関数で問題を探す。
	// workerも同じ関数で再判定する。
	breakGlass, err := s.getBreakGlass(ctx, token)
	if err != nil {
		return domain.ProjectCreationRequest{}, err
	}
	if problems := domain.FindProjectCreationProblems(projectRequest, s.organization, breakGlass); len(problems) > 0 {
		return domain.ProjectCreationRequest{}, &ProjectCreationProblemsError{Problems: problems}
	}
	return projectRequest, nil
}

// createProject records request as creating_project, then creates the
// project with token. It returns the progress, still at creating_project, and
// the new project's ID.
func (s *ProjectCreationService) createProject(ctx context.Context, id domain.ProjectCreationID, request domain.ProjectCreationRequest, token oauth2.TokenSource) (domain.ProjectCreationProgress, string, error) {
	// creating_projectとして記録する。
	// 記録済みのIDならエラーを返し、プロジェクトは作らない。
	progress, err := domain.NewProjectCreationProgress(id, 1, request, domain.StateCreatingProject, domain.StatusInProgress, "")
	if err != nil {
		return domain.ProjectCreationProgress{}, "", err
	}
	if _, err := s.progressStore.Create(ctx, progress); err != nil {
		return domain.ProjectCreationProgress{}, "", fmt.Errorf("record project creation request: %w", err)
	}

	// 利用者のtokenでプロジェクトを作成し、失敗したらerrorとして記録する。
	projectID, err := s.projectCreator.Create(ctx, token, request)
	if err != nil {
		return domain.ProjectCreationProgress{}, "", s.recordFailure(ctx, progress, fmt.Errorf("create project: %w", err))
	}
	return progress, projectID, nil
}

// addServiceAccountIAMBindings advances current past creation with
// projectID, recording adding_service_account_iam_bindings, then grants
// s.serviceAccount access to the project.
// The access only needs to cover what the worker does next, not lasting
// privileges (see spec.md §2.3).
func (s *ProjectCreationService) addServiceAccountIAMBindings(ctx context.Context, current domain.ProjectCreationProgress, projectID string, token oauth2.TokenSource) (domain.ProjectCreationProgress, error) {
	// adding_service_account_iam_bindingsとして記録する。
	next, err := current.AdvancePastCreation(projectID)
	if err != nil {
		// current.AdvancePastCreation()はエラーを返すことはないはずだが、万一発生したらprogressStoreは前段階のin_progressのままになる
		return domain.ProjectCreationProgress{}, err
	}
	if err := s.progressStore.Update(ctx, current, next); err != nil {
		return domain.ProjectCreationProgress{}, fmt.Errorf("record project creation: %w", err)
	}

	// 自動化Service Accountに権限を付与する、失敗したらerrorとして記録する。
	projectIamAdminRole, err := domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")
	if err != nil {
		// domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")はエラーを返すことはないはずだが、万一発生したらprogressStoreはin_progressのままになる
		return domain.ProjectCreationProgress{}, fmt.Errorf("invalid IAM role: %w", err)
	}
	if err := s.iamClient.AddBinding(ctx, token, next.ProjectID(), domain.NewServiceAccountPrincipal(s.serviceAccount), projectIamAdminRole); err != nil {
		return domain.ProjectCreationProgress{}, s.recordFailure(ctx, next, fmt.Errorf("adding IAM binding for service account: %w", err))
	}
	return next, nil
}

// startSetupJob advances current to adding_iam_bindings, records it, and
// starts the Cloud Run job that does the rest of the setup.
func (s *ProjectCreationService) startSetupJob(ctx context.Context, current domain.ProjectCreationProgress) error {
	// adding_iam_bindingsとして記録する。
	next, err := current.Advance()
	if err != nil {
		// current.Advance()はエラーを返すことはないはずだが、万一発生したらprogressStoreは前段階のin_progressのままになる
		return err
	}
	if err := s.progressStore.Update(ctx, current, next); err != nil {
		return fmt.Errorf("record project setup: %w", err)
	}

	// 残りの初期設定を行うCloud Run Jobs(worker.RunProjectCreation)を起動し、失敗したらerrorとして記録する。
	if err := s.jobStarter.Start(ctx, next.ID()); err != nil {
		return s.recordFailure(ctx, next, fmt.Errorf("start project setup job: %w", err))
	}
	return nil
}

// recordFailure records current as failed and returns err, joined with any
// error from recording it.
func (s *ProjectCreationService) recordFailure(ctx context.Context, current domain.ProjectCreationProgress, err error) error {
	next, failErr := current.Fail()
	if failErr != nil {
		// current.Fail()はエラーを返すことはないはずだが、万一発生したらprogressStoreはin_progressのままになる
		return errors.Join(err, failErr)
	}
	if updateErr := s.progressStore.Update(ctx, current, next); updateErr != nil {
		return errors.Join(err, fmt.Errorf("record project creation failure: %w", updateErr))
	}
	return err
}

// getBreakGlass looks up the break-glass groups at s.breakGlass's addresses with
// their current members.
func (s *ProjectCreationService) getBreakGlass(ctx context.Context, token oauth2.TokenSource) (domain.BreakGlass, error) {
	ultimate, err := s.googleGroups.LookUp(ctx, token, s.breakGlass.UltimateGroupEmail())
	if err != nil {
		return domain.BreakGlass{}, fmt.Errorf("look up ultimate break-glass group: %w", err)
	}
	emergency, err := s.googleGroups.LookUp(ctx, token, s.breakGlass.EmergencyGroupEmail())
	if err != nil {
		return domain.BreakGlass{}, fmt.Errorf("look up emergency break-glass group: %w", err)
	}
	return domain.NewBreakGlass(s.breakGlass, ultimate, emergency)
}

// ProjectCreationProblemsError is returned when a request has known problems.
type ProjectCreationProblemsError struct {
	Problems []domain.ProjectCreationProblem
}

func (e *ProjectCreationProblemsError) Error() string {
	descriptions := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		descriptions[i] = p.Description()
	}
	return "project creation request has problems: " + strings.Join(descriptions, "; ")
}

// InitiateProjectCreationAsServiceAccount accepts a project creation request
// and returns once it is recorded, without creating anything itself: the
// Cloud Run job (see package worker) creates the project using the
// automation service account's own credentials (ADC), and does the rest of
// the initial setup too. Progress is read with GetProjectCreationProgress.
// The returned error means the request was not accepted.
//
// Use this once the automation service account has organization-level
// project creator permission; see InitiateProjectCreationAsUser for now.
func (s *ProjectCreationService) InitiateProjectCreationAsServiceAccount(ctx context.Context, request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	// step:
	//   progressStore.Createで記録する。エラーならそのまま返却。

	// step:
	//   request.IDを渡して、Cloud Run Jobsのworker.RunProjectCreationを起動。
	//   (workerが自身でプロジェクトを作成するため、ここでの追加処理はない。)

	return InitiateProjectCreationResponse{ID: domain.ProjectCreationID(request.ID)}, nil
}

// GetProjectCreationProgressResponse is the result of looking up a project
// creation's progress.
type GetProjectCreationProgressResponse struct{}

// GetProjectCreationProgress returns the progress for a project creation request.
func (s *ProjectCreationService) GetProjectCreationProgress(id domain.ProjectCreationID) (GetProjectCreationProgressResponse, error) {
	return GetProjectCreationProgressResponse{}, nil
}

// RestartProjectCreationResponse is the result of restarting a project
// creation request.
type RestartProjectCreationResponse struct{}

// RestartProjectCreation resumes a project creation request after an error.
func (s *ProjectCreationService) RestartProjectCreation(id domain.ProjectCreationID) (RestartProjectCreationResponse, error) {
	return RestartProjectCreationResponse{}, nil
}
