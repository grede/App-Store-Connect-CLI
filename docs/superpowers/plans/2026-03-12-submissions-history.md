# Submissions History Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `asc review submissions-history` command that shows a chronological timeline of all review submissions for an app, enriched with version strings and item-level outcomes.

**Architecture:** CLI-layer aggregation over existing client methods. The command fetches review submissions, then enriches each with item states and version strings via sequential API calls. A pure `deriveOutcome()` function computes human-readable outcomes from item/submission states. Uses `PrintOutputWithRenderers` for table/markdown output with custom renderers.

**Tech Stack:** Go 1.26, ffcli, existing `internal/asc` client methods

**Spec:** `docs/superpowers/specs/2026-03-12-submissions-history-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/cli/reviews/submissions_history.go` | Command definition, flag parsing, aggregation logic, `deriveOutcome()`, table/markdown renderers |
| `internal/cli/reviews/submissions_history_test.go` | Unit tests for `deriveOutcome()` + integration tests with mock HTTP |
| `internal/cli/reviews/review.go` | Add `SubmissionsHistoryCommand()` to subcommands list (1-line change) |

No new files in `internal/asc/`. Reuses:
- `client.GetReviewSubmissions(ctx, appID, opts ...asc.ReviewSubmissionsOption)`
- `client.GetReviewSubmissionItems(ctx, submissionID, opts ...asc.ReviewSubmissionItemsOption)`
- `client.GetAppStoreVersion(ctx, versionID, opts ...asc.AppStoreVersionOption)`

---

## Chunk 1: Core Logic + Unit Tests

### Task 1: Outcome Derivation — Tests First

**Files:**
- Create: `internal/cli/reviews/submissions_history_test.go`
- Create: `internal/cli/reviews/submissions_history.go`

- [ ] **Step 1: Write failing tests for `deriveOutcome()`**

Create `internal/cli/reviews/submissions_history_test.go`:

```go
package reviews

import "testing"

func TestDeriveOutcome(t *testing.T) {
	tests := []struct {
		name            string
		submissionState string
		itemStates      []string
		want            string
	}{
		{
			name:            "all items approved",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED"},
			want:            "approved",
		},
		{
			name:            "any item rejected",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "REJECTED"},
			want:            "rejected",
		},
		{
			name:            "unresolved issues no rejected items",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"ACCEPTED"},
			want:            "rejected",
		},
		{
			name:            "rejected item takes priority over unresolved",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"REJECTED"},
			want:            "rejected",
		},
		{
			name:            "mixed non-rejected states falls through to submission state",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "ACCEPTED"},
			want:            "complete",
		},
		{
			name:            "no items uses submission state",
			submissionState: "WAITING_FOR_REVIEW",
			itemStates:      nil,
			want:            "waiting_for_review",
		},
		{
			name:            "in review state",
			submissionState: "IN_REVIEW",
			itemStates:      []string{"READY_FOR_REVIEW"},
			want:            "in_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveOutcome(tt.submissionState, tt.itemStates)
			if got != tt.want {
				t.Errorf("deriveOutcome(%q, %v) = %q, want %q", tt.submissionState, tt.itemStates, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestDeriveOutcome -v`
Expected: FAIL — `deriveOutcome` undefined

- [ ] **Step 3: Implement `deriveOutcome()` and types**

Create `internal/cli/reviews/submissions_history.go`:

