package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// newFakeProjectsServer fakes the Resource Manager project endpoint that Get
// relies on. It answers status for GET /v3/projects/my-project, returning
// project when status is 200, and 404 for any other project ID.
func newFakeProjectsServer(t *testing.T, status int, project *cloudresourcemanager.Project) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v3/projects/my-project", func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		json.NewEncoder(w).Encode(project)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

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
	cases := []struct {
		name    string
		status  int
		project *cloudresourcemanager.Project
		wantErr string // substring of the expected error; empty means success
		want    *Project
	}{
		{
			name:   "既存のプロジェクト",
			status: http.StatusOK,
			project: &cloudresourcemanager.Project{
				ProjectId:   "my-project",
				DisplayName: "My Project",
				Parent:      "organizations/100000000000",
				State:       "ACTIVE",
			},
			want: &Project{ID: "my-project", Name: "My Project", Parent: "organizations/100000000000", State: "ACTIVE"},
		},
		{
			name:   "親を持たないプロジェクト",
			status: http.StatusOK,
			project: &cloudresourcemanager.Project{
				ProjectId:   "my-project",
				DisplayName: "My Project",
				State:       "ACTIVE",
			},
			want: &Project{ID: "my-project", Name: "My Project", State: "ACTIVE"},
		},
		{
			name:   "DELETE_REQUESTED 状態のプロジェクト",
			status: http.StatusOK,
			project: &cloudresourcemanager.Project{
				ProjectId:   "my-project",
				DisplayName: "My Project",
				State:       "DELETE_REQUESTED",
			},
			want: &Project{ID: "my-project", Name: "My Project", State: "DELETE_REQUESTED"},
		},
		{
			name:    "存在しないプロジェクト",
			status:  http.StatusNotFound,
			wantErr: "get project",
		},
		{
			name:    "権限不足",
			status:  http.StatusForbidden,
			wantErr: "get project",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newFakeProjectsServer(t, c.status, c.project)
			client, err := NewProjectsClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
			if err != nil {
				t.Fatalf("NewProjectsClient: %v", err)
			}

			got, err := client.Get(context.Background(), "my-project")
			if c.wantErr != "" {
				if err == nil {
					t.Fatal("Get() = nil error, want error")
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("Get() error = %q, want it to contain %q", err, c.wantErr)
				}
				if got != nil {
					t.Errorf("Get() = %+v on error, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get() = %v, want nil error", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Get() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// fakeResponse is one scripted answer of the fake server: an error status with
// an optional body, or status 200 with op as the body.
type fakeResponse struct {
	status int
	body   string
	op     *cloudresourcemanager.Operation
}

func (r fakeResponse) write(w http.ResponseWriter) {
	if r.status != http.StatusOK {
		w.WriteHeader(r.status)
		io.WriteString(w, r.body)
		return
	}
	json.NewEncoder(w).Encode(r.op)
}

// newPollHandler serves GET /v3/operations/...: each poll answers the next entry
// of polls and is counted in count.
func newPollHandler(t *testing.T, polls []fakeResponse, count *int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if *count >= len(polls) {
			t.Errorf("unexpected poll %d, only %d scripted", *count+1, len(polls))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		polls[*count].write(w)
		*count++
	}
}

// newFakeDeleteServer fakes DELETE /v3/projects/my-project and the polling of
// the operation it starts. The delete call answers del, and each poll of
// GET /v3/operations/... answers the next entry of polls. It also returns a
// pointer to the number of polls received.
func newFakeDeleteServer(t *testing.T, del fakeResponse, polls []fakeResponse) (*httptest.Server, *int) {
	t.Helper()

	pollCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /v3/projects/my-project", func(w http.ResponseWriter, r *http.Request) {
		del.write(w)
	})
	mux.HandleFunc("GET /v3/operations/", newPollHandler(t, polls, &pollCount))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &pollCount
}

// createRequests records what the fake create server received.
type createRequests struct {
	projects []cloudresourcemanager.Project // bodies of the create requests
	polls    int
}

// newFakeCreateServer fakes POST /v3/projects and the polling of the operation
// it starts. Each create request answers the next entry of creates, and each
// poll the next entry of polls.
func newFakeCreateServer(t *testing.T, creates, polls []fakeResponse) (*httptest.Server, *createRequests) {
	t.Helper()

	got := &createRequests{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v3/projects", func(w http.ResponseWriter, r *http.Request) {
		var p cloudresourcemanager.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode create request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		got.projects = append(got.projects, p)

		i := len(got.projects) - 1
		if i >= len(creates) {
			t.Errorf("unexpected create request %d, only %d scripted", i+1, len(creates))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		creates[i].write(w)
	})
	mux.HandleFunc("GET /v3/operations/", newPollHandler(t, polls, &got.polls))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
}

func (r *createRequests) ids() []string {
	var ids []string
	for _, p := range r.projects {
		ids = append(ids, p.ProjectId)
	}
	return ids
}

func newTestProjectsClient(t *testing.T, endpoint string) *ProjectsClient {
	t.Helper()

	client, err := NewProjectsClient(context.Background(), option.WithEndpoint(endpoint), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewProjectsClient: %v", err)
	}
	client.pollInterval = time.Millisecond
	return client
}

// scriptedIDs makes a nextID function that returns ids in order, and checks
// that Create passes the IDs it has already tried.
func scriptedIDs(t *testing.T, ids ...string) func(string, []string) (string, bool) {
	return func(name string, tried []string) (string, bool) {
		if !slices.Equal(tried, ids[:min(len(tried), len(ids))]) {
			t.Errorf("tried = %v, want a prefix of %v", tried, ids)
		}
		if len(tried) >= len(ids) {
			return "", false
		}
		return ids[len(tried)], true
	}
}

func TestProjectsClient_Create(t *testing.T) {
	const name = "my-project"

	running := &cloudresourcemanager.Operation{Name: "operations/op-1"}
	finished := func(id, parent string) *cloudresourcemanager.Operation {
		b, err := json.Marshal(&cloudresourcemanager.Project{ProjectId: id, DisplayName: name, Parent: parent, State: "ACTIVE"})
		if err != nil {
			t.Fatal(err)
		}
		return &cloudresourcemanager.Operation{Name: "operations/op-1", Done: true, Response: googleapi.RawMessage(b)}
	}
	failed := &cloudresourcemanager.Operation{
		Name:  "operations/op-1",
		Done:  true,
		Error: &cloudresourcemanager.Status{Code: 8, Message: "quota exceeded"},
	}
	ok := func(op *cloudresourcemanager.Operation) fakeResponse {
		return fakeResponse{status: http.StatusOK, op: op}
	}
	status := func(code int) fakeResponse { return fakeResponse{status: code} }
	conflict := fakeResponse{
		status: http.StatusConflict,
		body:   `{"error":{"code":409,"message":"Requested entity already exists","status":"ALREADY_EXISTS"}}`,
	}

	const parent = "organizations/100000000000"
	suffixed := []string{name, name + "-111111", name + "-222222", name + "-333333", name + "-444444", name + "-555555"}

	cases := []struct {
		name           string
		responseParent string   // parent in the finished operation's response
		ids            []string // IDs to try in order; nil uses nextProjectID
		creates        []fakeResponse
		polls          []fakeResponse
		wantErr        string // substring of the expected error; empty means success
		wantConflict   bool   // the error wraps the 409 from the last attempt
		wantID         string // ID of the returned project
		wantRequested  []string
		wantPolls      int
	}{
		{
			name:          "最初の試行でnameをIDにしてプロジェクトが作られる",
			creates:       []fakeResponse{ok(running)},
			polls:         []fakeResponse{ok(finished(name, ""))},
			wantID:        name,
			wantRequested: []string{name},
			wantPolls:     1,
		},
		{
			name:           "完了したオペレーションの親がProject.Parentに入る",
			responseParent: parent,
			creates:        []fakeResponse{ok(running)},
			polls:          []fakeResponse{ok(finished(name, parent))},
			wantID:         name,
			wantRequested:  []string{name},
			wantPolls:      1,
		},
		{
			name:          "オペレーションが数回のポーリング後に完了する",
			creates:       []fakeResponse{ok(running)},
			polls:         []fakeResponse{ok(running), ok(running), ok(finished(name, ""))},
			wantID:        name,
			wantRequested: []string{name},
			wantPolls:     3,
		},
		{
			name:          "最初の試行が衝突し、接尾辞付きのIDで再試行して成功する",
			ids:           suffixed,
			creates:       []fakeResponse{conflict, ok(running)},
			polls:         []fakeResponse{ok(finished(suffixed[1], ""))},
			wantID:        suffixed[1],
			wantRequested: suffixed[:2],
			wantPolls:     1,
		},
		{
			name:          "2回衝突してから3回目で成功する",
			ids:           suffixed,
			creates:       []fakeResponse{conflict, conflict, ok(running)},
			polls:         []fakeResponse{ok(finished(suffixed[2], ""))},
			wantID:        suffixed[2],
			wantRequested: suffixed[:3],
			wantPolls:     1,
		},
		{
			name:          "6回すべて衝突すると最後の衝突のエラーを返す",
			ids:           suffixed,
			creates:       []fakeResponse{conflict, conflict, conflict, conflict, conflict, conflict},
			wantErr:       "no free project ID",
			wantConflict:  true,
			wantRequested: suffixed,
		},
		{
			name:          "再試行中の衝突以外の失敗ではそれ以上試行しない",
			ids:           suffixed,
			creates:       []fakeResponse{conflict, status(http.StatusForbidden)},
			wantErr:       "create project",
			wantRequested: suffixed[:2],
		},
		{
			name:          "作成リクエストが拒否される",
			creates:       []fakeResponse{status(http.StatusForbidden)},
			wantErr:       "create project",
			wantRequested: []string{name},
		},
		{
			name:          "オペレーションが衝突以外のエラーで完了する",
			creates:       []fakeResponse{ok(running)},
			polls:         []fakeResponse{ok(failed)},
			wantErr:       "quota exceeded",
			wantRequested: []string{name},
			wantPolls:     1,
		},
		{
			name:          "ポーリング中に500",
			creates:       []fakeResponse{ok(running)},
			polls:         []fakeResponse{ok(running), status(http.StatusInternalServerError)},
			wantErr:       "create project",
			wantRequested: []string{name},
			wantPolls:     2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, got := newFakeCreateServer(t, c.creates, c.polls)
			client := newTestProjectsClient(t, srv.URL)
			if c.ids != nil {
				client.nextID = scriptedIDs(t, c.ids...)
			}

			project, err := client.Create(context.Background(), name)

			if !slices.Equal(got.ids(), c.wantRequested) {
				t.Errorf("requested project IDs = %v, want %v", got.ids(), c.wantRequested)
			}
			for _, p := range got.projects {
				if p.DisplayName != name || p.Parent != "" {
					t.Errorf("request for %q has displayName %q and parent %q, want %q and no parent", p.ProjectId, p.DisplayName, p.Parent, name)
				}
			}
			if got.polls != c.wantPolls {
				t.Errorf("polled %d times, want %d", got.polls, c.wantPolls)
			}

			if c.wantErr != "" {
				if err == nil {
					t.Fatal("Create() = nil error, want error")
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("Create() error = %q, want it to contain %q", err, c.wantErr)
				}
				var gerr *googleapi.Error
				if c.wantConflict && !(errors.As(err, &gerr) && gerr.Code == http.StatusConflict) {
					t.Errorf("Create() error = %v, want it to wrap a 409", err)
				}
				if project != nil {
					t.Errorf("Create() = %+v on error, want nil", project)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() = %v, want nil error", err)
			}
			want := &Project{ID: c.wantID, Name: name, Parent: c.responseParent, State: "ACTIVE"}
			if !reflect.DeepEqual(project, want) {
				t.Errorf("Create() = %+v, want %+v", project, want)
			}
		})
	}

	// These use the real ID generator, so the IDs after the first are random.
	t.Run("接尾辞が入らないnameは衝突してもリトライしない", func(t *testing.T) {
		for _, n := range []int{29, 30} {
			long := strings.Repeat("a", n)
			srv, got := newFakeCreateServer(t, []fakeResponse{conflict}, nil)
			client := newTestProjectsClient(t, srv.URL)

			if _, err := client.Create(context.Background(), long); err == nil {
				t.Errorf("len %d: Create() = nil error, want error", n)
			}
			if !slices.Equal(got.ids(), []string{long}) {
				t.Errorf("len %d: requested project IDs = %v, want only %q", n, got.ids(), long)
			}
		}
	})

	t.Run("28文字のnameは1桁の接尾辞で再試行する", func(t *testing.T) {
		long := strings.Repeat("a", 28)
		srv, got := newFakeCreateServer(t, []fakeResponse{conflict, ok(running)}, []fakeResponse{ok(finished("x", ""))})
		client := newTestProjectsClient(t, srv.URL)

		if _, err := client.Create(context.Background(), long); err != nil {
			t.Fatalf("Create() = %v, want nil error", err)
		}
		ids := got.ids()
		if len(ids) != 2 || ids[0] != long {
			t.Fatalf("requested project IDs = %v, want %q then a suffixed ID", ids, long)
		}
		if sfx, _ := strings.CutPrefix(ids[1], long+"-"); len(sfx) != 1 || len(ids[1]) != 30 {
			t.Errorf("retry ID = %q, want %q plus a hyphen and 1 digit", ids[1], long)
		}
	})

	// (未決定) ポーリング中に context がキャンセルされる: 待機を止めてエラーを返す
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
	running := &cloudresourcemanager.Operation{Name: "operations/op-1"}
	done := &cloudresourcemanager.Operation{Name: "operations/op-1", Done: true}
	failed := &cloudresourcemanager.Operation{
		Name:  "operations/op-1",
		Done:  true,
		Error: &cloudresourcemanager.Status{Code: 9, Message: "project cannot be deleted"},
	}
	ok := func(op *cloudresourcemanager.Operation) fakeResponse {
		return fakeResponse{status: http.StatusOK, op: op}
	}
	status := func(code int) fakeResponse { return fakeResponse{status: code} }

	cases := []struct {
		name      string
		del       fakeResponse
		polls     []fakeResponse
		wantErr   string // substring of the expected error; empty means success
		wantPolls int
	}{
		{
			name: "オペレーションが最初のレスポンスで完了している",
			del:  ok(done),
		},
		{
			name:      "オペレーションが数回のポーリング後に完了する",
			del:       ok(running),
			polls:     []fakeResponse{ok(running), ok(running), ok(done)},
			wantPolls: 3,
		},
		{
			name:    "存在しないプロジェクト",
			del:     status(http.StatusNotFound),
			wantErr: "delete project",
		},
		{
			name:    "権限不足",
			del:     status(http.StatusForbidden),
			wantErr: "delete project",
		},
		{
			name:      "オペレーションがエラーで完了する",
			del:       ok(running),
			polls:     []fakeResponse{ok(failed)},
			wantErr:   "project cannot be deleted",
			wantPolls: 1,
		},
		{
			name:      "ポーリング中に 500",
			del:       ok(running),
			polls:     []fakeResponse{ok(running), status(http.StatusInternalServerError)},
			wantErr:   "delete project",
			wantPolls: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, polls := newFakeDeleteServer(t, c.del, c.polls)
			client, err := NewProjectsClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
			if err != nil {
				t.Fatalf("NewProjectsClient: %v", err)
			}
			client.pollInterval = time.Millisecond

			err = client.Delete(context.Background(), "my-project")
			if c.wantErr != "" {
				if err == nil {
					t.Fatal("Delete() = nil error, want error")
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("Delete() error = %q, want it to contain %q", err, c.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Delete() = %v, want nil error", err)
			}
			if *polls != c.wantPolls {
				t.Errorf("polled %d times, want %d", *polls, c.wantPolls)
			}
		})
	}
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
