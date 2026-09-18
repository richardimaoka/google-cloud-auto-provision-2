package gcp

import (
	"context"
	"fmt"
	"strings"

	cloudidentity "google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

// Group represents a Google Group.
type Group struct {
	Email   string
	Domain  string
	Members []Member
}

// Member represents a Google Group member.
type Member struct {
	Email  string
	Role   string
	Domain string
}

// GroupsClient is a client for Google Groups.
type GroupsClient struct {
	service *cloudidentity.Service
}

func NewGroupsClient(ctx context.Context, opts ...option.ClientOption) (*GroupsClient, error) {
	svc, err := cloudidentity.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new cloudidentity service: %w", err)
	}
	return &GroupsClient{service: svc}, nil
}

func (c *GroupsClient) Get(ctx context.Context, email string) (*Group, error) {
	lookup, err := c.service.Groups.Lookup().GroupKeyId(email).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("lookup group: %w", err)
	}

	var members []Member
	err = c.service.Groups.Memberships.List(lookup.Name).Context(ctx).Pages(ctx, func(page *cloudidentity.ListMembershipsResponse) error {
		for _, m := range page.Memberships {
			var role string
			if len(m.Roles) > 0 {
				role = m.Roles[0].Name
			}
			members = append(members, Member{
				Email:  m.PreferredMemberKey.Id,
				Role:   role,
				Domain: emailDomain(m.PreferredMemberKey.Id),
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	return &Group{
		Email:   email,
		Domain:  emailDomain(email),
		Members: members,
	}, nil
}

func emailDomain(email string) string {
	i := strings.LastIndex(email, "@")
	if i < 0 {
		return ""
	}
	return email[i+1:]
}

// SameDomain reports whether all of g's members have g's Domain.
func (g *Group) SameDomain() bool {
	for _, m := range g.Members {
		if m.Domain != g.Domain {
			return false
		}
	}
	return true
}
