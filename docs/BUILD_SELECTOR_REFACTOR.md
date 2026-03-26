# Build Selector Refactor

## Goal

Unify build selection across the `builds` command family under one resource-first
model:

- explicit build selection uses `--build-id`
- inferred build selection uses `--app ... --latest`
- specific build lookup uses `--app ... --build-number`

This cleanup standardizes deprecated and inconsistent selector vocabulary while
preserving compatibility aliases until the broader `1.0.0` cleanup.

## Accepted Decisions

- `--newest` remains as a deprecated alias for `--latest` during transition.
- `--latest` is the only inferred-build selector.
- `--build-id` is the canonical explicit build selector everywhere.
- `--build` remains as a deprecated alias for `--build-id` during transition.
- `--id` remains as a deprecated alias for `--build-id` on build-related read
  surfaces during transition.
- `--app ... --build-number` is a unique-lookup selector, not a ranked
  selector: after applying filters, zero matches is an error, more than one
  match is an error, and only exactly one match succeeds.
- `--app ... --latest` is the only ranked selector that may choose the newest
  matching build automatically.
- Implicit platform defaults for single-build lookup are removed from canonical
  selector behavior; the caller must pass `--platform` explicitly when needed to
  disambiguate results.
- `asc builds wait` keeps `--since`, but only as a wait-specific freshness
  constraint layered on top of `--latest` or `--build-number`, not as a
  standalone selector mode.
- `asc builds wait --app ... --version ...` without `--latest` or
  `--build-number` is not part of the final selector model.
- `asc builds latest` will be removed as a fetch command.
- `asc builds find` becomes a deprecated alias for `asc builds info` once
  `builds info` can resolve by `--build-number`, and removal moves to a later
  cleanup PR.
- `asc builds next-build-number` will replace current `asc builds latest --next`.
- `test-notes` becomes build-scoped plus `--locale`, not localization-ID-first.

## Non-Goals

- No app-first taxonomy rewrite. `builds` remains top-level.
- No mutation-command expansion in the first wave unless explicitly scoped.
- No selector-alias expansion beyond the existing `--build`, `--id`, and
  `--newest` compatibility spellings.

## PR Plan

### PR 1: Selector Vocabulary + Core Resolver

Status: complete

Progress checklist:

- [x] Add refactor tracker/design note
- [x] Standardize shared resolver validation wording to `--build-id`
- [x] Convert `asc builds dsyms` from `--build` to `--build-id`
- [x] Convert `asc builds wait` from `--build` / `--newest` to
  `--build-id` / `--latest`
- [x] Keep legacy selector spellings as deprecated aliases that warn and forward
  to the canonical flags
- [x] Update focused tests and command docs for the PR 1 slice
- [x] Defer deeper `builds wait` selector-engine sharing for now; current
  shared vocabulary cleanup is sufficient without adding more internal coupling
- [x] Extend `--build-id` vocabulary to the remaining read-oriented explicit
  build commands in `builds`

Scope:

- standardize explicit selector naming to `--build-id` in the shared
  resolver-facing commands
- standardize inferred selector naming to `--latest`
- keep `--newest` as a hidden deprecated alias for `--latest` on
  `asc builds wait`
- update shared resolver validation/error text to use `--build-id`
- keep command taxonomy unchanged for now

Commands in scope:

- `asc builds wait`
- `asc builds dsyms`
- `asc builds info`
- `asc builds app view`
- `asc builds pre-release-version view`
- `asc builds icons list`
- `asc builds beta-app-review-submission view`
- `asc builds build-beta-detail view`
- `asc builds links view`
- `asc builds metrics beta-usages`
- shared resolver helpers in `internal/cli/builds/resolve_build.go`

Files expected in scope:

- `internal/cli/builds/resolve_build.go`
- `internal/cli/builds/builds_wait.go`
- `internal/cli/builds/builds_dsyms.go`
- `internal/cli/builds/builds_dsyms_test.go`
- `internal/cli/cmdtest/builds_wait_test.go`
- `internal/cli/cmdtest/builds_dsyms_test.go`
- `internal/cli/cmdtest/commands_test.go`
- help/example updates in `internal/cli/builds/builds_commands.go` if touched

Design note:

1. Command placement in taxonomy
   Keep commands under `asc builds ...`. This PR only fixes selector vocabulary
   and shared resolution language.

