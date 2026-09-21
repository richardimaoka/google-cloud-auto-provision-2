// Package worker runs the initial setup of a target project as a Cloud Run job.
package worker

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// RunProjectCreation runs the initial setup of an accepted project creation,
// from the state recorded for id up to StateInitialSetupDone.
func RunProjectCreation(ctx context.Context, id domain.ProjectCreationID) error {
	// step:
	//   記録済みのdomain.ProjectCreationRequestを、webappと同じdomainの判定関数で再判定。
	//   受理後にグループ構成などが変わっている可能性があるため。判定に必要な情報はGoogle Cloudから取得。
	//   問題があれば以下をBigQueryに記録し、エラーを返却
	//     - ProjectCreationProgress, State = "creating_project", status = "error"
	//     - 問題と推奨解決法

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

	return nil
}
