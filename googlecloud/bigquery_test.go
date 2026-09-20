package googlecloud

import "testing"

func TestBigQueryClient_GetDataset(t *testing.T) {
	t.Run("存在するデータセットのIDとロケーションを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないデータセットはエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_GetTable(t *testing.T) {
	t.Run("存在するテーブルのIDとスキーマ（列名、型、モード）を返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないデータセットのテーブルは、テーブルが無い場合と区別できるエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestDiffSchema(t *testing.T) {
	t.Run("期待どおりのスキーマなら、差分はない", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("足りない列を報告する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("型が違う列を報告する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("期待にない余分な列は、差分にしない", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_query(t *testing.T) {
	t.Run("ジョブが未完了なら、完了まで待って結果を返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("ジョブが失敗したら、原因が分かるエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("結果が複数ページでも、全行を返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("待っている間にコンテキストがキャンセルされたら戻る", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_Query(t *testing.T) {
	t.Run("結果の行を、列名をキーにした値で返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("TIMESTAMP、NULL、入れ子の列を変換する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("値はSQL文字列に埋め込まず、クエリパラメータとして送る", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("SQL の文法エラーは、原因が分かるエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_Insert(t *testing.T) {
	t.Run("1行を追記する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("複数行を追記する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("行が0件なら、ジョブを送らずに成功する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("値はSQL文字列に埋め込まず、クエリパラメータとして送る", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("不正な文字を含むテーブル名と列名は拒否する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_Upsert(t *testing.T) {
	t.Run("キーが同じ行がなければ、新しい行を挿入する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("キーが同じ行があれば、その行を置き換える", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("複数行を Upsert する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("値はSQL文字列に埋め込まず、クエリパラメータとして送る", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("不正な文字を含むテーブル名と列名は拒否する", func(t *testing.T) {
		t.Skip("not implemented")
	})
	t.Run("存在しないテーブルはエラーを返す", func(t *testing.T) {
		t.Skip("not implemented")
	})
}
