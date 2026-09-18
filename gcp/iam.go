package gcp

import "context"

// Binding represents an IAM role binding.
type Binding struct {
	Role   string
	Member string
}

// IAMClient is a client for IAM policies.
type IAMClient struct{}

func NewIAMClient(ctx context.Context) (*IAMClient, error) {
	return nil, nil
}

func (c *IAMClient) GetPolicy(ctx context.Context, resource string) ([]Binding, error) {
	return nil, nil
}
