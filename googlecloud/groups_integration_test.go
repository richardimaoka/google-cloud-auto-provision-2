//go:build integration

package googlecloud

// These tests call the real Cloud Identity Groups API and only read from it.
// Run them with:
//
//	go test -tags integration ./googlecloud
//
// Put the variables below in .env.integration and load it for one run with a
// subshell, so the variables are gone from your shell afterwards:
//
//	(set -a; . ./.env.integration; set +a; go test -tags integration ./googlecloud)
//
// Credentials come from Application Default Credentials. The tests refuse to
// run unless the project recorded in ADC matches GOOGLECLOUD_TEST_QUOTA_PROJECT, so a
// stale ADC project can't be used by accident. Each test also fails when the
// environment variable naming its fixture is unset:
//
//	GOOGLECLOUD_TEST_QUOTA_PROJECT      quota project; must match the ADC project
//	GOOGLECLOUD_TEST_DOMAIN             domain that owns the fixture groups
//	GOOGLECLOUD_TEST_GROUP_EMPTY        group with no members
//	GOOGLECLOUD_TEST_GROUP_MIXED_ROLES  group with at least one OWNER and one MEMBER
//	GOOGLECLOUD_TEST_GROUP_LARGE        group with more members than one page (> 200)
//	GOOGLECLOUD_TEST_GROUP_SAME_DOMAIN  group whose members are all in its own domain
//	GOOGLECLOUD_TEST_GROUP_EXTERNAL     group with a member outside its own domain

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2/google"
	cloudidentity "google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

func TestMain(m *testing.M) {
	if err := checkADCProject(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "integration test setup: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// checkADCProject fails unless the project ADC points at is the one named by
// GOOGLECLOUD_TEST_QUOTA_PROJECT. User credentials record it as quota_project_id and
// service account keys as project_id.
func checkADCProject(ctx context.Context) error {
	want := os.Getenv("GOOGLECLOUD_TEST_QUOTA_PROJECT")
	if want == "" {
		return errors.New("GOOGLECLOUD_TEST_QUOTA_PROJECT is not set")
	}

	creds, err := google.FindDefaultCredentials(ctx, cloudidentity.CloudPlatformScope)
	if err != nil {
		return fmt.Errorf("find default credentials: %w", err)
	}

	var adc struct {
		QuotaProjectID string `json:"quota_project_id"`
		ProjectID      string `json:"project_id"`
	}
	if len(creds.JSON) > 0 {
		if err := json.Unmarshal(creds.JSON, &adc); err != nil {
			return fmt.Errorf("parse default credentials: %w", err)
		}
	}

	got := cmp.Or(adc.QuotaProjectID, adc.ProjectID, creds.ProjectID)
	if got != want {
		return fmt.Errorf("ADC project is %q but GOOGLECLOUD_TEST_QUOTA_PROJECT is %q; "+
			"run `gcloud auth application-default set-quota-project %s` or fix the env file", got, want, want)
	}
	return nil
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("%s is not set", key)
	}
	return v
}

func newIntegrationClient(t *testing.T) *GroupsClient {
	t.Helper()

	quotaProject := requireEnv(t, "GOOGLECLOUD_TEST_QUOTA_PROJECT")
	client, err := NewGroupsClient(context.Background(), option.WithQuotaProject(quotaProject))
	if err != nil {
		t.Fatalf("NewGroupsClient: %v", err)
	}
	return client
}

func TestGroupsClient_Get_Integration(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		check func(t *testing.T, g *Group)
	}{
		{
			name: "メンバー0人のグループ",
			env:  "GOOGLECLOUD_TEST_GROUP_EMPTY",
			check: func(t *testing.T, g *Group) {
				if len(g.Members) != 0 {
					t.Errorf("Members = %+v, want none", g.Members)
				}
			},
		},
		{
			name: "メンバーが複数かつロールが混在するグループ",
			env:  "GOOGLECLOUD_TEST_GROUP_MIXED_ROLES",
			check: func(t *testing.T, g *Group) {
				roles := map[string]bool{}
				for _, m := range g.Members {
					roles[m.Role] = true
				}
				for _, want := range []string{"OWNER", "MEMBER"} {
					if !roles[want] {
						t.Errorf("no member with role %s in %+v", want, g.Members)
					}
				}
			},
		},
		{
			name: "メンバーが複数ページにまたがるグループ",
			env:  "GOOGLECLOUD_TEST_GROUP_LARGE",
			check: func(t *testing.T, g *Group) {
				// The API returns at most 200 memberships per page, so more
				// than that proves every page was fetched.
				if len(g.Members) <= 200 {
					t.Errorf("len(Members) = %d, want > 200", len(g.Members))
				}
				seen := map[string]bool{}
				for _, m := range g.Members {
					if seen[m.Email] {
						t.Errorf("duplicate member %s", m.Email)
					}
					seen[m.Email] = true
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			email := requireEnv(t, c.env)
			client := newIntegrationClient(t)

			got, err := client.Get(context.Background(), email)
			if err != nil {
				t.Fatalf("Get(%q) = %v, want nil error", email, err)
			}

			if got.Email != email {
				t.Errorf("Email = %q, want %q", got.Email, email)
			}
			if want := emailDomain(email); got.Domain != want {
				t.Errorf("Domain = %q, want %q", got.Domain, want)
			}
			for _, m := range got.Members {
				if m.Email == "" || m.Role == "" {
					t.Errorf("member %+v has empty Email or Role", m)
				}
				if want := emailDomain(m.Email); m.Domain != want {
					t.Errorf("member %s Domain = %q, want %q", m.Email, m.Domain, want)
				}
			}
			c.check(t, got)
		})
	}
}

func TestGroupsClient_Get_NotFound_Integration(t *testing.T) {
	domain := requireEnv(t, "GOOGLECLOUD_TEST_DOMAIN")
	client := newIntegrationClient(t)

	email := fmt.Sprintf("does-not-exist-%d@%s", time.Now().UnixNano(), domain)
	if _, err := client.Get(context.Background(), email); err == nil {
		t.Fatalf("Get(%q) = nil error, want error", email)
	}
}

func TestGroupsClient_Get_Unauthenticated_Integration(t *testing.T) {
	email := requireEnv(t, "GOOGLECLOUD_TEST_GROUP_EMPTY")

	client, err := NewGroupsClient(context.Background(), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("NewGroupsClient: %v", err)
	}
	if _, err := client.Get(context.Background(), email); err == nil {
		t.Fatalf("Get(%q) = nil error, want error", email)
	}
}

func TestGroup_SameDomain_Integration(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want bool
	}{
		{name: "全メンバーのドメインがグループのドメインと一致", env: "GOOGLECLOUD_TEST_GROUP_SAME_DOMAIN", want: true},
		{name: "メンバーのドメインがグループのドメインと異なる", env: "GOOGLECLOUD_TEST_GROUP_EXTERNAL", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			email := requireEnv(t, c.env)
			client := newIntegrationClient(t)

			g, err := client.Get(context.Background(), email)
			if err != nil {
				t.Fatalf("Get(%q) = %v, want nil error", email, err)
			}
			if got := g.SameDomain(); got != c.want {
				t.Errorf("SameDomain() = %v, want %v (members: %+v)", got, c.want, g.Members)
			}
		})
	}
}
