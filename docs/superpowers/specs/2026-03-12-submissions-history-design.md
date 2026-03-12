# Submissions History — Design Spec

## Goal

Add `asc review submissions-history` command that shows a chronological timeline of all review submissions for an app, enriched with version strings and item-level outcomes. Provides a single-command answer to "how many times was this version rejected?" and "what's my review history?".

## API Limitation

The App Store Connect API does **not** expose rejection reason text. Rejection reasons are only visible in the Resolution Center web UI and email notifications. This command surfaces submission *states* and *outcomes* — not the reasons behind them.

## Command

```
asc review submissions-history --app <APP_ID> [flags]
```

### Flags

| Flag         | Type   | Required | Default | Description |
|--------------|--------|----------|---------|-------------|
| `--app`      | string | yes*     | `ASC_APP_ID` | App Store Connect app ID |
| `--platform` | string | no       | —       | Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS |
| `--state`    | string | no       | —       | Filter by submission state (comma-separated) |
| `--version`  | string | no       | —       | Filter by version string (client-side post-fetch filter) |
| `--limit`    | int    | no       | 0       | Maximum results per page (1-200) |
| `--paginate` | bool   | no       | false   | Fetch all pages |
| `--output`   | string | no       | TTY-aware | Output format: json, table, markdown |
| `--pretty`   | bool   | no       | false   | Pretty-print JSON |

**No `--next` flag:** Unlike `submissions-list`, this command enriches each page with item/version data before rendering. Raw cursor forwarding is impractical because each page must be fully enriched before output. Use `--paginate` to fetch all results.

### Table Output

```
VERSION   PLATFORM  STATE              SUBMITTED              OUTCOME    ITEMS
3.1.1     TV_OS     COMPLETE           2026-03-01T12:00:00Z   approved   1 approved
3.1.0     TV_OS     COMPLETE           2026-02-15T10:00:00Z   approved   1 approved
3.0.0     TV_OS     UNRESOLVED_ISSUES  2026-01-20T08:00:00Z   rejected   1 rejected
2.0.0     TV_OS     COMPLETE           2025-12-01T09:00:00Z   approved   1 approved
```

### JSON Output

```json
[
  {
    "submissionId": "abc-123",
    "versionString": "3.1.1",
    "platform": "TV_OS",
    "state": "COMPLETE",
    "submittedDate": "2026-03-01T12:00:00Z",
    "outcome": "approved",
    "items": [
      {
        "id": "item-1",
        "state": "APPROVED",
        "type": "appStoreVersion",
        "resourceId": "ver-456"
      }
    ]
  }
]
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0    | Success (including empty results) |
| 1    | API error or internal error |
| 2    | Invalid flags or missing required flags |

## Architecture

### Approach: CLI-Layer Aggregation

The command reuses existing client methods with sequential API calls:

1. `client.GetReviewSubmissions(ctx, appID, opts ...asc.ReviewSubmissionsOption)` — fetch submissions
2. `client.GetReviewSubmissionItems(ctx, submissionID, opts ...asc.ReviewSubmissionItemsOption)` — fetch items per submission
3. `client.GetAppStoreVersion(ctx, versionID, opts ...asc.AppStoreVersionOption)` — fetch version string per item

**Why not `include` parameters?** The API supports `include=items` on the submissions endpoint, which would reduce API calls. However, the existing client methods don't support `include` query params, and adding that infrastructure is out of scope for this feature. The N+1 approach is acceptable because submission count per app is typically < 50. **Follow-up:** Add `include` support to the query builder as a separate optimization PR.

A single `shared.ContextWithTimeout(ctx)` covers the entire aggregation. Individual API calls share this context so the overall timeout applies.

### Data Flow

```
GetReviewSubmissions (with --paginate or single page)
  └─ for each submission:
       └─ skip if submittedDate is empty (pre-submission, not yet submitted)
       └─ GetReviewSubmissionItems(ctx, submissionID)
            └─ for each item with appStoreVersion relationship:
                 └─ GetAppStoreVersion(ctx, versionID) → extract versionString
       └─ assemble SubmissionHistoryEntry with derived outcome
  └─ apply --version filter (client-side, if specified)
  └─ sort by submittedDate descending
  └─ render via shared.PrintOutputWithRenderers (table/json/markdown)
