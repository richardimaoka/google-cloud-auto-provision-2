package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Organization represents the Google Cloud organization this application is set up for.
type Organization struct {
	id     string
	domain string
}

// NewOrganization returns an Organization, or an error if id is not a numeric ID
// (without the "organizations/" prefix) or domain is empty.
func NewOrganization(id, domain string) (Organization, error) {
	if id == "" || strings.Trim(id, "0123456789") != "" {
		return Organization{}, fmt.Errorf("invalid organization ID %q: must be numeric", id)
	}
	if domain == "" {
		return Organization{}, errors.New("organization domain is empty")
	}
	return Organization{id: id, domain: domain}, nil
}

// ID returns the numeric ID, without the "organizations/" prefix.
func (o Organization) ID() string { return o.id }

// Domain returns the domain of the organization, such as "example.com".
func (o Organization) Domain() string { return o.domain }
