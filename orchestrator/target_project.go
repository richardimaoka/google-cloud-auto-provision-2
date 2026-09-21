// Package orchestrator coordinates Google Cloud project provisioning.
package orchestrator

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// InitiateProjectCreationRequest is the input for initiating target project
// creation.
type InitiateProjectCreationRequest struct {
	// ID is issued by the caller and doubles as an idempotency key: retrying
	// with the same ID never starts a second project creation.
	ID domain.ProjectCreationID
}

// InitiateProjectCreationResponse is the result of initiating target project
// creation.
type InitiateProjectCreationResponse struct {
	// ID identifies the accepted project creation. It is the ID of the request.
	ID domain.ProjectCreationID
}

// InitiateProjectCreation accepts a project creation request and returns once
// it is recorded. The rest of the initial setup keeps running in the
// background, and its progress is read with GetProjectCreationProgress. The
// returned error means the request was not accepted.
func InitiateProjectCreation(ctx context.Context, request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	// step:
	//   引数からdomain.ProjectCreationRequestを構成し、バリデーション。同期で実行。
	//   構成に必要な情報はGoogle Cloudから取得。
	//   問題があれば、BigQueryには記録せず、問題と推奨解決法とともにエラーを返却。
	//   (ProjectCreationProgressはdomain.ProjectCreationRequestを必須で持つため、構成前には記録できない)

	// step:
	//   以下をBigQueryに記録。同期で実行。
	//     - ProjectCreationProgress, State = "creating_project", status = "in_progress"
	//   request.IDが記録済みなら重複申請なので、記録も後続処理の起動もせず、記録済みのIDで成功を返却。
	//   ただし記録済みのrequestと中身が違う場合はエラーを返却。

	// early return:
	//   ここで、記録したIDを含むレスポンスを返却。以降はgoroutineでバックグラウンド実行。
	//   ctxは呼び出し元のレスポンス返却時にキャンセルされるため、以降はcontext.WithoutCancelで切り離したctxを使う。
	//   以降の問題は戻り値では返せないため、BigQueryに status = "error" として記録。
	//   呼び出し元はGetProjectCreationProgressで確認し、RestartProjectCreationで再開できる。

	// step:
	//   プロジェクト作成requestをGoogle Cloudに送信。
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "creating_project", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   以下をBigQueryに記録した後で、IAM binding設定requestをGoogle Cloudに送信。
	//     - ProjectCreationProgress, State = "adding_iam_bindings", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "adding_iam_bindings", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   以下をBigQueryに記録した後で、PAM entitlement設定requestをGoogle Cloudに送信。
	//     - ProjectCreationProgress, State = "adding_pam_entitlements", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "adding_pam_entitlements", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   以下をBigQueryに記録。
	//     - ProjectCreationProgress, State = "initial_setup_done", status = "success"

	return InitiateProjectCreationResponse{ID: request.ID}, nil
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
