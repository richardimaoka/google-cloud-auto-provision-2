// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"

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
	ID domain.ProjectCreationID
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
}

func NewProjectCreationService(store progressStore, creator projectCreator, groups groupLookup, organization domain.Organization) *ProjectCreationService {
	return &ProjectCreationService{store: store, creator: creator, groups: groups, organization: organization}
}

// recordProjectCreationRequest records progress. If progress.ID is already
// recorded, it records nothing and returns alreadyRecorded=true.
func (s *ProjectCreationService) recordProjectCreationRequest(ctx context.Context, progress domain.ProjectCreationProgress) (alreadyRecorded bool, err error) {
	// step:
	//   s.store.Createで記録。エラーなら返却。
	//   ただし記録済みのrequestと中身が違う場合はエラーを返却(storeから記録済みの内容を得る方法は未定)。

	return false, nil
}

// InitiateProjectCreationAsUser accepts a project creation request under the
// caller's own Google account and returns once it is recorded. It creates
// the project and grants the automation service account access to it under
// token — the caller's own OAuth-delegated credentials — before handing the
// rest of the initial setup to a Cloud Run job (see package worker), whose
// progress is read with GetProjectCreationProgress. The returned error means
// the request was not accepted.
//
// Use this while the automation service account has no organization-level
// project creator permission. Once it's granted (or deliberately withheld
// for good), delete whichever of this function and
// InitiateProjectCreationAsServiceAccount lost out, and fold
// recordProjectCreationRequest into the survivor.
func (s *ProjectCreationService) InitiateProjectCreationAsUser(ctx context.Context, request InitiateProjectCreationRequest, token oauth2.TokenSource) (InitiateProjectCreationResponse, error) {
	// step:
	//   request.DisplayName・Requester・DeveloperGroupからdomain.ProjectCreationRequestを構成し、
	//   バリデーション(domain.NewEmail, domain.NewGoogleGroup, domain.NewProjectCreationRequest)。
	//   判定に必要な情報(DeveloperGroupのメンバー、break-glassグループのメンバー等)はs.groupsで取得。
	//   判定はdomainの判定関数(I/Oなし、取得した情報とs.organizationを引数で受け取る)で行う。workerも同じ関数で再判定する。
	//   問題があれば、記録せず、問題と推奨解決法とともにエラーを返却。

	// step:
	//   State = "creating_project", status = "in_progress"のdomain.ProjectCreationProgressを構成し、
	//   recordProjectCreationRequestを呼ぶ。エラーならそのまま返却。
	//   alreadyRecordedならプロジェクト作成もworker起動もせず、request.IDで成功を返却。

	// step:
	//   tokenでs.creator.Createを呼ぶ。エラーならworkerを起動せず返却。
	//   成功したら、作成したプロジェクトのIDを渡してprogress.AdvancePastCreationを呼び、
	//   その結果をs.store.Updateで記録(State = "adding_iam_bindings", status = "in_progress"になる)。

	// step:
	//   request.IDを渡して、Cloud Run Jobsのworker.RunProjectCreationを起動。

	return InitiateProjectCreationResponse{ID: request.ID}, nil
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

	return InitiateProjectCreationResponse{ID: request.ID}, nil
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
