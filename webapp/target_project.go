// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// InitiateProjectCreationRequest is the input for initiating target project
// creation. DisplayName, Requester and DeveloperGroup are as submitted on the
// form (see webapp/server.go): raw, unvalidated strings, not yet the domain
// types they become inside recordProjectCreationRequest. DeveloperGroup's
// members aren't included here; they're looked up from Google Cloud.
type InitiateProjectCreationRequest struct {
	// ID is issued by the caller and doubles as an idempotency key: retrying
	// with the same ID never starts a second project creation.
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
	// Create returns alreadyRecorded=true, recording nothing, if progress.ID exists.
	Create(ctx context.Context, progress domain.ProjectCreationProgress) (alreadyRecorded bool, err error)
	Update(ctx context.Context, progress domain.ProjectCreationProgress) error
}

// projectCreator creates a project and grants the automation service account access to it.
type projectCreator interface {
	// Create acts as the user who owns token and returns the new project's ID.
	Create(ctx context.Context, token oauth2.TokenSource, request domain.ProjectCreationRequest) (projectID string, err error)
}

// groupLookup gets Google Groups with their members.
type groupLookup interface {
	Get(ctx context.Context, email domain.Email) (domain.GoogleGroup, error)
}

// ProjectCreationService starts and follows project provisioning.
type ProjectCreationService struct {
	store   progressStore
	creator projectCreator
	groups  groupLookup

	organization domain.Organization
	breakGlass   domain.BreakGlassRequirement
}

func NewProjectCreationService(store progressStore, creator projectCreator, groups groupLookup, organization domain.Organization, breakGlass domain.BreakGlassRequirement) *ProjectCreationService {
	return &ProjectCreationService{store: store, creator: creator, groups: groups, organization: organization, breakGlass: breakGlass}
}

// recordProjectCreationRequest records progress. If progress.ID is already
// recorded, it records nothing and returns alreadyRecorded=true.
func (s *ProjectCreationService) recordProjectCreationRequest(ctx context.Context, progress domain.ProjectCreationProgress) (alreadyRecorded bool, err error) {
	alreadyRecorded, err = s.store.Create(ctx, progress)
	if err != nil {
		return false, fmt.Errorf("record project creation request: %w", err)
	}
	// step:
	//   記録済みのrequestと中身が違う場合はエラーを返却(storeから記録済みの内容を得る方法は未定)。

	return alreadyRecorded, nil
}

// InitiateProjectCreationAsUser records the request, creates the project with
// the caller's token, and hands the rest of the setup to a Cloud Run job.
// Use it until the automation service account can create projects itself.
func (s *ProjectCreationService) InitiateProjectCreationAsUser(ctx context.Context, request InitiateProjectCreationRequest, token oauth2.TokenSource) (InitiateProjectCreationResponse, error) {
	// フォームの文字列からdomain.ProjectCreationRequestを構成する。
	// DeveloperGroupのメンバーはs.groupsで取得。不正な値ならエラーを返す。
	requester, err := domain.NewEmail(request.Requester)
	if err != nil {
		return InitiateProjectCreationResponse{}, fmt.Errorf("requester: %w", err)
	}
	developerGroupEmail, err := domain.NewEmail(request.DeveloperGroup)
	if err != nil {
		return InitiateProjectCreationResponse{}, fmt.Errorf("developer group: %w", err)
	}
	developerGroup, err := s.groups.Get(ctx, developerGroupEmail)
	if err != nil {
		return InitiateProjectCreationResponse{}, fmt.Errorf("get developer group: %w", err)
	}
	projectRequest, err := domain.NewProjectCreationRequest(request.DisplayName, requester, developerGroup)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}

	// break-glassグループの現在のメンバーを取得し、domainの判定関数で問題を探す。
	// 問題があれば記録せずに返す(workerも同じ関数で再判定する)。
	breakGlass, err := s.getBreakGlass(ctx)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	if problems := domain.FindProjectCreationProblems(projectRequest, s.organization, breakGlass); len(problems) > 0 {
		return InitiateProjectCreationResponse{}, &ProjectCreationProblemsError{Problems: problems}
	}

	// creating_projectとして記録する。
	// 記録済みのIDなら重複申請なので、プロジェクトを作らずに成功を返す。
	id := domain.ProjectCreationID(request.ID)
	progress, err := domain.NewProjectCreationProgress(id, 1, projectRequest, domain.StateCreatingProject, domain.StatusInProgress, "")
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	alreadyRecorded, err := s.recordProjectCreationRequest(ctx, progress)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	if alreadyRecorded {
		return InitiateProjectCreationResponse{ID: id}, nil
	}

	// 利用者のtokenでプロジェクトを作成し、自動化Service Accountに権限を付与する。
	// 成功したらadding_iam_bindingsに進め、失敗したらerrorとして記録する。
	projectID, err := s.creator.Create(ctx, token, projectRequest)
	if err != nil {
		err = fmt.Errorf("create project: %w", err)
		if failed, failErr := progress.Fail(); failErr != nil {
			err = errors.Join(err, failErr)
		} else if updateErr := s.store.Update(ctx, failed); updateErr != nil {
			err = errors.Join(err, fmt.Errorf("record project creation failure: %w", updateErr))
		}
		return InitiateProjectCreationResponse{}, err
	}
	progress, err = progress.AdvancePastCreation(projectID)
	if err != nil {
		return InitiateProjectCreationResponse{}, err
	}
	if err := s.store.Update(ctx, progress); err != nil {
		return InitiateProjectCreationResponse{}, fmt.Errorf("record project creation: %w", err)
	}

	// step:
	//   request.IDを渡して、Cloud Run Jobsのworker.RunProjectCreationを起動。

	return InitiateProjectCreationResponse{ID: id}, nil
}

