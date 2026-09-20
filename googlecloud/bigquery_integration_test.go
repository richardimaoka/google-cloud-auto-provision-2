//go:build integration

package googlecloud

import "testing"

func TestBigQueryClient_GetTable_Integration(t *testing.T) {
	t.Run("作成したテーブルの、スキーマを取得できる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_Insert_Integration(t *testing.T) {
	t.Run("追記した行を、Query で読み戻せる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}

func TestBigQueryClient_Upsert_Integration(t *testing.T) {
	t.Run("同じキーで2回 Upsert すると、行は1つで、内容は新しいほうになる", func(t *testing.T) {
		t.Skip("not implemented")
	})
}
