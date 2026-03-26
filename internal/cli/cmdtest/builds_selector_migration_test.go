package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	buildsLegacyBuildWarning  = "Warning: `--build` is deprecated. Use `--build-id`."
	buildsLegacyIDWarning     = "Warning: `--id` as a build selector is deprecated. Use `--build-id`."
	buildsLegacyNewestWarning = "Warning: `--newest` is deprecated. Use `--latest`."
)

func TestBuildsSelectorAliasesWarnAndMatchCanonicalValidationPaths(t *testing.T) {
	run := func(t *testing.T, args []string) (string, string, error) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		return stdout, stderr, runErr
	}

	tests := []struct {
		name          string
		canonicalArgs []string
		aliasArgs     []string
		warning       string
		wantErr       string
	}{
		{
			name:          "wait build alias",
			canonicalArgs: []string{"builds", "wait", "--build-id", "BUILD_123", "--poll-interval", "0"},
			aliasArgs:     []string{"builds", "wait", "--build", "BUILD_123", "--poll-interval", "0"},
			warning:       buildsLegacyBuildWarning,
			wantErr:       "Error: --poll-interval must be greater than 0",
		},
		{
			name:          "wait newest alias",
			canonicalArgs: []string{"builds", "wait", "--app", "APP_123", "--latest", "--poll-interval", "0"},
			aliasArgs:     []string{"builds", "wait", "--app", "APP_123", "--newest", "--poll-interval", "0"},
			warning:       buildsLegacyNewestWarning,
			wantErr:       "Error: --poll-interval must be greater than 0",
		},
		{
			name:          "dsyms build alias",
			canonicalArgs: []string{"builds", "dsyms", "--build-id", "BUILD_123", "--app", "APP_123"},
			aliasArgs:     []string{"builds", "dsyms", "--build", "BUILD_123", "--app", "APP_123"},
			warning:       buildsLegacyBuildWarning,
			wantErr:       "Error: --build-id cannot be combined with --app, --latest, --build-number, --version, or --platform",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonicalStdout, canonicalStderr, canonicalErr := run(t, test.canonicalArgs)
			if !errors.Is(canonicalErr, flag.ErrHelp) {
				t.Fatalf("expected canonical ErrHelp, got %v", canonicalErr)
			}
			if canonicalStdout != "" {
				t.Fatalf("expected empty canonical stdout, got %q", canonicalStdout)
			}
			if !strings.Contains(canonicalStderr, test.wantErr) {
				t.Fatalf("expected canonical stderr to contain %q, got %q", test.wantErr, canonicalStderr)
			}

			aliasStdout, aliasStderr, aliasErr := run(t, test.aliasArgs)
			if !errors.Is(aliasErr, flag.ErrHelp) {
				t.Fatalf("expected alias ErrHelp, got %v", aliasErr)
			}
			if aliasStdout != "" {
				t.Fatalf("expected empty alias stdout, got %q", aliasStdout)
			}
			requireStderrContainsWarning(t, aliasStderr, test.warning)
			if strings.Contains(aliasStderr, "flag provided but not defined") {
				t.Fatalf("expected deprecated alias warning instead of parse error, got %q", aliasStderr)
			}

			if stripDeprecatedCommandWarnings(aliasStderr) != stripDeprecatedCommandWarnings(canonicalStderr) {
				t.Fatalf("expected alias stderr to match canonical stderr apart from warnings, canonical=%q alias=%q", canonicalStderr, aliasStderr)
			}
		})
	}
}

func TestBuildsSelectorAliasesWarnAndMatchCanonicalFetchPaths(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/builds/BUILD_123":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","relationships":{}}}`), nil
		case "/v1/builds/BUILD_123/preReleaseVersion":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`), nil
		case "/v1/builds/BUILD_123/app":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case "/v1/builds/BUILD_123/icons":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildIcons","id":"icon-1"}]}`), nil
		case "/v1/builds/BUILD_123/betaAppReviewSubmission":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"review-1"}}`), nil
		case "/v1/builds/BUILD_123/buildBetaDetail":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1"}}`), nil
		case "/v1/builds/BUILD_123/relationships/app":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case "/v1/builds/BUILD_123/metrics/betaBuildUsages":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaBuildUsages","id":"usage-1"}]}`), nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	run := func(t *testing.T, args []string) (string, string) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	tests := []struct {
		name          string
		canonicalArgs []string
		aliasArgs     []string
		warning       string
	}{
		{
			name:          "info build alias",
			canonicalArgs: []string{"builds", "info", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "info", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "app view id alias",
			canonicalArgs: []string{"builds", "app", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "app", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "pre-release-version view id alias",
			canonicalArgs: []string{"builds", "pre-release-version", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "pre-release-version", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "icons list build alias",
			canonicalArgs: []string{"builds", "icons", "list", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "icons", "list", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "beta-app-review-submission view id alias",
			canonicalArgs: []string{"builds", "beta-app-review-submission", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "beta-app-review-submission", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "build-beta-detail view build alias",
			canonicalArgs: []string{"builds", "build-beta-detail", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "build-beta-detail", "view", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "links view build alias",
			canonicalArgs: []string{"builds", "links", "view", "--build-id", "BUILD_123", "--type", "app", "--output", "json"},
			aliasArgs:     []string{"builds", "links", "view", "--build", "BUILD_123", "--type", "app", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "metrics beta-usages build alias",
			canonicalArgs: []string{"builds", "metrics", "beta-usages", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "metrics", "beta-usages", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonicalStdout, canonicalStderr := run(t, test.canonicalArgs)
			aliasStdout, aliasStderr := run(t, test.aliasArgs)

			if canonicalStderr != "" {
				t.Fatalf("expected canonical stderr to be empty, got %q", canonicalStderr)
			}
			requireStderrContainsWarning(t, aliasStderr, test.warning)
			assertOnlyDeprecatedCommandWarnings(t, aliasStderr)

			if canonicalStdout != aliasStdout {
				t.Fatalf("expected canonical and alias stdout to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
			}
		})
	}
}
