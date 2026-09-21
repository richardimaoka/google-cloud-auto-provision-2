// Package orchestrator coordinates Google Cloud project provisioning.
package orchestrator

import "github.com/yusni/google-cloud-auto-provision-2/domain"

// InitiateProjectCreationRequest is the input for initiating target project
// creation.
type InitiateProjectCreationRequest struct{}

// InitiateProjectCreationResponse is the result of initiating target project
// creation.
type InitiateProjectCreationResponse struct{}

// InitiateProjectCreation starts the initial setup for a target project.
func InitiateProjectCreation(request InitiateProjectCreationRequest) (InitiateProjectCreationResponse, error) {
	// step:
	//   以下をBigQueryに記録した後で、引数からdomain.ProjectCreationRequestを構成し、バリデーション。
	//   構成に必要な情報はGoogle Cloudから取得。
	//     - ProjectCreationProgress, State = "initiating_project_creation", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "initiating_project_creation", status = "error"
	//     - 問題と推奨解決法
	//   注意:
	//     ProjectCreationProgressはdomain.ProjectCreationRequestを必須で持つため、
	//     requestの構成に失敗するとProjectCreationProgressを作成できない。
	//     その場合のBigQueryへの記録方法は要検討。

	// step:
	//   以下をBigQueryに記録した後で、プロジェクト作成requestをGoogle Cloudに送信。
	//     - ProjectCreationProgress, State = "creating_project", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "creating_project", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   以下をBigQueryに記録した後で、break glass設定requestをGoogle Cloudに送信。
	//     - ProjectCreationProgress, State = "adding_break_glass_groups", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "adding_break_glass_groups", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   以下をBigQueryに記録した後で、サービスアカウント権限設定requestをGoogle Cloudに送信。
	//     - ProjectCreationProgress, State = "adding_service_account_permissions", status = "in_progress"
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "adding_service_account_permissions", status = "error"
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

	return InitiateProjectCreationResponse{}, nil
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
