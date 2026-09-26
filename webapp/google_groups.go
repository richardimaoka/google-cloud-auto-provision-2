package webapp

import (
	"context"
	"fmt"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// cloudIdentityGroups is the googleGroups backed by googlecloud.GroupsClient.
type cloudIdentityGroups struct {
	// opts are added to every client it builds. Tests set the endpoint here.
	opts []option.ClientOption
}

func (g *cloudIdentityGroups) LookUp(ctx context.Context, token oauth2.TokenSource, email domain.Email) (domain.GoogleGroup, error) {
	client, err := googlecloud.NewGroupsClient(ctx, append(g.opts, option.WithTokenSource(token))...)
	if err != nil {
		return domain.GoogleGroup{}, err
	}
	group, err := client.Get(ctx, email.String())
	if err != nil {
		return domain.GoogleGroup{}, err
	}
	members := make([]domain.GoogleGroupMember, 0, len(group.Members))
	for _, m := range group.Members {
		memberEmail, err := domain.NewEmail(m.Email)
		if err != nil {
			return domain.GoogleGroup{}, fmt.Errorf("member of %s: %w", email, err)
		}
		members = append(members, domain.NewGoogleGroupMember(memberEmail, m.Role))
	}
	return domain.NewGoogleGroup(email, members), nil
}