2. OpenAPI / endpoint impact
   No endpoint shape changes. Existing build lookup and build fetch endpoints are
   reused; only CLI selector vocabulary and resolver plumbing change.

3. UX shape
   Canonical selector forms become:
   - `--build-id BUILD_ID`
   - `--app APP --latest`
   - `--app APP --build-number NUM`

4. Backward-compatibility / deprecation impact
   This PR keeps the existing selector aliases working in the touched commands
   while warning toward `--build-id` / `--latest`. Removal stays deferred to a
   later cleanup PR closer to `1.0.0`.

5. RED -> GREEN test plan
   - update command/unit tests to expect `--build-id`
   - update wait tests to prefer `--latest` while keeping `--newest` warnings
   - implement flag/help/warning changes
   - run focused tests for builds wait/dsyms/selector validation

### PR 2: Make `builds info` Canonical

Status: complete

Scope:

- add shared selector support to `asc builds info`
- keep `asc builds find` as a deprecated alias to `asc builds info`

Design note:

1. Command placement in taxonomy
   Keep the canonical entry point at `asc builds info`; retain `find` only as a
   hidden deprecated alias during the transition.

2. OpenAPI / endpoint impact
   Reuse `GET /v1/builds` filters (`filter[app]`, `filter[version]`,
   `filter[preReleaseVersion.platform]`) plus `GET /v1/builds/{id}` /
   `GET /v1/builds/{id}/preReleaseVersion`. No new endpoint surface is needed.

3. UX shape
   Canonical selector forms become:
   - `--build-id BUILD_ID`
   - `--app APP --latest`
   - `--app APP --build-number NUM [--platform PLATFORM]`

   `--app APP --build-number NUM` requires a unique match. When multiple
   platforms can satisfy the lookup, callers must pass
   `--platform PLATFORM` explicitly.

4. Backward-compatibility / deprecation impact
   `asc builds find` remains available in this PR as a deprecated shim that
   warns and forwards to `asc builds info`. The canonical migration path is:

   `asc builds find --app APP --build-number NUM`
   -> `asc builds info --app APP --build-number NUM`

   The deprecated shim resolves app-scoped selectors with the same
   unique-match semantics as `asc builds info`, including explicit
   `--platform` disambiguation when needed.

5. RED -> GREEN test plan
   - replace most `builds find` coverage with `builds info` selector coverage
   - add deprecated alias coverage for `builds find`
   - update validation/exit-code expectations for app-scoped `builds info`
   - keep `builds find` hidden from canonical help while preserving execution
   - run focused selector tests, then full required checks

### PR 3: Replace `builds latest` With `builds next-build-number`

Status: complete

Scope:

- move `--next` behavior into `asc builds next-build-number`
- remove `asc builds latest` as a fetch command

Design note:

1. Command placement in taxonomy
   Canonical "latest build details" lookup moves to `asc builds info --latest`.
   Canonical next build number calculation moves to `asc builds next-build-number`.
   `asc builds latest` stays only as a hidden deprecated compatibility shim
   during the transition.

2. OpenAPI / endpoint impact
   Reuse the same `GET /v1/builds`, `GET /v1/preReleaseVersions`, and
   `GET /v1/apps/{id}/buildUploads` calls already used by `builds latest`.
   No new API surface is required; this PR only redistributes existing lookup
   behavior across clearer CLI entry points.

3. UX shape
   Canonical commands become:
   - `asc builds info --app APP --latest`
   - `asc builds info --app APP --latest --version 1.2.3 --platform IOS`
   - `asc builds next-build-number --app APP --version 1.2.3 --platform IOS`

   `builds info --latest` should own latest-build fetch filters such as
   `--version`, `--platform`, `--processing-state`, and
   `--exclude-expired` / `--not-expired`.

4. Backward-compatibility / deprecation impact
   `asc builds latest` should stop being a canonical fetch command in help and
   docs, but remain available as a hidden deprecated shim for one transition
   cycle. Without `--next` it should warn toward `asc builds info --latest`;
   with `--next` it should warn toward `asc builds next-build-number`.
   Commands that resolve `--latest` through the shared build resolver now use
   the stronger latest-selection semantics that previously only lived in
   `asc builds latest`.

