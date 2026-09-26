package webapp

import (
	"context"
	"errors"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// resourceManagerIAM is the iamClient backed by googlecloud.IAMClient.
type resourceManagerIAM struct {
	// opts are added to every client it builds. Tests set the endpoint here.
	opts []option.ClientOption
}

func (c *resourceManagerIAM) AddBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Principal, role domain.IAMRole) error {
	client, err := googlecloud.NewIAMClient(ctx, append(c.opts, option.WithTokenSource(token))...)
	if err != nil {
		return err
	}
	return client.AddBinding(ctx, projectID, role.String(), principal.String())
}

func (c *resourceManagerIAM) RemoveBinding(ctx context.Context, token oauth2.TokenSource, projectID string, principal domain.Principal, role domain.IAMRole) error {
	client, err := googlecloud.NewIAMClient(ctx, append(c.opts, option.WithTokenSource(token))...)
	if err != nil {
		return err
	}
	err = client.RemoveBinding(ctx, projectID, role.String(), principal.String())
	if errors.Is(err, googlecloud.ErrBindingNotFound) {
		return nil
	}
	return err
}