```go
package reviews

import (
	"strings"
)

// SubmissionHistoryEntry is the assembled result for one submission.
type SubmissionHistoryEntry struct {
	SubmissionID  string                 `json:"submissionId"`
	VersionString string                 `json:"versionString"`
	Platform      string                 `json:"platform"`
	State         string                 `json:"state"`
	SubmittedDate string                 `json:"submittedDate"`
	Outcome       string                 `json:"outcome"`
	Items         []SubmissionHistoryItem `json:"items"`
}

// SubmissionHistoryItem is a summary of one item in a submission.
type SubmissionHistoryItem struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Type       string `json:"type"`
	ResourceID string `json:"resourceId"`
}

// deriveOutcome computes a human-readable outcome from submission and item states.
// Priority order:
// 1. Any item REJECTED → "rejected"
// 2. All items APPROVED → "approved"
// 3. Submission state UNRESOLVED_ISSUES → "rejected"
// 4. Fallback → lowercase submission state
func deriveOutcome(submissionState string, itemStates []string) string {
	hasRejected := false
	allApproved := len(itemStates) > 0

	for _, s := range itemStates {
		if s == "REJECTED" {
			hasRejected = true
		}
		if s != "APPROVED" {
			allApproved = false
		}
	}

	if hasRejected {
		return "rejected"
	}
	if allApproved {
		return "approved"
	}
	if submissionState == "UNRESOLVED_ISSUES" {
		return "rejected"
	}
	return strings.ToLower(submissionState)
}

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestDeriveOutcome -v`
Expected: PASS — all 7 cases green

- [ ] **Step 5: Commit**

```bash
cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI
git add internal/cli/reviews/submissions_history.go internal/cli/reviews/submissions_history_test.go
git commit -m "feat(review): add deriveOutcome logic for submissions history

Pure function that computes human-readable outcome from submission
and item states. Priority: rejected items > all approved > unresolved
issues > fallback to lowercase state."
```

---

### Task 2: Command Definition + Flag Parsing

**Files:**
- Modify: `internal/cli/reviews/submissions_history.go`
- Modify: `internal/cli/reviews/review.go:37-58`

- [ ] **Step 1: Write failing test for missing --app flag**

Add to `internal/cli/reviews/submissions_history_test.go`:

```go
func TestSubmissionsHistoryCommand_MissingApp(t *testing.T) {
	cmd := SubmissionsHistoryCommand()
	if cmd.Name != "submissions-history" {
		t.Fatalf("unexpected command name: %s", cmd.Name)
	}

	// Unset any env that could provide app ID
	t.Setenv("ASC_APP_ID", "")

	err := cmd.ParseAndRun(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for missing --app, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got: %v", err)
	}
}
```

Add the necessary imports at the top of the test file:

```go
import (
	"context"
	"errors"
	"flag"
	"testing"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestSubmissionsHistoryCommand_MissingApp -v`
Expected: FAIL — `SubmissionsHistoryCommand` undefined

- [ ] **Step 3: Implement command definition**

Add to `internal/cli/reviews/submissions_history.go` (add imports, keep existing types and `deriveOutcome`):

```go
package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubmissionsHistoryCommand returns the submissions-history subcommand.
func SubmissionsHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-history", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	state := fs.String("state", "", "Filter by submission state (comma-separated)")
	version := fs.String("version", "", "Filter by version string (client-side)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-history",
		ShortUsage: "asc review submissions-history [flags]",
		ShortHelp:  "Show submission history timeline for an app.",
		LongHelp: `Show a chronological timeline of all review submissions for an app,
enriched with version strings and item-level outcomes.

Note: The App Store Connect API does not expose rejection reason text.
This command shows submission states and computed outcomes, not reasons.

Examples:
  asc review submissions-history --app "123456789"
  asc review submissions-history --app "123456789" --platform TV_OS
  asc review submissions-history --app "123456789" --paginate
  asc review submissions-history --app "123456789" --version "3.1.1"
  asc review submissions-history --app "123456789" --state COMPLETE,UNRESOLVED_ISSUES`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-history: --limit must be between 1 and 200")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}
			states := shared.SplitCSVUpper(*state)

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Fetch + enrich + render (placeholder stubs for now)
			entries, err := enrichSubmissions(requestCtx, client, nil, strings.TrimSpace(*version))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				entries,
				*output.Output,
				*output.Pretty,
				func() error { return printHistoryTable(entries) },
				func() error { return printHistoryMarkdown(entries) },
			)
		},
	}
}
```

Also add placeholder stubs so it compiles:

