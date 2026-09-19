package gcp

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

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
	// The suffix is random, so these check its properties, not its value.
	suffix := func(t *testing.T, id, name string) string {
		t.Helper()
		rest, ok := strings.CutPrefix(id, name+"-")
		if !ok {
			t.Fatalf("id %q does not start with %q", id, name+"-")
		}
		return rest
	}
	isDigits := func(s string) bool {
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return s != ""
	}

	t.Run("triedが空ならnameそのもの", func(t *testing.T) {
		for _, n := range []int{1, 6, 30} {
			name := strings.Repeat("a", n)
			id, ok := nextProjectID(name, nil)
			if !ok || id != name {
				t.Errorf("len %d: nextProjectID = %q, %v, want %q, true", n, id, ok, name)
			}
		}
	})

	t.Run("接尾辞は数字で桁数が短縮され全体が30文字以下", func(t *testing.T) {
		cases := []struct {
			nameLen    int // length of the project name
			wantDigits int // expected length of the suffix
			desc       string
		}{
			{nameLen: 1, wantDigits: 6, desc: "短い name は6桁"},
			{nameLen: 10, wantDigits: 6, desc: "10文字でも6桁"},
			{nameLen: 23, wantDigits: 6, desc: "6桁のまま収まる最長の name(23 + 1 + 6 = 30)"},
			{nameLen: 24, wantDigits: 5, desc: "24文字は5桁に短縮(24 + 1 + 5 = 30)"},
			{nameLen: 25, wantDigits: 4, desc: "25文字は4桁に短縮"},
			{nameLen: 26, wantDigits: 3, desc: "26文字は3桁に短縮"},
			{nameLen: 27, wantDigits: 2, desc: "27文字は2桁に短縮"},
			{nameLen: 28, wantDigits: 1, desc: "接尾辞が入る最長の name(28 + 1 + 1 = 30)"},
		}
		for _, c := range cases {
			name := strings.Repeat("a", c.nameLen)
			id, ok := nextProjectID(name, []string{name})
			if !ok {
				t.Errorf("%s: ok = false, want true", c.desc)
				continue
			}
			sfx := suffix(t, id, name)
			if !isDigits(sfx) || len(sfx) != c.wantDigits {
				t.Errorf("%s: suffix = %q, want %d digits", c.desc, sfx, c.wantDigits)
			}
			if len(id) > 30 {
				t.Errorf("%s: id %q is %d characters, want at most 30", c.desc, id, len(id))
			}
		}
	})

	t.Run("接尾辞が入らないnameではfalse", func(t *testing.T) {
		for _, n := range []int{29, 30} {
			name := strings.Repeat("a", n)
			if id, ok := nextProjectID(name, []string{name}); ok {
				t.Errorf("len %d: nextProjectID = %q, true, want false", n, id)
			}
		}
	})

	t.Run("triedに含まれるIDは返さない", func(t *testing.T) {
		// A 28-character name has a 1-digit suffix, so only 10 IDs exist.
		name := strings.Repeat("a", 28)
		tried := []string{name}
		for d := 0; d < 9; d++ {
			tried = append(tried, fmt.Sprintf("%s-%d", name, d))
		}
		id, ok := nextProjectID(name, tried)
		if !ok || id != name+"-9" {
			t.Errorf("nextProjectID = %q, %v, want %q, true", id, ok, name+"-9")
		}

		tried = append(tried, name+"-9")
		if id, ok := nextProjectID(name, tried); ok {
			t.Errorf("all IDs tried: nextProjectID = %q, true, want false", id)
		}
	})

	t.Run("triedを増やしながら5回呼ぶとすべて異なる", func(t *testing.T) {
		name := "my-project"
		tried := []string{name}
		for i := 0; i < 5; i++ {
			id, ok := nextProjectID(name, tried)
			if !ok {
				t.Fatalf("call %d: ok = false, want true", i)
			}
			if slices.Contains(tried, id) {
				t.Fatalf("call %d: %q was already tried", i, id)
			}
			tried = append(tried, id)
		}
	})
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
