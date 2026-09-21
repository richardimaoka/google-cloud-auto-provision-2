package googlecloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// newFakeBigQueryServer fakes the BigQuery dataset endpoint that GetDataset
// relies on. It answers status for GET /projects/my-project/datasets/my_dataset,
// returning dataset when status is 200, and 404 for any other project or dataset
// ID.
func newFakeBigQueryServer(t *testing.T, status int, dataset *bigquery.Dataset) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/datasets/my_dataset", func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		json.NewEncoder(w).Encode(dataset)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestBigQueryClient(t *testing.T, srv *httptest.Server) *BigQueryClient {
	t.Helper()

	client, err := NewBigQueryClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewBigQueryClient: %v", err)
	}
	return client
}

func TestBigQueryClient_GetDataset(t *testing.T) {
	t.Run("存在するデータセットのIDとロケーションを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusOK, &bigquery.Dataset{
			DatasetReference: &bigquery.DatasetReference{ProjectId: "my-project", DatasetId: "my_dataset"},
			Location:         "US",
		})
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err != nil {
			t.Fatalf("GetDataset() = %v, want nil error", err)
		}
		want := &Dataset{ID: "my_dataset", Location: "US"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetDataset() = %+v, want %+v", got, want)
		}
	})
	t.Run("存在しないデータセットはエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusNotFound, nil)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err == nil {
			t.Fatal("GetDataset() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "get dataset") {
			t.Errorf("GetDataset() error = %q, want it to contain %q", err, "get dataset")
		}
		if got != nil {
			t.Errorf("GetDataset() = %+v on error, want nil", got)
		}
	})
	t.Run("権限不足はエラーを返す", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusForbidden, nil)
		client := newTestBigQueryClient(t, srv)

		got, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		if err == nil {
			t.Fatal("GetDataset() = nil error, want error")
		}
		if !strings.Contains(err.Error(), "get dataset") {
			t.Errorf("GetDataset() error = %q, want it to contain %q", err, "get dataset")
		}
		if got != nil {
			t.Errorf("GetDataset() = %+v on error, want nil", got)
		}
	})
	t.Run("エラーから *googleapi.Error を取り出せる", func(t *testing.T) {
		srv := newFakeBigQueryServer(t, http.StatusNotFound, nil)
		client := newTestBigQueryClient(t, srv)

		_, err := client.GetDataset(context.Background(), "my-project", "my_dataset")
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) {
			t.Fatalf("GetDataset() error = %v, want it to wrap a *googleapi.Error", err)
		}
		if gerr.Code != http.StatusNotFound {
			t.Errorf("googleapi.Error.Code = %d, want %d", gerr.Code, http.StatusNotFound)
		}
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
