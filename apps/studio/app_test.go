package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
)

func TestParseAppsListOutputAcceptsEmptyEnvelope(t *testing.T) {
	rawApps, err := parseAppsListOutput([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("parseAppsListOutput() error = %v", err)
	}
	if len(rawApps) != 0 {
		t.Fatalf("len(rawApps) = %d, want 0", len(rawApps))
	}
}

func TestParseAvailabilityViewOutputReturnsResourceID(t *testing.T) {
	availabilityID, available, err := parseAvailabilityViewOutput([]byte(`{"data":{"id":"availability-123","attributes":{"availableInNewTerritories":true}}}`))
	if err != nil {
		t.Fatalf("parseAvailabilityViewOutput() error = %v", err)
	}
	if availabilityID != "availability-123" {
		t.Fatalf("availabilityID = %q, want availability-123", availabilityID)
	}
	if !available {
		t.Fatal("available = false, want true")
	}
}

func TestBundledASCPathPrefersAppBundleResources(t *testing.T) {
	tmp := t.TempDir()
	resourceDir := filepath.Join(tmp, "ASC Studio.app", "Contents", "Resources", "bin")
	if err := os.MkdirAll(resourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundled := filepath.Join(resourceDir, "asc")
	if err := os.WriteFile(bundled, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalExecutable := osExecutableFunc
	originalGetwd := getwdFunc
	t.Cleanup(func() {
		osExecutableFunc = originalExecutable
		getwdFunc = originalGetwd
	})

	osExecutableFunc = func() (string, error) {
		return filepath.Join(tmp, "ASC Studio.app", "Contents", "MacOS", "ASC Studio"), nil
	}
	getwdFunc = func() (string, error) {
		return filepath.Join(tmp, "workspace"), nil
	}

	app := &App{}
	if got := app.bundledASCPath(); got != bundled {
		t.Fatalf("bundledASCPath() = %q, want %q", got, bundled)
	}
}

func TestEnsureSessionSingleFlightsConcurrentCalls(t *testing.T) {
	tmp := t.TempDir()
	settingsStore := settings.NewStore(tmp)
	if err := settingsStore.Save(settings.StudioSettings{
		AgentCommand:     "fake-agent",
		WorkspaceRoot:    "/tmp/workspace",
		PreferBundledASC: true,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var startCalls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	app := &App{
		rootDir:      tmp,
		settings:     settingsStore,
		threads:      threads.NewStore(tmp),
		approvals:    approvals.NewQueue(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]chan struct{}),
		startAgent: func(context.Context, acp.LaunchSpec) (agentClient, error) {
			startCalls.Add(1)
			return &fakeAgentClient{
				bootstrapFn: func(context.Context, acp.SessionConfig) (string, error) {
					select {
					case started <- struct{}{}:
					default:
					}
					<-release
					return "session-1", nil
				},
			}, nil
		},
	}

	thread := threads.Thread{ID: "thread-1"}
	results := make(chan *threadSession, 2)
	errorsCh := make(chan error, 2)

	go func() {
		session, err := app.ensureSession(thread)
		errorsCh <- err
		results <- session
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first agent bootstrap")
	}

	go func() {
		session, err := app.ensureSession(thread)
		errorsCh <- err
		results <- session
	}()

	close(release)

	var sessions []*threadSession
	for range 2 {
		select {
		case err := <-errorsCh:
			if err != nil {
				t.Fatalf("ensureSession() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ensureSession result")
		}
		select {
		case session := <-results:
			sessions = append(sessions, session)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for session")
		}
	}

	if got := startCalls.Load(); got != 1 {
		t.Fatalf("startCalls = %d, want 1", got)
	}
	if len(sessions) != 2 || sessions[0] != sessions[1] {
		t.Fatal("ensureSession() did not reuse the same session for concurrent callers")
	}
}

type fakeAgentClient struct {
	bootstrapFn func(context.Context, acp.SessionConfig) (string, error)
	promptFn    func(context.Context, string, string) (acp.PromptResult, []acp.Event, error)
	closeFn     func() error
}

func (f *fakeAgentClient) Bootstrap(ctx context.Context, cfg acp.SessionConfig) (string, error) {
	if f.bootstrapFn != nil {
		return f.bootstrapFn(ctx, cfg)
	}
	return "session-1", nil
}

func (f *fakeAgentClient) Prompt(ctx context.Context, sessionID string, prompt string) (acp.PromptResult, []acp.Event, error) {
	if f.promptFn != nil {
		return f.promptFn(ctx, sessionID, prompt)
	}
	return acp.PromptResult{Status: "completed"}, nil, nil
}

func (f *fakeAgentClient) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}
