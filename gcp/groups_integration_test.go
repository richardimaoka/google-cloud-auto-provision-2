//go:build integration

package gcp

import (
	"context"
	"testing"
)

func TestGroupsClient_Get_Integration(t *testing.T) {
	ctx := context.Background()
	client, err := NewGroupsClient(ctx)
	if err != nil {
		t.Fatalf("NewGroupsClient: %v", err)
	}

	if _, err := client.Get(ctx, "test-group@example.com"); err != nil {
		t.Fatalf("Get: %v", err)
	}
}
