// Package worker runs the initial setup of a target project as a Cloud Run job.
package worker

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
)

// RunProjectCreation runs the initial setup of an accepted project creation,
// from the state recorded for id up to StateInitialSetupDone.
//
// Below, "問題があれば" covers two different kinds of failure, decided 2026-09-22:
//   - A known domain.ProjectCreationProblem (e.g. domain.ProjectIDCollision): the
//     requester can act on it, so its Recommendation is recorded alongside the
//     error status and returned to them; no alert.
//   - Any other error from Google Cloud (e.g. PERMISSION_DENIED: this worker's
//     service account lacks a permission it needs): the requester can't fix it,
//     so only the raw error is recorded (no recommendation), and it is also
//     logged and alerted so an operator notices. ProjectCreationProgress still
//     moves to status = "error" either way, so GetProjectCreationProgress does
//     not show "in_progress" forever.
//
// Neither the BigQuery schema for recording a problem/recommendation nor the
// alerting is implemented yet.
//
// State = "creating_project" covers two different situations, decided
// 2026-09-23 (see webapp.InitiateProjectCreationAsUser and
// webapp.InitiateProjectCreationAsServiceAccount):
//   - Initiated via InitiateProjectCreationAsServiceAccount: this function
//     creates the project itself, with createProject, using its own
//     credentials (ADC).
//   - Initiated via InitiateProjectCreationAsUser: webapp already created the
//     project and called domain.ProjectCreationProgress.AdvancePastCreation
//     before handing off here, so this function skips createProject entirely
//     and reads the project ID the recorded ProjectCreationProgress already
//     carries (see ProjectCreationProgress.ProjectID).
func RunProjectCreation(ctx context.Context, id domain.ProjectCreationID) error {
	// step:
	//   記録済みのdomain.ProjectCreationRequestを、webappと同じdomainの判定関数で再判定。
	//   受理後にグループ構成などが変わっている可能性があるため。判定に必要な情報はGoogle Cloudから取得。
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "creating_project", status = "error"
	//     - 問題と推奨解決法

	// step:
	//   取得したProjectCreationProgressのStateがまだ"creating_project"ならcreateProjectを呼ぶ
	//   (InitiateProjectCreationAsServiceAccountで開始した場合。
	//   InitiateProjectCreationAsUserで開始した場合はここに来た時点で
	//   既にこのStateを通過済みのはずなので呼ばない)。
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

	return nil
}

// createProject creates the Google Cloud project for request, using this
// worker's own credentials (ADC, the automation service account). It is only
// called when initiated via webapp.InitiateProjectCreationAsServiceAccount —
// for webapp.InitiateProjectCreationAsUser, webapp already created the
// project under the caller's own OAuth-delegated credentials, so
// RunProjectCreation skips this call entirely rather than calling it with
// different credentials.
func createProject(ctx context.Context, request domain.ProjectCreationRequest) (*googlecloud.Project, error) {
	// step:
	//   ADC(このワーカーの自動化サービスアカウント)でgooglecloud.NewProjectsClientを構成し、
	//   request.DisplayNameでプロジェクトを作成(ProjectsClient.Create)。
	//   失敗時は、問題と推奨解決法とともにエラーを返却(BigQueryにはこの関数からは記録しない。
	//   呼び出し元のRunProjectCreationがまとめて記録する)。

	return nil, nil
}
