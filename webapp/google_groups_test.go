package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"golang.org/x/oauth2"
	cloudidentity "google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

// cloudIdentityRequests records what newFakeCloudIdentityServer received.
type cloudIdentityRequests struct {
	// auth holds each request's Authorization header.
	auth []string
	// groupKeys holds each lookup's groupKey.id, the address asked for.
	groupKeys []string
}

// newFakeCloudIdentityServer serves one group with memberships, or fails the
// lookup with lookupStatus if it isn't 200.
func newFakeCloudIdentityServer(t *testing.T, lookupStatus int, memberships []*cloudidentity.Membership) (*httptest.Server, *cloudIdentityRequests) {
	t.Helper()
	got := &cloudIdentityRequests{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/groups:lookup", func(w http.ResponseWriter, r *http.Request) {
		got.auth = append(got.auth, r.Header.Get("Authorization"))
		got.groupKeys = append(got.groupKeys, r.URL.Query().Get("groupKey.id"))
		if lookupStatus != http.StatusOK {
			w.WriteHeader(lookupStatus)
			return
		}
		json.NewEncoder(w).Encode(&cloudidentity.LookupGroupNameResponse{Name: "groups/1"})
	})
	mux.HandleFunc("/v1/groups/1/memberships", func(w http.ResponseWriter, r *http.Request) {
		got.auth = append(got.auth, r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(&cloudidentity.ListMembershipsResponse{Memberships: memberships})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
}

func testMembership(email, role string) *cloudidentity.Membership {
	return &cloudidentity.Membership{
		PreferredMemberKey: &cloudidentity.EntityKey{Id: email},
		Roles:              []*cloudidentity.MembershipRole{{Name: role}},
	}
}

func TestCloudIdentityGroups_LookUp(t *testing.T) {
	token := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "user-token"})
	groupEmail, err := domain.NewEmail("developer-group@example.com")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("利用者のtokenでグループとメンバーを取得し、domainの型に変換する", func(t *testing.T) {
		srv, requests := newFakeCloudIdentityServer(t, http.StatusOK, []*cloudidentity.Membership{
			testMembership("alice@example.com", "OWNER"),
			testMembership("bob@example.com", "MEMBER"),
		})
		groups := &cloudIdentityGroups{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		got, err := groups.LookUp(context.Background(), token, groupEmail)
		if err != nil {
			t.Fatalf("LookUp() = %v", err)
		}
		if got.Email() != groupEmail {
			t.Errorf("Email() = %q, want %q", got.Email(), groupEmail)
		}
		members := got.Members()
		if len(members) != 2 ||
			members[0].Email().String() != "alice@example.com" || members[0].Role() != "OWNER" ||
			members[1].Email().String() != "bob@example.com" || members[1].Role() != "MEMBER" {
			t.Errorf("Members() = %v, want alice@example.com/OWNER and bob@example.com/MEMBER", members)
		}
		// It must look up the requested group, not some other address.
		if len(requests.groupKeys) != 1 || requests.groupKeys[0] != groupEmail.String() {
			t.Errorf("looked up groupKey.id = %q, want [%q]", requests.groupKeys, groupEmail)
		}
		// Every call must act as the user, not as the app.
		for _, a := range requests.auth {
			if a != "Bearer user-token" {
				t.Errorf("Authorization = %q, want %q", a, "Bearer user-token")
			}
		}
		if len(requests.auth) != 2 {
			t.Errorf("server got %d requests, want 2 (lookup and memberships)", len(requests.auth))
		}
	})

	t.Run("グループが見つからなければエラーを返す", func(t *testing.T) {
		srv, _ := newFakeCloudIdentityServer(t, http.StatusNotFound, nil)
		groups := &cloudIdentityGroups{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if _, err := groups.LookUp(context.Background(), token, groupEmail); err == nil {
			t.Error("LookUp(missing group) = nil error, want an error")
		}
	})

	t.Run("メンバーがメールアドレスでなければエラーを返す", func(t *testing.T) {
		srv, _ := newFakeCloudIdentityServer(t, http.StatusOK, []*cloudidentity.Membership{
			testMembership("not-an-email", "MEMBER"),
		})
		groups := &cloudIdentityGroups{opts: []option.ClientOption{option.WithEndpoint(srv.URL)}}

		if _, err := groups.LookUp(context.Background(), token, groupEmail); err == nil {
			t.Error("LookUp(member without an email) = nil error, want an error")
		}
	})
}
