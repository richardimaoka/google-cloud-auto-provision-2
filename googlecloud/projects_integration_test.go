//go:build integration

package googlecloud

// This test calls the real Resource Manager API and creates and deletes one
// real project. Each run uses up one project of the organization's project
// quota, and one project ID for good, because the ID of a deleted project can't
// be reused. So it asks for confirmation on the terminal first, and fails if
// there is no answer within 2 minutes. Pass -force to skip the question, for
// CI. Load the variables from .env.integration for one run, as described in
// groups_integration_test.go:
//
//	(set -a; . ./.env.integration; set +a; go test -tags integration -count=1 -v ./googlecloud -run Lifecycle)
//	(set -a; . ./.env.integration; set +a; go test -tags integration -count=1 -v ./googlecloud -run Lifecycle -force)
//
// -count=1 stops go test from replaying a cached result instead of running it
// again. -v shows the client's full log, which the test prints to stderr.
//
// It shares the environment of the other integration tests, see
// groups_integration_test.go. There is no way to run Get, Create or Delete on
// their own: the test only deletes the project it has just created, so it can't
// be pointed at a project that exists already.

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/option"
)

var force = flag.Bool("force", false,
	"run the project lifecycle integration test without asking for confirmation")

const (
	confirmTimeout = 2 * time.Minute
	confirmPrompt  = "WARNING: this creates and deletes a REAL Google Cloud project.\n" +
		"It uses up project quota, and the ID can never be reused.\n" +
		"Continue? [yes/no]: "
)

// confirmLifecycle asks on the terminal whether to go on. Anything but "yes"
// skips the test. No terminal, or no answer in time, fails it. It reads from
// /dev/tty because go test doesn't always connect stdin, and buffers output.
func confirmLifecycle(t *testing.T) {
	t.Helper()

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("cannot ask for confirmation, there is no terminal (%v). Pass -force to run without asking", err)
	}
	defer tty.Close()

	fmt.Fprint(tty, confirmPrompt)
	type reply struct {
		line string
		err  error
	}
	answer := make(chan reply, 1)
	go func() {
		line, err := bufio.NewReader(tty).ReadString('\n')
		answer <- reply{strings.TrimSpace(line), err}
	}()

	select {
	case r := <-answer:
		if r.err != nil && r.line == "" {
			t.Fatalf("read the answer: %v", r.err)
		}
		if !strings.EqualFold(r.line, "yes") {
			t.Skipf("not confirmed (answered %q)", r.line)
		}
	case <-time.After(confirmTimeout):
		t.Fatalf("no answer within %s", confirmTimeout)
	}
}

func TestProjectsClient_Lifecycle_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	quotaProject := requireEnv(t, "GOOGLE_TEST_QUOTA_PROJECT")
	client, err := NewProjectsClient(ctx, option.WithQuotaProject(quotaProject))
	if err != nil {
		t.Fatalf("NewProjectsClient: %v", err)
	}

	if !*force {
		confirmLifecycle(t)
	}

	// Log everything the client logs, down to each poll, to stderr. A run
	// against real resources can't be repeated with more logging after it fails,
	// so the full log has to be there the first time. It shows with -v.
	client.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// A valid project ID, and unique enough not to collide with earlier runs.
	name := fmt.Sprintf("it-lifecycle-%d", time.Now().Unix())

	t.Logf("creating project %q", name)
	created, err := client.Create(ctx, name)
	if err != nil {
		t.Fatalf("Create(%q) = %v, want nil error", name, err)
	}
	t.Logf("created project %q", created.ID)

	// Delete only the project Create returned, even if a later step fails.
	deleted := false
	t.Cleanup(func() {
		if deleted {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := client.Delete(ctx, created.ID); err != nil {
			fmt.Fprintf(os.Stderr, "could not clean up project %q, delete it by hand: %v\n", created.ID, err)
		}
	})

	if created.Name != name {
		t.Errorf("Create() Name = %q, want %q", created.Name, name)
	}
	if created.State != "ACTIVE" {
		t.Errorf("Create() State = %q, want ACTIVE", created.State)
	}

	got, err := client.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get(%q) after Create = %v, want nil error", created.ID, err)
	}
	if got.ID != created.ID || got.State != "ACTIVE" {
		t.Errorf("Get(%q) after Create = %+v, want that ID and state ACTIVE", created.ID, got)
	}

	if err := client.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete(%q) = %v, want nil error", created.ID, err)
	}
	deleted = true

	got, err = client.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get(%q) after Delete = %v, want nil error", created.ID, err)
	}
	if got.State != "DELETE_REQUESTED" {
		t.Errorf("Get(%q) after Delete State = %q, want DELETE_REQUESTED", created.ID, got.State)
	}
}
