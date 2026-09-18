package gcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	cloudidentity "google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

// newFakeGroupsServer fakes the two Cloud Identity Groups endpoints Get
// relies on. lookupStatus is returned for the group lookup; pages is served,
// one page per call, from the memberships list. A nil page makes the list call
// fail with a 500 instead.
func newFakeGroupsServer(t *testing.T, lookupStatus int, pages [][]*cloudidentity.Membership) *httptest.Server {
	t.Helper()

	pageIdx := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/groups:lookup", func(w http.ResponseWriter, r *http.Request) {
		if lookupStatus != http.StatusOK {
			w.WriteHeader(lookupStatus)
			return
		}
		json.NewEncoder(w).Encode(&cloudidentity.LookupGroupNameResponse{Name: "groups/1"})
	})
	mux.HandleFunc("/v1/groups/1/memberships", func(w http.ResponseWriter, r *http.Request) {
		if pages[pageIdx] == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := &cloudidentity.ListMembershipsResponse{Memberships: pages[pageIdx]}
		pageIdx++
		if pageIdx < len(pages) {
			resp.NextPageToken = "next"
		}
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func membership(email string, roles ...string) *cloudidentity.Membership {
	m := &cloudidentity.Membership{
		PreferredMemberKey: &cloudidentity.EntityKey{Id: email},
	}
	for _, r := range roles {
		m.Roles = append(m.Roles, &cloudidentity.MembershipRole{Name: r})
	}
	return m
}

func TestGroupsClient_Get(t *testing.T) {
	cases := []struct {
		name        string
		lookupCode  int
		pages       [][]*cloudidentity.Membership
		wantErr     string // substring of the expected error; empty means success
		wantMembers []Member
	}{
		{
			name:       "メンバー0人のグループ",
			lookupCode: http.StatusOK,
			pages:      [][]*cloudidentity.Membership{{}},
		},
		{
			name:       "メンバーが複数かつロールが混在するグループ",
			lookupCode: http.StatusOK,
			pages: [][]*cloudidentity.Membership{{
				membership("owner@example.com", "OWNER"),
				membership("member@example.com", "MEMBER"),
			}},
			wantMembers: []Member{
				{Email: "owner@example.com", Role: "OWNER", Domain: "example.com"},
				{Email: "member@example.com", Role: "MEMBER", Domain: "example.com"},
			},
		},
		{
			// The API lists an owner's roles as both OWNER and MEMBER.
			name:       "複数のロールを持つメンバーは最も強いロールになる",
			lookupCode: http.StatusOK,
			pages: [][]*cloudidentity.Membership{{
				membership("owner@example.com", "OWNER", "MEMBER"),
				membership("manager@example.com", "MEMBER", "MANAGER"),
			}},
			wantMembers: []Member{
				{Email: "owner@example.com", Role: "OWNER", Domain: "example.com"},
				{Email: "manager@example.com", Role: "MANAGER", Domain: "example.com"},
			},
		},
		{
			name:       "メンバーが複数ページにまたがるグループ",
			lookupCode: http.StatusOK,
			pages: [][]*cloudidentity.Membership{
				{membership("a@example.com", "MEMBER")},
				{membership("b@example.com", "MEMBER")},
			},
			wantMembers: []Member{
				{Email: "a@example.com", Role: "MEMBER", Domain: "example.com"},
				{Email: "b@example.com", Role: "MEMBER", Domain: "example.com"},
			},
		},
		{
			name:       "存在しないグループ",
			lookupCode: http.StatusNotFound,
			wantErr:    "lookup group",
		},
		{
			name:       "権限不足",
			lookupCode: http.StatusForbidden,
			wantErr:    "lookup group",
		},
		{
			name:       "メンバー一覧の取得が1ページ目で失敗する",
			lookupCode: http.StatusOK,
			pages:      [][]*cloudidentity.Membership{nil},
			wantErr:    "list memberships",
		},
		{
			name:       "メンバー一覧の取得が2ページ目で失敗する",
			lookupCode: http.StatusOK,
			pages: [][]*cloudidentity.Membership{
				{membership("a@example.com", "MEMBER")},
				nil,
			},
			wantErr: "list memberships",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newFakeGroupsServer(t, c.lookupCode, c.pages)
			client, err := NewGroupsClient(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
			if err != nil {
				t.Fatalf("NewGroupsClient: %v", err)
			}

			got, err := client.Get(context.Background(), "group@example.com")
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
			if !reflect.DeepEqual(got.Members, c.wantMembers) {
				t.Errorf("Get() Members = %+v, want %+v", got.Members, c.wantMembers)
			}
		})
	}
}

func TestEmailDomain(t *testing.T) {
	cases := []struct {
		name  string
		email string
		want  string
	}{
		{name: "通常のメールアドレス", email: "a@example.com", want: "example.com"},
		{name: "@がない", email: "nobody", want: ""},
		{name: "空文字列", email: "", want: ""},
		{name: "@が複数ある場合は最後の@以降", email: "a@b@example.com", want: "example.com"},
		{name: "@で終わる", email: "a@", want: ""},
		{name: "大文字小文字はそのまま", email: "a@Example.COM", want: "Example.COM"},
	}
	for _, c := range cases {
		if got := emailDomain(c.email); got != c.want {
			t.Errorf("%s: emailDomain(%q) = %q, want %q", c.name, c.email, got, c.want)
		}
	}
}

func TestStrongestRole(t *testing.T) {
	roles := func(names ...string) []*cloudidentity.MembershipRole {
		return membership("x@example.com", names...).Roles
	}

	cases := []struct {
		name  string
		roles []*cloudidentity.MembershipRole
		want  string
	}{
		{name: "ロールなし", roles: nil, want: ""},
		{name: "MEMBERのみ", roles: roles("MEMBER"), want: "MEMBER"},
		{name: "OWNERとMEMBER", roles: roles("OWNER", "MEMBER"), want: "OWNER"},
		{name: "順序が逆でもOWNERが最強", roles: roles("MEMBER", "OWNER"), want: "OWNER"},
		{name: "MANAGERとMEMBER", roles: roles("MEMBER", "MANAGER"), want: "MANAGER"},
		{name: "OWNERとMANAGERとMEMBER", roles: roles("MANAGER", "MEMBER", "OWNER"), want: "OWNER"},
		{name: "未知のロールは既知のロールより弱い", roles: roles("UNKNOWN", "MEMBER"), want: "MEMBER"},
		{name: "未知のロールのみ", roles: roles("UNKNOWN"), want: "UNKNOWN"},
		{name: "nilを含む", roles: append(roles("MEMBER"), nil), want: "MEMBER"},
	}
	for _, c := range cases {
		if got := strongestRole(c.roles); got != c.want {
			t.Errorf("%s: strongestRole() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGroup_SameDomain(t *testing.T) {
	cases := []struct {
		name    string
		domain  string
		members []Member
		want    bool
	}{
		{
			name:   "メンバーなし",
			domain: "example.com",
			want:   true,
		},
		{
			name:    "メンバーのドメインがグループのドメインと一致",
			domain:  "example.com",
			members: []Member{{Domain: "example.com"}},
			want:    true,
		},
		{
			name:   "全メンバーのドメインがグループのドメインと一致",
			domain: "example.com",
			members: []Member{
				{Email: "a@example.com", Domain: "example.com"},
				{Email: "b@example.com", Domain: "example.com"},
			},
			want: true,
		},
		{
			name:    "メンバーのドメインの大文字小文字だけが異なる",
			domain:  "example.com",
			members: []Member{{Email: "a@Example.com", Domain: "Example.com"}},
			want:    true,
		},
		{
			name:    "グループのドメインの大文字小文字だけが異なる",
			domain:  "Example.COM",
			members: []Member{{Email: "a@example.com", Domain: "example.com"}},
			want:    true,
		},
		{
			name:   "メンバーのドメインがグループのドメインと異なる",
			domain: "example.com",
			members: []Member{
				{Email: "a@example.com", Domain: "example.com"},
				{Email: "b@other.com", Domain: "other.com"},
			},
			want: false,
		},
	}
	for _, c := range cases {
		g := &Group{Domain: c.domain, Members: c.members}
		if got := g.SameDomain(); got != c.want {
			t.Errorf("%s: SameDomain() = %v, want %v", c.name, got, c.want)
		}
	}
}