// getBreakGlass gets the break-glass groups at s.breakGlass's addresses with
// their current members.
func (s *ProjectCreationService) getBreakGlass(ctx context.Context) (domain.BreakGlass, error) {
	ultimate, err := s.groups.Get(ctx, s.breakGlass.UltimateGroupEmail())
	if err != nil {
		return domain.BreakGlass{}, fmt.Errorf("get ultimate break-glass group: %w", err)
	}
	emergency, err := s.groups.Get(ctx, s.breakGlass.EmergencyGroupEmail())
	if err != nil {
		return domain.BreakGlass{}, fmt.Errorf("get emergency break-glass group: %w", err)
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
	//   recordProjectCreationRequestを呼ぶ。エラーならそのまま返却。

	// step:
	//   request.IDを渡して、Cloud Run Jobsのworker.RunProjectCreationを起動。
	//   (workerが自身でプロジェクトを作成するため、ここでの追加処理はない。)

	return InitiateProjectCreationResponse{ID: domain.ProjectCreationID(request.ID)}, nil
}

// createProjectAndGrantAutomationAccess creates the Google Cloud project for
// request and grants the automation service account access to it, both under
// callerCredentials — the caller's own OAuth-delegated credentials. It is
// only called from InitiateProjectCreationAsUser: worker.RunProjectCreation
// creates the project itself, under the automation service account's own
// credentials, when initiated via InitiateProjectCreationAsServiceAccount.
//
// The access granted here only needs to cover what worker.RunProjectCreation
// does next (adding IAM bindings, setting up the PAM entitlement) — not
// lasting privileges, since Phase 3 onward hands ongoing operation to the
// service account's Workload Identity (see spec.md §2.3).
//
// This builds the googlecloud clients itself instead of taking one as a
// parameter, so tests can only reach it via callerCredentials, faking the
// HTTP transport rather than mocking in Go — undecided if that's worth
// changing; revisit later.
func (s *ProjectCreationService) createProjectAndGrantAutomationAccess(ctx context.Context, request domain.ProjectCreationRequest, callerCredentials ...option.ClientOption) (*googlecloud.Project, error) {
	// step:
	//   callerCredentialsでgooglecloud.NewProjectsClientを構成し、
	//   request.DisplayNameでプロジェクトを作成(ProjectsClient.Create)。
	//   失敗時は、問題と推奨解決法とともにエラーを返却(BigQueryにはこの関数からは記録しない。
	//   呼び出し元のInitiateProjectCreationAsUserがまとめて記録する)。

	// step:
	//   同じcallerCredentialsでgooglecloud.NewIAMClientを構成し、作成したプロジェクトに対して
	//   AUTOMATION_SA_EMAILへ必要なロールのIAM bindingを付与(IAMClient.AddBinding)。
	//   失敗時は、問題と推奨解決法とともにエラーを返却。

	return nil, nil
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
