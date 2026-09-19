package gcp

import "testing"

// Unit tests use a fake httptest server, like groups_test.go, that serves:
//
//	POST   /v3/projects        create, returns an Operation
//	GET    /v3/operations/<id> poll an Operation
//	GET    /v3/projects/<id>   get
//	DELETE /v3/projects/<id>   delete, returns an Operation
//
// Create and Delete block until their operation is done, so the fake needs to
// be able to report "not done" for a few polls before "done".

func TestProjectsClient_Get(t *testing.T) {
	t.Skip("not implemented")

	// - 既存のプロジェクト: ID, Name, Parent, State が全て対応づけられる
	// - 親を持たないプロジェクト: Parent が空
	// - DELETE_REQUESTED 状態のプロジェクト: エラーにならず State がそのまま返る
	// - 存在しないプロジェクト: "get project" を含むエラー
	// - 権限不足: "get project" を含むエラー
}

func TestProjectsClient_Create(t *testing.T) {
	t.Skip("not implemented")

	// - リクエストボディの displayName が name と一致し、projectId が name から
	//   生成されたものになる
	// - クライアントの parent がリクエストの parent に入る
	// - クライアントの parent が空のときは parent がリクエストに含まれない
	// - オペレーションが最初のレスポンスで完了している: プロジェクトが返る
	// - オペレーションが数回のポーリング後に完了する: ポーリングが繰り返される
	// - オペレーションがエラーで完了する(クォータ超過など): メッセージを含むエラー
	// - 作成リクエスト自体が拒否される(403 など): "create project" を含むエラー
	// - ポーリング中に 500: ハングせずエラーを返す
	// - (未決定) ID が既に存在する(ALREADY_EXISTS): 新しい ID で数回リトライする
	// - (未決定) ポーリング中に context がキャンセルされる: 待機を止めてエラーを返す
}

func TestGenerateProjectID(t *testing.T) {
	t.Skip("not implemented")

	// どの入力でも次を満たす: 6〜30文字、小文字・数字・ハイフンのみ、
	// 英字で始まる、末尾がハイフンでない。
	//
	// - 通常の名前("My Project"): 小文字化され、空白がハイフンになる
	// - 記号やアンダースコアを含む名前: ハイフンに置き換えられ、連続しない
	// - 数字で始まる名前: 英字で始まるように接頭辞が付く
	// - 30文字を超える長い名前: サフィックスの分を残して切り詰められる
	// - 切り詰めた結果がハイフンで終わる: 末尾のハイフンが取り除かれる
	// - 日本語など英数字を含まない名前: 固定の接頭辞にフォールバックする
	// - 同じ名前で2回呼ぶ: ランダムなサフィックスにより結果が異なる
}

func TestProjectsClient_Delete(t *testing.T) {
	t.Skip("not implemented")

	// - オペレーションが最初のレスポンスで完了している: nil を返す
	// - オペレーションが数回のポーリング後に完了する: ポーリングが繰り返される
	// - 存在しないプロジェクト: "delete project" を含むエラー
	// - 権限不足: "delete project" を含むエラー
	// - オペレーションがエラーで完了する: エラーを返す
	// - ポーリング中に 500: ハングせずエラーを返す
}

// Integration tests (would go in projects_integration_test.go behind the
// "integration" build tag). Create and Delete change a real organization, and
// each run leaves a project in DELETE_REQUESTED, so these are still undecided.
//
// - (未決定) ライフサイクル: ランダムな名前で Create → 返された ID で Get して
//   ACTIVE を確認 → Delete → Get で DELETE_REQUESTED を確認。失敗時も
//   t.Cleanup で削除する。GCP_TEST_PARENT が未設定なら失敗させる。
// - 存在しないランダムな ID で Get: エラーになる(読み取りのみ)。