5. RED -> GREEN test plan
   - move latest-fetch coverage to `builds info --latest`
   - move next-build-number coverage to `builds next-build-number`
   - add deprecated alias coverage for `builds latest`
   - update workflow/docs/help text to use `builds info --latest` and
     `builds next-build-number`
   - run focused latest/next-build-number tests, then full required checks

### PR 4: Redesign `builds test-notes`

Status: complete

Scope:

- make `test-notes` build-scoped plus `--locale`
- replace build-related `--id` usage with `--build-id`
- optionally keep `--localization-id` as a low-level escape hatch
- keep hidden deprecated `--build` / `--id` flag aliases during the transition

### PR 5: Legacy Removal + Remaining Read Commands

Status: complete

Scope:

- remove live `beta-build-localizations` behavior and leave removed-command guidance
- remove live `builds test-notes get` behavior and leave removed-command guidance
- standardize remaining read-oriented build commands on `--build-id`

### PR 6: Mutating Command Selector Vocabulary

Status: complete

Scope:

- standardize visible explicit selector naming to `--build-id` for mutating build commands
- keep hidden deprecated `--build` aliases with warnings during the transition
- leave inferred selector rollout for mutating commands to PR 7 so the final
  selector contract can land in one pass

### PR 7: Finalize Unified Build Selectors

Status: complete

Scope:

- make the final selector contract true across build-targeting `asc builds`
  commands
- treat `--app ... --build-number` as exact-after-filtering unique lookup
- remove implicit `IOS` defaulting from canonical build-number lookup
- keep `--since` only as a `builds wait` freshness constraint on top of
  `--latest` or `--build-number`
- reject version-only or since-only `builds wait` selection
- update tests, docs, and generated command docs to reflect the final model

Design note:

1. Command placement in taxonomy
   Keep all single-build operations under `asc builds ...`. This PR finalizes
   selector semantics rather than moving commands across command groups.

2. OpenAPI / endpoint impact
   Reuse the existing build lookup endpoints. The change is in selector
   validation and client-side ambiguity handling, not in API shape.

3. UX shape
   The final selector contract for build-targeting commands is:
   - `--build-id BUILD_ID` for direct identity
   - `--app APP --latest [--version VER] [--platform PLATFORM]` for ranked
     selection
   - `--app APP --build-number NUM [--version VER] [--platform PLATFORM]` for
     unique lookup

   `--latest` is the only selector allowed to rank candidates and pick the most
   recent build automatically. `--build-number` must resolve to exactly one
   build after filters are applied, otherwise the CLI should return an
   actionable ambiguity error. `builds wait` may additionally accept `--since`
   as a freshness guard, but only together with `--latest` or `--build-number`.

4. Backward-compatibility / deprecation impact
   Keep existing hidden compatibility aliases (`--build`, `--id`, `--newest`,
   deprecated command wrappers) during the transition, but move canonical help,
   validation, and examples to the final selector contract.

5. RED -> GREEN test plan
   - add failing resolver tests for unique `--build-number` behavior
   - add failing `builds wait` tests for invalid version-only and since-only
     selection
   - migrate remaining build-targeting commands in `asc builds` to the shared
     selector contract
   - update command docs/help snapshots and migration tests
   - run focused selector tests, then full required checks

## Command Target Shape

Examples for the end state:

The `builds test-notes` examples below reflect the canonical selector shape
after the later selector-unification work.

```bash
asc builds info --build-id "BUILD_ID"
asc builds info --app "123" --latest
asc builds info --app "123" --build-number "42" --platform IOS

asc builds wait --app "123" --latest
asc builds dsyms --app "123" --build-number "42" --platform IOS

asc builds test-notes list --app "123" --latest
asc builds test-notes view --app "123" --latest --locale "en-US"
asc builds test-notes create --app "123" --latest --locale "en-US" --whats-new "..."
asc builds test-notes update --app "123" --latest --locale "en-US" --whats-new "..."
asc builds test-notes delete --app "123" --latest --locale "en-US" --confirm
```

## Notes

- `builds latest` remains in the repo after PR 3 as a hidden deprecated shim
  that warns toward `builds info --latest` and `builds next-build-number`.
- PR 6 standardized explicit `--build-id` for mutating commands as an
  incremental step. The final target is now unified selectors across
  build-targeting `asc builds` commands under the PR 7 contract above.
