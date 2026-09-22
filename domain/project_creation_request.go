package domain

import (
	"errors"
	"fmt"
)

type ProjectCreationRequest struct {
	DisplayName    string
	Requester      Email
	DeveloperGroup GoogleGroup
}

// NewProjectCreationRequest returns an error if displayName is invalid, or if
// requester or developerGroup's email is the zero Email.
func NewProjectCreationRequest(displayName string, requester Email, developerGroup GoogleGroup) (ProjectCreationRequest, error) {
	if !validProjectDisplayName(displayName) {
		return ProjectCreationRequest{}, fmt.Errorf("invalid project display name %q", displayName)
	}
	if requester == (Email{}) {
		return ProjectCreationRequest{}, errors.New("requester is empty")
	}
	if developerGroup.email == (Email{}) {
		return ProjectCreationRequest{}, errors.New("developer group email is empty")
	}
	return ProjectCreationRequest{
		DisplayName:    displayName,
		Requester:      requester,
		DeveloperGroup: developerGroup,
	}, nil
}

// validProjectDisplayName reports whether name is a valid Google Cloud project
// display name: 4 to 30 permitted ASCII characters, ending in a letter or digit.
func validProjectDisplayName(name string) bool {
	if len(name) < 4 || len(name) > 30 {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '\'' || c == '"' || c == ' ' || c == '!') {
			return false
		}
	}
	last := name[len(name)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9')
}
