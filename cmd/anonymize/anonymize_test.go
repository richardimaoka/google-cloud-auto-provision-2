package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const samplesDataDir = "../../gcp/samples/data"

func compact(t *testing.T, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	return buf.String()
}

func readSamples(t *testing.T) map[string][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(samplesDataDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no samples found in %s (err: %v)", samplesDataDir, err)
	}
	samples := map[string][]byte{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		samples[filepath.Base(f)] = data
	}
	return samples
}

// A sample already holds only placeholders, so anonymizing it must change nothing.
func TestAnonymize_ExistingSamplesAreUnchanged(t *testing.T) {
	for name, data := range readSamples(t) {
		got, err := anonymize(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s changed:\n%s", name, got)
		}
	}
}

var errorIDRE = regexp.MustCompile(`(Ci[QR][A-Za-z0-9+/_-]+)((?:%3D|=)*)`)

// Put made-up real-looking values in place of the placeholders of a sample, the
// way a recorded response would have them, and anonymize it: the sample must
// come back. This checks the rules against the samples recorded so far.
func TestAnonymize_RestoresSamplesFromRealLookingValues(t *testing.T) {
	values := strings.NewReplacer(
		"projects/123456789012", "projects/555000111222",
		"projects/987654321098", "projects/444333222111",
		"organizations/100000000000", "organizations/777888999000",
		"1234567890123456789", "8882223334445556667",
		"0123456789abcdef", "deadbeefcafe0123",
		"my-project-123", "it-test-project-77",
		// The same length as the placeholder, so an encoded error ID keeps its padding.
		"this-does-not-exist-1", "someone-elses-proj-99",
		// Members of an IAM policy.
		"service-123456789012", "service-555000111222",
		"123456789012-compute", "555000111222-compute",
		"user:alice@example.com", "user:jane.roe@company.test",
		"user:bob@example.com", "user:joe.bloggs@company.test",
		"group:group-1@example.com", "group:team@company.test",
		"my-service-account", "svc-real-name",
		"BwAAAAAAAAA=", "BwZzzzzzzzz=",
	)
	resource := func(res string) (string, error) {
		return values.Replace(res), nil
	}

	for name, data := range readSamples(t) {
		text := values.Replace(string(data))
		var failure error
		text = errorIDRE.ReplaceAllStringFunc(text, func(m string) string {
			parts := errorIDRE.FindStringSubmatch(m)
			blob, err := rewriteErrorID(parts[1], parts[2], "a1b2c3d4-e5f6-4789-abcd-0123456789ab", resource)
			if err != nil {
				failure = err
			}
			return blob
		})
		if failure != nil {
			t.Errorf("%s: %v", name, failure)
			continue
		}

		got, err := anonymize([]byte(text))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s does not come back:\n%s", name, got)
		}
	}
}

