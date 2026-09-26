// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
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
	Update(ctx context.Context, progress domain.ProjectCreationProgress) error
}

// projectCreator creates a project and grants the automation service account access to it.
type projectCreator interface {
	// Create acts as the user who owns token and returns the new project's ID.
	Create(ctx context.Context, token oauth2.TokenSource, request domain.ProjectCreationRequest) (projectID string, err error)
}

// googleGroups looks up Google Groups with their members.
type googleGroups interface {
	LookUp(ctx context.Context, email domain.Email) (domain.GoogleGroup, error)
}

// jobStarter starts the Cloud Run job that continues a project creation.
type jobStarter interface {
	Start(ctx context.Context, id domain.ProjectCreationID) error
}

// iamClient changes a project's IAM bindings, acting as the user who owns token.
type iamClient interface {
	AddBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Email, role domain.IAMRole) error
	RemoveBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Email, role domain.IAMRole) error
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

func NewProjectCreationService(store progressStore, creator projectCreator, groups googleGroups, jobs jobStarter, iam iamClient, organization domain.Organization, breakGlass domain.BreakGlassRequirement, serviceAccount domain.Email) *ProjectCreationService {
	return &ProjectCreationService{progressStore: store, projectCreator: creator, googleGroups: groups, jobStarter: jobs, iamClient: iam, organization: organization, breakGlass: breakGlass, serviceAccount: serviceAccount}
}

// InitiateProjectCreationAsUser records the request, creates the project with
// the caller's token, and hands the rest of the setup to a Cloud Run job.
// Use it until the automation service account can create projects itself.
func (s *ProjectCreationService) InitiateProjectCreationAsUser(ctx context.Context, request InitiateProjectCreationRequest, token oauth2.TokenSource) (InitiateProjectCreationResponse, error) {
	projectRequest, err := s.checkPrerequisites(ctx, request)
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
func (s *ProjectCreationService) checkPrerequisites(ctx context.Context, request InitiateProjectCreationRequest) (domain.ProjectCreationRequest, error) {
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
	developerGroup, err := s.googleGroups.LookUp(ctx, developerGroupEmail)
	if err != nil {
		return domain.ProjectCreationRequest{}, fmt.Errorf("look up developer group: %w", err)
	}
	projectRequest, err := domain.NewProjectCreationRequest(request.DisplayName, requester, developerGroup)
	if err != nil {
		return domain.ProjectCreationRequest{}, err
	}

	// break-glassグループの現在のメンバーを取得し、domainの判定関数で問題を探す。
	// workerも同じ関数で再判定する。
	breakGlass, err := s.getBreakGlass(ctx)
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

// addServiceAccountIAMBindings advances progress past creation with
// projectID, recording adding_service_account_iam_bindings, then grants
// s.serviceAccount access to the project.
// The access only needs to cover what the worker does next, not lasting
// privileges (see spec.md §2.3).
func (s *ProjectCreationService) addServiceAccountIAMBindings(ctx context.Context, progress domain.ProjectCreationProgress, projectID string, token oauth2.TokenSource) (domain.ProjectCreationProgress, error) {
	// adding_service_account_iam_bindingsとして記録する。
	progress, err := progress.AdvancePastCreation(projectID)
	if err != nil {
		// progress.AdvancePastCreation()はエラーを返すことはないはずだが、万一発生したらprogressStoreは前段階のin_progressのままになる
		return domain.ProjectCreationProgress{}, err
	}
	if err := s.progressStore.Update(ctx, progress); err != nil {
		return domain.ProjectCreationProgress{}, fmt.Errorf("record project creation: %w", err)
	}

	// 自動化Service Accountに権限を付与する、失敗したらerrorとして記録する。
	projectIamAdminRole, err := domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")
	if err != nil {
		// domain.NewIAMRole("roles/resourcemanager.projectIamAdmin")はエラーを返すことはないはずだが、万一発生したらprogressStoreはin_progressのままになる
		return domain.ProjectCreationProgress{}, fmt.Errorf("invalid IAM role: %w", err)
	}
	if err := s.iamClient.AddBinding(ctx, token, progress.ProjectID(), s.serviceAccount, projectIamAdminRole); err != nil {
		return domain.ProjectCreationProgress{}, s.recordFailure(ctx, progress, fmt.Errorf("adding IAM binding for service account: %w", err))
	}
	return progress, nil
}

// startSetupJob advances progress to adding_iam_bindings, records it, and
// starts the Cloud Run job that does the rest of the setup.
func (s *ProjectCreationService) startSetupJob(ctx context.Context, progress domain.ProjectCreationProgress) error {
	// adding_iam_bindingsとして記録する。
	progress, err := progress.Advance()
	if err != nil {
		// progress.Advance()はエラーを返すことはないはずだが、万一発生したらprogressStoreは前段階のin_progressのままになる
		return err
	}
	if err := s.progressStore.Update(ctx, progress); err != nil {
		return fmt.Errorf("record project setup: %w", err)
	}

	// 残りの初期設定を行うCloud Run Jobs(worker.RunProjectCreation)を起動し、失敗したらerrorとして記録する。
	if err := s.jobStarter.Start(ctx, progress.ID()); err != nil {
		return s.recordFailure(ctx, progress, fmt.Errorf("start project setup job: %w", err))
	}
	return nil
}

// recordFailure records progress as failed and returns err, joined with any
// error from recording it.
func (s *ProjectCreationService) recordFailure(ctx context.Context, progress domain.ProjectCreationProgress, err error) error {
	failed, failErr := progress.Fail()
	if failErr != nil {
		// progress.Fail()はエラーを返すことはないはずだが、万一発生したらprogressStoreはin_progressのままになる
		return errors.Join(err, failErr)
	}
	if updateErr := s.progressStore.Update(ctx, failed); updateErr != nil {
		return errors.Join(err, fmt.Errorf("record project creation failure: %w", updateErr))
	}
	return err
}

// getBreakGlass looks up the break-glass groups at s.breakGlass's addresses with
// their current members.
func (s *ProjectCreationService) getBreakGlass(ctx context.Context) (domain.BreakGlass, error) {
	ultimate, err := s.googleGroups.LookUp(ctx, s.breakGlass.UltimateGroupEmail())
	if err != nil {
		return domain.BreakGlass{}, fmt.Errorf("look up ultimate break-glass group: %w", err)
	}
	emergency, err := s.googleGroups.LookUp(ctx, s.breakGlass.EmergencyGroupEmail())
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
