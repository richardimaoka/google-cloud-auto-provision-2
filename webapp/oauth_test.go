package webapp

import (
	"encoding/json"
	"testing"
)

func TestParseClientConfig(t *testing.T) {
	client := func(uris ...string) []byte {
		b, _ := json.Marshal(map[string]any{"web": map[string]any{
			"client_id":     "id.apps.googleusercontent.com",
			"client_secret": "not-a-real-secret",
			"redirect_uris": uris,
		}})
		return b
	}

	cases := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "localhostのURIが1つ", data: client("http://localhost:8080/oauth2/callback")},
		{name: "127.0.0.1のURIが1つ", data: client("http://127.0.0.1:8080/oauth2/callback")},
		{name: "URIが複数", data: client("http://localhost:8080/a", "http://localhost:8080/b"), wantErr: true},
		{name: "URIがない", data: client(), wantErr: true},
		{name: "httpsのURI", data: client("https://localhost:8080/cb"), wantErr: true},
		{name: "localhost以外のホスト", data: client("http://example.com:8080/cb"), wantErr: true},
		{name: "ポートがない", data: client("http://localhost/cb"), wantErr: true},
		{name: "パスがない", data: client("http://localhost:8080"), wantErr: true},
		{name: "webではなくinstalled", data: []byte(`{"installed":{"client_id":"x","client_secret":"y","redirect_uris":["http://localhost:8080/cb"]}}`), wantErr: true},
		{name: "JSONではない", data: []byte("not json"), wantErr: true},
	}
	for _, c := range cases {
		cfg, redirect, err := parseClientConfig(c.data)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if err == nil && cfg.RedirectURL != redirect.String() {
			t.Errorf("%s: cfg.RedirectURL = %q, want %q", c.name, cfg.RedirectURL, redirect.String())
		}
	}
}
