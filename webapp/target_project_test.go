package webapp

import "testing"

// TestInitiateProjectCreationAsUser is a scaffold: InitiateProjectCreationAsUser
// is still a stub, so every case is skipped until it has a real body to test.
func TestInitiateProjectCreationAsUser(t *testing.T) {
	t.Run("正常系: 記録・プロジェクト作成・権限付与・AdvancePastCreation・worker起動まで行う", func(t *testing.T) {
		t.Skip("InitiateProjectCreationAsUser is not implemented yet")
	})
	t.Run("recordProjectCreationRequestがエラーならプロジェクトを作成せずエラーを返す", func(t *testing.T) {
		t.Skip("InitiateProjectCreationAsUser is not implemented yet")
	})
	t.Run("記録済みのIDで再試行すると二重にプロジェクトを作成しない", func(t *testing.T) {
		t.Skip("InitiateProjectCreationAsUser is not implemented yet")
	})
	t.Run("createProjectAndGrantAutomationAccessがエラーならworkerを起動せずエラーを返す", func(t *testing.T) {
		t.Skip("InitiateProjectCreationAsUser is not implemented yet")
	})
}

// TestCreateProjectAndGrantAutomationAccess is a scaffold:
// createProjectAndGrantAutomationAccess is still a stub, so every case is
// skipped until it has a real body to test.
func TestCreateProjectAndGrantAutomationAccess(t *testing.T) {
	t.Run("callerCredentialsでプロジェクトを作成し、自動化Service Accountへ権限を付与する", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("プロジェクト作成が失敗したら権限付与はせずエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
	t.Run("権限付与が失敗したらエラーを返す", func(t *testing.T) {
		t.Skip("createProjectAndGrantAutomationAccess is not implemented yet")
	})
}