```go
func enrichSubmissions(_ context.Context, _ *asc.Client, _ []asc.ReviewSubmissionResource, _ string) ([]SubmissionHistoryEntry, error) {
	return nil, nil
}

func printHistoryTable(_ []SubmissionHistoryEntry) error {
	return nil
}

func printHistoryMarkdown(_ []SubmissionHistoryEntry) error {
	return nil
}
```

- [ ] **Step 4: Wire into review.go subcommands**

In `internal/cli/reviews/review.go`, add `SubmissionsHistoryCommand()` to the subcommands list. Find the line with `ReviewSubmissionsListCommand()` and add the new command right after it:

```go
		Subcommands: []*ffcli.Command{
			ReviewDetailsGetCommand(),
			ReviewDetailsForVersionCommand(),
			ReviewDetailsCreateCommand(),
			ReviewDetailsUpdateCommand(),
			ReviewDetailsAttachmentsListCommand(),
			ReviewDetailsAttachmentsGetCommand(),
			ReviewDetailsAttachmentsUploadCommand(),
			ReviewDetailsAttachmentsDeleteCommand(),
			SubmissionsHistoryCommand(),           // ← ADD THIS LINE
			ReviewSubmissionsListCommand(),
			ReviewSubmissionsGetCommand(),
			ReviewSubmissionsCreateCommand(),
			ReviewSubmissionsSubmitCommand(),
			ReviewSubmissionsCancelCommand(),
			ReviewSubmissionsUpdateCommand(),
			ReviewSubmissionsItemsIDsCommand(),
			ReviewItemsGetCommand(),
			ReviewItemsListCommand(),
			ReviewItemsAddCommand(),
			ReviewItemsUpdateCommand(),
			ReviewItemsRemoveCommand(),
		},
```

Also add to the `LongHelp` examples:

```
  asc review submissions-history --app "123456789"
```

- [ ] **Step 5: Run tests to verify flag validation**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestSubmissionsHistoryCommand -v`
Expected: PASS

- [ ] **Step 6: Add invalid limit test**

Add to `internal/cli/reviews/submissions_history_test.go`:

```go
func TestSubmissionsHistoryCommand_InvalidLimit(t *testing.T) {
	cmd := SubmissionsHistoryCommand()
	t.Setenv("ASC_APP_ID", "test-app")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	err := cmd.ParseAndRun(context.Background(), []string{"--limit", "999"})
	if err == nil {
		t.Fatal("expected error for invalid limit, got nil")
	}
	if !strings.Contains(err.Error(), "--limit must be between 1 and 200") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

Add `"strings"` to the test imports.

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestSubmissionsHistoryCommand_InvalidLimit -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI
git add internal/cli/reviews/submissions_history.go internal/cli/reviews/submissions_history_test.go internal/cli/reviews/review.go
git commit -m "feat(review): add submissions-history command skeleton

Registers asc review submissions-history with flag parsing, validation,
and placeholder aggregation. Wired into review.go subcommands."
```

---

## Chunk 2: Aggregation Logic + Integration Tests

### Task 3: Implement `enrichSubmissions()` Aggregation + Full `Exec` Body

**Files:**
- Modify: `internal/cli/reviews/submissions_history.go` (replace placeholder `enrichSubmissions`, update `Exec` body with pagination + enrichment)
- Modify: `internal/cli/reviews/submissions_history_test.go`

- [ ] **Step 1: Write failing integration tests with mock HTTP**

Add to `internal/cli/reviews/submissions_history_test.go`. Consolidate all imports at the top of the file:

```go
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)
```

Then add test helpers and test functions:

```go
// testRoundTripper implements http.RoundTripper for testing.
type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestHistoryClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("write key error: %v", err)
	}

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient error: %v", err)
	}
	return client
}

// makeSubmissions builds []asc.ReviewSubmissionResource from test data.
func makeSubmissions(entries ...struct {
	id, platform, state, date string
}) []asc.ReviewSubmissionResource {
	var subs []asc.ReviewSubmissionResource
	for _, e := range entries {
		subs = append(subs, asc.ReviewSubmissionResource{
			ID: e.id,
			Attributes: asc.ReviewSubmissionAttributes{
				Platform:        asc.Platform(e.platform),
				SubmissionState: asc.ReviewSubmissionState(e.state),
				SubmittedDate:   e.date,
			},
		})
	}
	return subs
}