func TestAnonymize_Rules(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "成功のレスポンスの識別子",
			in: `{"name":"projects/555000111222","parent":"organizations/777888999000","projectId":"real-proj-x",` +
				`"displayName":"real-proj-x","state":"ACTIVE","createTime":"2026-09-19T15:02:48.006Z","etag":"W/\"abcdef0123456789\""}`,
			want: `{"name":"projects/123456789012","parent":"organizations/100000000000","projectId":"my-project-123",` +
				`"displayName":"my-project-123","state":"ACTIVE","createTime":"2026-09-19T15:02:48.006Z","etag":"W/\"0123456789abcdef\""}`,
		},
		{
			name: "同じ値は同じプレースホルダー、別の値は次のプレースホルダー",
			in:   `{"a":"projects/111111111111","b":"projects/222222222222","c":"projects/111111111111"}`,
			want: `{"a":"projects/123456789012","b":"projects/234567890123","c":"projects/123456789012"}`,
		},
		{
			name: "エラーのレスポンスのプロジェクト番号は自分のものとは別の番号にする",
			in:   `{"error":{"code":403,"message":"denied on projects/555000111222"}}`,
			want: `{"error":{"code":403,"message":"denied on projects/987654321098"}}`,
		},
		{
			name: "オペレーションの番号",
			in:   `{"a":"operations/create_project.global.7050570549568373752","b":"operations/dp.7312384975121014873"}`,
			want: `{"a":"operations/create_project.global.1234567890123456789","b":"operations/dp.1234567890123456789"}`,
		},
		{
			name: "違反の subject はプロジェクト ID",
			in:   `{"error":{"details":[{"violations":[{"subject":"real-proj-x","description":"Project not active"}]}]}}`,
			want: `{"error":{"details":[{"violations":[{"subject":"my-project-123","description":"Project not active"}]}]}}`,
		},
		{
			name: "プロジェクト ID はメッセージの中も置き換える",
			in:   `{"projectId":"real-proj-x","message":"the project real-proj-x is here"}`,
			want: `{"projectId":"my-project-123","message":"the project my-project-123 is here"}`,
		},
		{
			name: "projectId にないプロジェクト名は変えない",
			in:   `{"error":{"message":"denied on projects/this-does-not-exist-1"}}`,
			want: `{"error":{"message":"denied on projects/this-does-not-exist-1"}}`,
		},
		{
			name: "IAM ポリシーのメンバーと etag",
			in: `{"version":1,"etag":"BwZzzzzzzzz=","bindings":[` +
				`{"role":"roles/owner","members":["user:jane.doe@company.test","group:team@company.test","domain:company.test","allUsers"]},` +
				`{"role":"roles/editor","members":["serviceAccount:555000111222-compute@developer.gserviceaccount.com",` +
				`"serviceAccount:svc-a@real-proj-x.iam.gserviceaccount.com",` +
				`"serviceAccount:service-555000111222@gcp-sa-pubsub.iam.gserviceaccount.com",` +
				`"serviceAccount:555000111222@cloudservices.gserviceaccount.com",` +
				`"serviceAccount:real-proj-x@appspot.gserviceaccount.com"]},` +
				`{"role":"roles/viewer","members":["projectViewer:real-proj-x","user:jane.doe@company.test","user:john@company.test"]}]}`,
			want: `{"version":1,"etag":"BwAAAAAAAAA=","bindings":[` +
				`{"role":"roles/owner","members":["user:alice@example.com","group:group-1@example.com","domain:example.com","allUsers"]},` +
				`{"role":"roles/editor","members":["serviceAccount:123456789012-compute@developer.gserviceaccount.com",` +
				`"serviceAccount:my-service-account@my-project-123.iam.gserviceaccount.com",` +
				`"serviceAccount:service-123456789012@gcp-sa-pubsub.iam.gserviceaccount.com",` +
				`"serviceAccount:123456789012@cloudservices.gserviceaccount.com",` +
				`"serviceAccount:my-project-123@appspot.gserviceaccount.com"]},` +
				`{"role":"roles/viewer","members":["projectViewer:my-project-123","user:alice@example.com","user:bob@example.com"]}]}`,
		},
		{
			name: "サービスアカウントのプロジェクト番号は projects/<番号> と同じプレースホルダー",
			in: `{"name":"projects/555000111222","bindings":[{"role":"roles/editor",` +
				`"members":["serviceAccount:555000111222-compute@developer.gserviceaccount.com"]}]}`,
			want: `{"name":"projects/123456789012","bindings":[{"role":"roles/editor",` +
				`"members":["serviceAccount:123456789012-compute@developer.gserviceaccount.com"]}]}`,
		},
		{
			name: "エラーのレスポンスで projectId にないプロジェクト ID は this-does-not-exist-N にする",
			in:   `{"error":{"message":"denied on projects/real-other-a, projects/real-other-b and projects/real-other-a"}}`,
			want: `{"error":{"message":"denied on projects/this-does-not-exist-1, projects/this-does-not-exist-2 and projects/this-does-not-exist-1"}}`,
		},
		{
			name: "作り物の名前(this-does-not-exist で始まる)は、ハイフンがなくても変えない",
			in:   `{"error":{"message":"denied on projects/this-does-not-existtttt123456"}}`,
			want: `{"error":{"message":"denied on projects/this-does-not-existtttt123456"}}`,
		},
		{
			name: "エラーのレスポンスでも projectId のプロジェクトは my-project-N のまま",
			in:   `{"error":{"message":"denied on projects/real-proj-x","details":[{"violations":[{"subject":"real-proj-x"}]}]}}`,
			want: `{"error":{"message":"denied on projects/my-project-123","details":[{"violations":[{"subject":"my-project-123"}]}]}}`,
		},
		{
			name: "example.com のメールアドレスは残ってもよい(大文字小文字は問わない)",
			in:   `{"a":"owner@example.com","b":"member@EXAMPLE.COM"}`,
			want: `{"a":"owner@example.com","b":"member@EXAMPLE.COM"}`,
		},
	}
	for _, c := range cases {
		got, err := anonymize([]byte(c.in))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if compact(t, got) != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, compact(t, got), c.want)
		}
	}
}