```

### Outcome Derivation (priority order)

The `outcome` field is computed using this precedence:

1. If any item has state `REJECTED` → `"rejected"`
2. If all items have state `APPROVED` → `"approved"`
3. If submission state is `UNRESOLVED_ISSUES` (and no items are REJECTED — possible during state transition) → `"rejected"`
4. Otherwise → lowercase of submission state (e.g., `"in_review"`, `"waiting_for_review"`, `"complete"`)

This covers mixed states (e.g., 1 APPROVED + 1 ACCEPTED → falls through to rule 4, using submission state).

### New Types (internal/cli/reviews/)

```go
// SubmissionHistoryEntry is the assembled result for one submission.
type SubmissionHistoryEntry struct {
    SubmissionID  string                  `json:"submissionId"`
    VersionString string                  `json:"versionString"`
    Platform      string                  `json:"platform"`
    State         string                  `json:"state"`
    SubmittedDate string                  `json:"submittedDate"`
    Outcome       string                  `json:"outcome"`
    Items         []SubmissionHistoryItem `json:"items"`
}

// SubmissionHistoryItem is a summary of one item in a submission.
type SubmissionHistoryItem struct {
    ID         string `json:"id"`
    State      string `json:"state"`
    Type       string `json:"type"`
    ResourceID string `json:"resourceId"`
}

// SubmissionHistoryResult wraps the history entries for output.
type SubmissionHistoryResult struct {
    Entries []SubmissionHistoryEntry `json:"entries"`
}
```

### File Layout

| File | Purpose |
|------|---------|
| `internal/cli/reviews/submissions_history.go` | Command definition + aggregation logic + `deriveOutcome()` pure function |
| `internal/cli/reviews/submissions_history_test.go` | Unit tests (outcome derivation) + cmdtest integration tests |
| `internal/cli/reviews/review.go` | Add `SubmissionsHistoryCommand()` to subcommands list |

No new files in `internal/asc/`. Command must set `UsageFunc: shared.DefaultUsageFunc`. Registration is covered by existing `reviews.ReviewCommand()` in `internal/cli/registry/registry.go` line 116.

## Edge Cases

1. **No submissions:** Print empty table, exit 0
2. **Submission with no items:** Show entry with "0 items" in ITEMS column, outcome from rule 4 (lowercase state)
3. **Version fetch fails (404):** Use `"unknown"` for versionString via `asc.IsNotFound(err)`, don't fail the whole command
4. **Item without appStoreVersion relationship:** Include item in Items list with empty resourceId, skip version enrichment
5. **Empty submittedDate:** Skip submissions that haven't been submitted yet (state `READY_FOR_REVIEW`). These are drafts, not part of review history.
6. **API timeout:** Single `shared.ContextWithTimeout` covers entire aggregation; if it expires mid-enrichment, return error

## Backward Compatibility

New command — no existing behavior changes. Additive only.

## Testing Plan

TDD approach: RED (failing test) → GREEN (minimal implementation) → REFACTOR.

### Unit Tests (outcome derivation)
- `deriveOutcome()` pure function tested with:
  - All items APPROVED → "approved"
  - Any item REJECTED → "rejected"
  - UNRESOLVED_ISSUES state, no REJECTED items → "rejected"
  - Mixed non-rejected states → lowercase submission state
  - No items → lowercase submission state

### cmdtest Integration Tests (httptest mock server)
- **Happy path:** 2 submissions with mixed outcomes, assert table output and JSON parsed via `json.Unmarshal`
- **Empty results:** No submissions, assert empty table, exit 0
- **Version 404:** Version fetch returns 404, assert `"unknown"` in output
- **Flag validation:** Invalid `--limit` → exit code 2 + stderr message
- **Missing `--app`:** No app ID → exit code 2 + stderr message
- **`--version` filter:** Client-side filtering works correctly
- **`--platform` filter:** Platform normalization and API-level filtering

### Live Smoke Test
Read-only test with SajdaTV app (app ID `6759179587`, platform TV_OS).
