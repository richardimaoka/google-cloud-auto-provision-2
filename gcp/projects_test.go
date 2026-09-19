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
//
// The client holds its project ID generator (nextProjectID) in a field, so the
// Create tests can replace it with a scripted sequence of IDs instead of random
// ones.
//
// The fake builds its responses for each test case. Their shapes follow the
// samples in sampledata/, which are only for reference.

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

	// - 最初の試行: リクエストの projectId が name そのもの、displayName も name
	// - クライアントの parent がリクエストの parent に入る
	// - クライアントの parent が空のときは parent がリクエストに含まれない
	// - 作成が受理され(未完了のオペレーションが返る)、最初のポーリングで完了する:
	//   response の Project が返る
	// - オペレーションが数回のポーリング後に完了する: ポーリングが繰り返される
	//
	// ID の衝突とリトライ(衝突は作成リクエストに対して即座に 409 で返る):
	// - 最初の試行が 409 ALREADY_EXISTS で衝突: 接尾辞付きの ID で再試行し、
	//   2回目で成功したプロジェクトが返る
	// - 2回衝突してから成功する: 3回目の試行で成功し、ID は毎回異なる
	// - 6回すべて衝突: 作成リクエストはちょうど6回で、最後の衝突のエラーを返す
	// - 再試行中に衝突以外の失敗(403 など): それ以上試行せずそのエラーを返す
	// - name が長すぎて接尾辞が入らない(29〜30文字): 最初の試行が衝突したら
	//   リトライせず、作成リクエストは1回だけでエラーを返す
	// - name が28文字: 接尾辞が1桁に短縮された ID で再試行する
	//
	// その他の失敗:
	// - 作成リクエスト自体が拒否される(403 など): "create project" を含むエラー
	// - オペレーションが衝突以外のエラーで完了する(クォータ超過など): メッセージを含むエラー
	// - ポーリング中に 500: ハングせずエラーを返す
	// - (未決定) ポーリング中に context がキャンセルされる: 待機を止めてエラーを返す
}

func TestNextProjectID(t *testing.T) {
	t.Skip("not implemented")

	// 接尾辞は乱数なので、値ではなく性質を検証する。
	//
	// - tried が空: name そのものを返す(30文字の name でも同じ)
	// - tried が空でない: name + "-" + 数字6桁 を返す
	// - 接尾辞は数字だけで、ID全体が30文字以下
	// - name が23文字以下: 接尾辞は6桁
	// - name が24〜28文字: 接尾辞が 30 - len(name) - 1 桁に短縮される
	//   (24文字なら5桁、28文字なら1桁)
	// - name が29〜30文字で tried が空でない: false を返す
	// - tried に含まれる ID は返さない
	// - tried を増やしながら5回呼ぶ: すべて異なる ID が返る
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
// "integration" build tag). Create and Delete change a real organization. Each
// run leaves a project in DELETE_REQUESTED, and the ID of a deleted project can
// never be used again, so these are still undecided.
//
// - (未決定) ライフサイクル: ランダムな名前で Create → 返された ID で Get して
//   ACTIVE を確認 → Delete → Get で DELETE_REQUESTED を確認。失敗時も
//   t.Cleanup で削除する。GCP_TEST_PARENT が未設定なら失敗させる。
// - 存在しないランダムな ID で Get: エラーになる(読み取りのみ)。