func TestEnrichSubmissions_HappyPath(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-2",
					"attributes": {"state": "REJECTED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-1":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-1", "attributes": {"versionString": "3.1.1", "platform": "TV_OS"}},
				"links": {"self": "/v1/appStoreVersions/ver-1"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-2":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-2", "attributes": {"versionString": "3.0.0", "platform": "TV_OS"}},
				"links": {"self": "/v1/appStoreVersions/ver-2"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "TV_OS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "TV_OS", "UNRESOLVED_ISSUES", "2026-02-15T10:00:00Z"},
	)

	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Sorted by submittedDate descending
	if entries[0].VersionString != "3.1.1" {
		t.Errorf("first entry version = %q, want %q", entries[0].VersionString, "3.1.1")
	}
	if entries[0].Outcome != "approved" {
		t.Errorf("first entry outcome = %q, want %q", entries[0].Outcome, "approved")
	}
	if entries[1].VersionString != "3.0.0" {
		t.Errorf("second entry version = %q, want %q", entries[1].VersionString, "3.0.0")
	}
	if entries[1].Outcome != "rejected" {
		t.Errorf("second entry outcome = %q, want %q", entries[1].Outcome, "rejected")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("first entry items count = %d, want 1", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_EmptySubmissions(t *testing.T) {
	client := newTestHistoryClient(t, testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no API calls expected for empty submissions")
		return nil, nil
	}))
	entries, err := enrichSubmissions(context.Background(), client, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestEnrichSubmissions_Version404(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-gone"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-gone":
			return testJSONResponse(404, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
}

func TestEnrichSubmissions_VersionFilter(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-1", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-2", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-1":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-1", "attributes": {"versionString": "2.0.0", "platform": "IOS"}},
				"links": {"self": "/v1/appStoreVersions/ver-1"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-2":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-2", "attributes": {"versionString": "1.0.0", "platform": "IOS"}},
				"links": {"self": "/v1/appStoreVersions/ver-2"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "IOS", "COMPLETE", "2026-02-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after version filter, got %d", len(entries))
	}
	if entries[0].VersionString != "2.0.0" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "2.0.0")
	}
}

func TestEnrichSubmissions_NoItems(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if entries[0].Outcome != "complete" {
		t.Errorf("outcome = %q, want %q", entries[0].Outcome, "complete")
	}
	if len(entries[0].Items) != 0 {
		t.Errorf("items count = %d, want 0", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_ItemWithoutVersionRelationship(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("items count = %d, want 1", len(entries[0].Items))
	}
	if entries[0].Items[0].ResourceID != "" {
		t.Errorf("item resourceId = %q, want empty", entries[0].Items[0].ResourceID)
	}
}

func TestEnrichSubmissions_SkipsEmptySubmittedDate(t *testing.T) {
	// Should not make any API calls for the draft submission
	calls := 0
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-draft", "IOS", "READY_FOR_REVIEW", ""},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (draft skipped), got %d", len(entries))
	}
	if calls != 0 {
		t.Errorf("expected 0 API calls for draft submissions, got %d", calls)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestEnrichSubmissions -v`
Expected: FAIL — `enrichSubmissions` returns nil (placeholder)

- [ ] **Step 3: Implement `enrichSubmissions()` and update `Exec` body**

Replace the placeholder in `internal/cli/reviews/submissions_history.go`.

**Architecture note:** Pagination is handled at the command level (in `Exec`) using the established `shared.PaginateWithSpinner` pattern. The `enrichSubmissions()` function takes already-fetched submissions and enriches them. This avoids calling a non-existent `shared.PaginateAll` and follows the exact pattern used by `submissions-list`.

First, update the `Exec` function to handle pagination before calling enrichment:

```go
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-history: --limit must be between 1 and 200")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}
			states := shared.SplitCSVUpper(*state)

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionsOption{
				asc.WithReviewSubmissionsLimit(*limit),
				asc.WithReviewSubmissionsPlatforms(platforms),
				asc.WithReviewSubmissionsStates(states),
			}

			// Fetch submissions (with or without pagination)
			var submissions []asc.ReviewSubmissionResource
			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionsLimit(200))
				resp, pErr := shared.PaginateWithSpinner(requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, asc.WithReviewSubmissionsNextURL(nextURL))
					},
				)
				if pErr != nil {
					return fmt.Errorf("review submissions-history: %w", pErr)
				}
				if aggResp, ok := resp.(*asc.ReviewSubmissionsResponse); ok {
					submissions = aggResp.Data
				}
			} else {
				resp, fErr := client.GetReviewSubmissions(requestCtx, resolvedAppID, opts...)
				if fErr != nil {
					return fmt.Errorf("review submissions-history: %w", fErr)
				}
				submissions = resp.Data
			}

			// Enrich with items + version strings
			entries, err := enrichSubmissions(requestCtx, client, submissions, strings.TrimSpace(*version))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				entries,
				*output.Output,
				*output.Pretty,
				func() error { return printHistoryTable(entries) },
				func() error { return printHistoryMarkdown(entries) },
			)
		},
