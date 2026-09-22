// Package webapp holds what the web application calls to start and follow
// project provisioning.
package webapp

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
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

// recordProjectCreationRequest validates request, then records it in
// BigQuery as ProjectCreationProgress at StateCreatingProject — or, if
// request.ID is already recorded, returns the existing ID unchanged instead
// of starting a second project creation. Both InitiateProjectCreationAsUser
// and InitiateProjectCreationAsServiceAccount call this first; it has no
// principal-specific behavior, since nothing here needs the requester's own
// credentials.
func recordProjectCreationRequest(ctx context.Context, request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	// step:
	//   request.DisplayName・Requester・DeveloperGroupからdomain.ProjectCreationRequestを構成し、
	//   バリデーション(domain.NewEmail, domain.NewGoogleGroup, domain.NewProjectCreationRequest)。
	//   判定に必要な情報(DeveloperGroupのメンバー、break-glassグループのメンバー等)はGoogle Cloudから取得。
	//   判定はdomainの判定関数(I/Oなし、取得した情報を引数で受け取る)で行う。workerも同じ関数で再判定する。
	//   問題があれば、BigQueryには記録せず、問題と推奨解決法とともにエラーを返却。
	//   (ProjectCreationProgressはdomain.ProjectCreationRequestを必須で持つため、構成前には記録できない)

	// step:
	//   以下をBigQueryに記録。
	//     - ProjectCreationProgress, State = "creating_project", status = "in_progress"
	//   request.IDが記録済みなら重複申請なので、記録も後続処理の起動もせず、記録済みのIDで成功を返却。
	//   ただし記録済みのrequestと中身が違う場合はエラーを返却。

	return InitiateProjectCreationResponse{ID: request.ID}, nil
}

// InitiateProjectCreationAsUser accepts a project creation request under the
// caller's own Google account and returns once it is recorded. It creates
// the project and grants the automation service account access to it under
// callerCredentials — the caller's own OAuth-delegated credentials, from a
// server-side OAuth session (not yet implemented, see
// webapp/oauth-client.md) — before handing the rest of the initial setup to
// a Cloud Run job (see package worker), whose progress is read with
// GetProjectCreationProgress. The returned error means the request was not
// accepted.
//
// Use this while the automation service account has no organization-level
// project creator permission. Once it's granted (or deliberately withheld
// for good), delete whichever of this function and
// InitiateProjectCreationAsServiceAccount lost out, and fold
// recordProjectCreationRequest into the survivor.
func InitiateProjectCreationAsUser(ctx context.Context, request InitiateProjectCreationRequest, callerCredentials ...option.ClientOption) (InitiateProjectCreationResponse, error) {
	// step:
	//   recordProjectCreationRequestを呼ぶ。エラーならそのまま返却。

	// step:
	//   callerCredentialsでcreateProjectAndGrantAutomationAccessを呼ぶ。
	//   成功したら、作成したプロジェクトのIDを渡してProjectCreationProgress.AdvancePastCreationを呼び、
	//   その結果をBigQueryに記録(State = "adding_iam_bindings", status = "in_progress"になる)。

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
func InitiateProjectCreationAsServiceAccount(ctx context.Context, request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
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
func createProjectAndGrantAutomationAccess(ctx context.Context, request domain.ProjectCreationRequest, callerCredentials ...option.ClientOption) (*googlecloud.Project, error) {
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
func GetProjectCreationProgress(id domain.ProjectCreationID) (GetProjectCreationProgressResponse, error) {
	return GetProjectCreationProgressResponse{}, nil
}

// RestartProjectCreationResponse is the result of restarting a project
// creation request.
type RestartProjectCreationResponse struct{}

// RestartProjectCreation resumes a project creation request after an error.
func RestartProjectCreation(id domain.ProjectCreationID) (RestartProjectCreationResponse, error) {
	return RestartProjectCreationResponse{}, nil
}
