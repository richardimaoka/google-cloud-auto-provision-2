package webapp

import (
	"context"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/googlecloud"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// resourceManagerProjects is the projectCreator backed by googlecloud.ProjectsClient.
type resourceManagerProjects struct {
	// opts are added to every client it builds. Tests set the endpoint here.
	opts []option.ClientOption
}

func (p *resourceManagerProjects) Create(ctx context.Context, token oauth2.TokenSource, request domain.ProjectCreationRequest) (string, error) {
	client, err := googlecloud.NewProjectsClient(ctx, append(p.opts, option.WithTokenSource(token))...)
	if err != nil {
		return "", err
	}
	project, err := client.Create(ctx, request.DisplayName)
	if err != nil {
		return "", err
	}
	return project.ID, nil
}