```

Then implement `enrichSubmissions()` (renamed from `fetchSubmissionHistory`):

```go
// enrichSubmissions takes already-fetched submissions and enriches each with
// item states and version strings. Applies client-side version filtering and
// sorts by submittedDate descending.
func enrichSubmissions(ctx context.Context, client *asc.Client, submissions []asc.ReviewSubmissionResource, versionFilter string) ([]SubmissionHistoryEntry, error) {
	var entries []SubmissionHistoryEntry
	for _, sub := range submissions {
		// Skip pre-submission drafts (no submittedDate)
		if strings.TrimSpace(sub.Attributes.SubmittedDate) == "" {
			continue
		}

		entry := SubmissionHistoryEntry{
			SubmissionID:  sub.ID,
			Platform:      string(sub.Attributes.Platform),
			State:         string(sub.Attributes.SubmissionState),
			SubmittedDate: sub.Attributes.SubmittedDate,
		}

		// Fetch items for this submission
		itemsResp, err := client.GetReviewSubmissionItems(ctx, sub.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch items for submission %s: %w", sub.ID, err)
		}

		var itemStates []string
		for _, item := range itemsResp.Data {
			histItem := SubmissionHistoryItem{
				ID:    item.ID,
				State: item.Attributes.State,
			}

			// Extract version relationship if present
			if item.Relationships != nil && item.Relationships.AppStoreVersion != nil {
				histItem.Type = "appStoreVersion"
				histItem.ResourceID = item.Relationships.AppStoreVersion.Data.ID

				// Fetch version string
				if histItem.ResourceID != "" {
					verResp, verErr := client.GetAppStoreVersion(ctx, histItem.ResourceID)
					if verErr != nil {
						if asc.IsNotFound(verErr) {
							entry.VersionString = "unknown"
						} else {
							return nil, fmt.Errorf("failed to fetch version %s: %w", histItem.ResourceID, verErr)
						}
					} else if entry.VersionString == "" {
						entry.VersionString = verResp.Data.Attributes.VersionString
					}
				}
			}

			itemStates = append(itemStates, item.Attributes.State)
			entry.Items = append(entry.Items, histItem)
		}

		entry.Outcome = deriveOutcome(entry.State, itemStates)

		if entry.VersionString == "" {
			entry.VersionString = "unknown"
		}

		entries = append(entries, entry)
	}

	// Client-side version filter
	if versionFilter != "" {
		var filtered []SubmissionHistoryEntry
		for _, e := range entries {
			if e.VersionString == versionFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by submittedDate descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SubmittedDate > entries[j].SubmittedDate
	})

	return entries, nil
}
```

Add `"sort"` to the import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestEnrichSubmissions -v`
Expected: PASS — all 7 integration tests green