// The project IDs are collected from a map, and Go visits a map in a different
// order each time. They must still get their placeholders in the order they
// first appear in the text, and the same every time.
func TestAnonymize_ProjectIDsGetPlaceholdersInOrderOfAppearance(t *testing.T) {
	in := `{"first":{"projectId":"real-b"},"second":{"projectId":"real-a"},"message":"real-a and real-b"}`
	want := `{"first":{"projectId":"my-project-123"},"second":{"projectId":"my-project-456"},` +
		`"message":"my-project-456 and my-project-123"}`

	for i := 0; i < 50; i++ {
		got, err := anonymize([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		if compact(t, got) != want {
			t.Fatalf("run %d:\n got %s\nwant %s", i, compact(t, got), want)
		}
	}
}

func TestAnonymize_Format(t *testing.T) {
	// A zero-width space and a byte order mark, as can arrive by copy and paste.
	invisible := string(rune(0x200b)) + string(rune(0xfeff))
	got, err := anonymize([]byte("\n" + invisible + `{"a":[1,2],"b":{}}` + "\n" + invisible + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": {}\n}\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnonymize_Errors(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"JSONではない", `not json`, "not valid JSON"},
		{"メールアドレスが残る", `{"user":"someone@company.test"}`, "is left"},
		{"example.com を含むだけのドメイン", `{"user":"bob@notexample.com"}`, "is left"},
		{"example.com で終わるだけのドメイン", `{"user":"carol@counterexample.com"}`, "is left"},
		{"example.com で始まるだけのドメイン", `{"user":"dave@example.com.attacker.test"}`, "is left"},
		{"example.com のサブドメイン", `{"user":"erin@mail.example.com"}`, "is left"},
		{"別名のドメイン(以前は許していた)", `{"user":"emergency-group@example.com.test-google-a.com"}`, "is left"},
		{
			"IAM: 条件付きのバインディング",
			`{"bindings":[{"role":"roles/viewer","members":["allUsers"],"condition":{"title":"t"}}]}`,
			"has a condition",
		},
		{
			"IAM: 削除されたメンバー(uid が付く)",
			`{"bindings":[{"role":"roles/viewer","members":["deleted:user:someone@company.test?uid=123"]}]}`,
			"a member is in a form this tool doesn't know",
		},
		{
			"IAM: ワークロード ID のメンバー",
			`{"bindings":[{"role":"roles/viewer","members":["principalSet://iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/*"]}]}`,
			"a member is in a form this tool doesn't know",
		},
		{
			"IAM: 知らない形のサービスアカウント",
			`{"bindings":[{"role":"roles/viewer","members":["serviceAccount:someone@other.gserviceaccount.com"]}]}`,
			"a service account is in a form this tool doesn't know",
		},
		{
			"IAM: メンバー以外の場所にあるサービスアカウントのメールアドレスは残る",
			`{"note":"svc@real-proj-x.iam.gserviceaccount.com"}`,
			"is left",
		},
		{
			"IAM: ユーザーのプレースホルダーが足りない",
			`{"bindings":[{"role":"r","members":["user:a@x.test","user:b@x.test","user:c@x.test","user:d@x.test","user:e@x.test"]}]}`,
			"distinct user email addresses",
		},
		{
			"他のプロジェクトの ID のプレースホルダーが足りない",
			`{"error":{"message":"projects/real-other-a projects/real-other-b projects/real-other-c projects/real-other-d"}}`,
			"distinct project IDs of other projects",
		},
		{
			"プロジェクト番号のプレースホルダーが足りない",
			`{"a":"projects/111111111111","b":"projects/222222222222","c":"projects/333333333333","d":"projects/444444444444"}`,
			"distinct project numbers",
		},
		{
			"プロジェクト ID のプレースホルダーが足りない",
			`{"a":{"projectId":"real-a"},"b":{"projectId":"real-b"},"c":{"projectId":"real-c"},"d":{"projectId":"real-d"}}`,
			"distinct project IDs",
		},
	}
	for _, c := range cases {
		_, err := anonymize([]byte(c.in))
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.wantErr)
		}
	}
}

// makeErrorID builds an encoded error ID, with the padding in the given style:
// "" for none, "=" or "%3D".
func makeErrorID(uuid, res, style string) string {
	raw := append([]byte{0x0a, byte(len(uuid))}, uuid...)
	raw = append(raw, 0x12, byte(len(res)))
	raw = append(raw, res...)
	s := base64.StdEncoding.EncodeToString(raw)
	switch style {
	case "":
		return strings.TrimRight(s, "=")
	case "%3D":
		return strings.ReplaceAll(s, "=", "%3D")
	}
	return s
}

func decodeErrorID(t *testing.T, id string) string {
	t.Helper()
	id = strings.ReplaceAll(id, "%3D", "=")
	raw, err := base64.StdEncoding.DecodeString(id + strings.Repeat("=", (4-len(id)%4)%4))
	if err != nil {
		t.Fatalf("%q is not base64: %v", id, err)
	}
	return string(raw)
}

func TestAnonymize_ErrorIDs(t *testing.T) {
	const uuid = "a1b2c3d4-e5f6-4789-abcd-0123456789ab"

	t.Run("UUIDとプロジェクト番号を置き換え、パディングの書き方を保つ", func(t *testing.T) {
		for _, style := range []string{"", "=", "%3D"} {
			id := makeErrorID(uuid, "projects/555000111222", style)
			in := `{"error":{"message":"see errorId=` + id + ` .","details":[{"metadata":{"error_info_id":"` + id + `"}}]}}`

			got, err := anonymize([]byte(in))
			if err != nil {
				t.Fatalf("style %q: %v", style, err)
			}
			want := makeErrorID(zeroUUID, "projects/987654321098", style)
			if strings.Count(string(got), want) != 2 {
				t.Errorf("style %q: want the ID %q twice in %s", style, want, got)
			}
			if strings.Contains(string(got), id) {
				t.Errorf("style %q: the original ID is still in %s", style, got)
			}
		}
	})

	t.Run("プロジェクト ID が変わるとエンコード内の長さも合わせる", func(t *testing.T) {
		id := makeErrorID(uuid, "projects/real-proj-x", "")
		in := `{"projectId":"real-proj-x","id":"` + id + `"}`

		got, err := anonymize([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		var out struct{ ID string }
		if err := json.Unmarshal(bytes.ReplaceAll(got, []byte(`"id"`), []byte(`"ID"`)), &out); err != nil {
			t.Fatal(err)
		}
		want := "\n$" + zeroUUID + "\x12\x17projects/my-project-123" // 23 bytes
		if decoded := decodeErrorID(t, out.ID); decoded != want {
			t.Errorf("decoded ID = %q, want %q", decoded, want)
		}
	})

	t.Run("エラーのレスポンスで、メッセージとエンコード内の他のプロジェクトの ID は同じプレースホルダーにする", func(t *testing.T) {
		id := makeErrorID(uuid, "projects/real-other-a", "")
		in := `{"error":{"message":"denied on projects/real-other-a, errorId=` + id + `"}}`

		got, err := anonymize([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		want := makeErrorID(zeroUUID, "projects/this-does-not-exist-1", "")
		if !strings.Contains(string(got), "projects/this-does-not-exist-1, errorId="+want) {
			t.Errorf("want projects/this-does-not-exist-1 and the ID %q in %s", want, got)
		}
		if strings.Contains(string(got), "real-other-a") || strings.Contains(string(got), id) {
			t.Errorf("a real value is left in %s", got)
		}
	})

	t.Run("エンコード内にだけある他のプロジェクトの ID も置き換える", func(t *testing.T) {
		id := makeErrorID(uuid, "projects/real-other-a", "")
		in := `{"error":{"code":403,"id":"` + id + `"}}`

		got, err := anonymize([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		want := makeErrorID(zeroUUID, "projects/this-does-not-exist-1", "")
		if !strings.Contains(string(got), want) {
			t.Errorf("want the ID %q in %s", want, got)
		}
	})

	t.Run("知らない形式のエラーIDはエラーにして書かない", func(t *testing.T) {
		raw := append([]byte{0x0a, 36}, uuid...)
		raw = append(raw, 0x99, 3, 'a', 'b', 'c') // not the 0x12 field
		in := `{"id":"` + strings.TrimRight(base64.StdEncoding.EncodeToString(raw), "=") + `"}`

		if _, err := anonymize([]byte(in)); err == nil || !strings.Contains(err.Error(), "layout") {
			t.Errorf("err = %v, want it to mention the layout", err)
		}
	})
}

func TestRun_WritesToTheOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "raw", "projects.get.success.example.json")
	if err := os.MkdirAll(filepath.Dir(in), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(in, []byte(`{"name":"projects/555000111222"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "data", "projects.get.success.example.json")
	if err := run(in, out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"name\": \"projects/123456789012\"\n}\n"; string(got) != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