- [ ] **Step 5: Run all reviews tests to check for regressions**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -v`
Expected: PASS — no regressions

- [ ] **Step 6: Commit**

```bash
cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI
git add internal/cli/reviews/submissions_history.go internal/cli/reviews/submissions_history_test.go
git commit -m "feat(review): implement submissions history aggregation

Fetches review submissions, enriches with item states and version
strings via sequential API calls. Handles version 404 gracefully,
skips pre-submission drafts, supports client-side version filtering."
```

---

### Task 4: Table and Markdown Renderers

**Files:**
- Modify: `internal/cli/reviews/submissions_history.go` (replace placeholder renderers)

- [ ] **Step 1: Write failing test for table output**

Add to `internal/cli/reviews/submissions_history_test.go`:

```go
func TestPrintHistoryTable_NoError(t *testing.T) {
	entries := []SubmissionHistoryEntry{
		{
			SubmissionID:  "sub-1",
			VersionString: "3.1.1",
			Platform:      "TV_OS",
			State:         "COMPLETE",
			SubmittedDate: "2026-03-01T12:00:00Z",
			Outcome:       "approved",
			Items:         []SubmissionHistoryItem{{ID: "i1", State: "APPROVED", Type: "appStoreVersion", ResourceID: "v1"}},
		},
	}
	// Should not panic or error
	err := printHistoryTable(entries)
	if err != nil {
		t.Fatalf("printHistoryTable error: %v", err)
	}
}
```

- [ ] **Step 2: Implement renderers**

Replace the placeholder functions in `internal/cli/reviews/submissions_history.go`:

```go
func printHistoryTable(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printHistoryMarkdown(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func formatItemsSummary(items []SubmissionHistoryItem) string {
	if len(items) == 0 {
		return "0 items"
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[strings.ToLower(item.State)]++
	}
	var parts []string
	for state, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, state))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 go test ./internal/cli/reviews/ -run TestPrintHistoryTable -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI
git add internal/cli/reviews/submissions_history.go internal/cli/reviews/submissions_history_test.go
git commit -m "feat(review): add table and markdown renderers for submissions history

Custom renderers using asc.RenderTable/RenderMarkdown with item
summary formatting (e.g., '1 approved', '1 rejected')."
```

---

## Chunk 3: Final Verification

### Task 5: Format, Lint, Full Test Suite

**Files:** None modified — verification only.

- [ ] **Step 1: Run gofumpt**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ~/go/bin/gofumpt -w internal/cli/reviews/submissions_history.go internal/cli/reviews/submissions_history_test.go internal/cli/reviews/review.go`

If gofumpt changes anything, commit the fix.

- [ ] **Step 2: Run make format**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && make format`

- [ ] **Step 3: Run make lint**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && make lint`

Fix any issues found.

- [ ] **Step 4: Run full test suite**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && ASC_BYPASS_KEYCHAIN=1 make test`

All tests must pass.

- [ ] **Step 5: Generate and check command docs**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && make generate-command-docs && make check-command-docs`

Since we added a new command, `docs/COMMANDS.md` will be updated. Commit the changes.

- [ ] **Step 6: Final commit if needed**

```bash
cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI
git add -A
git commit -m "chore: format and update command docs for submissions-history"
```

### Task 6: Live Smoke Test (Optional)

**Files:** None — manual verification.

- [ ] **Step 1: Build binary**

Run: `cd /Users/abdoelrhman/Developer/side/QuraanTV/App-Store-Connect-CLI && go build -o /tmp/asc .`

- [ ] **Step 2: Test with SajdaTV**

Run: `/tmp/asc review submissions-history --app 6759179587 --platform TV_OS`

Verify: Output shows a table of past submissions with version strings and outcomes.

Run: `/tmp/asc review submissions-history --app 6759179587 --output json --pretty`

Verify: JSON output with `submissionId`, `versionString`, `platform`, `state`, `submittedDate`, `outcome`, `items` fields.

- [ ] **Step 3: Test help output**

Run: `/tmp/asc review submissions-history --help`

Verify: Shows all flags, examples, and the API limitation note.
