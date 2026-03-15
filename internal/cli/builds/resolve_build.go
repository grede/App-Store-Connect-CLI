package builds

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ResolveBuildOptions configures how a build is resolved.
type ResolveBuildOptions struct {
	BuildID     string
	AppID       string
	Version     string
	BuildNumber string
	Platform    string
	Latest      bool
}

// ResolveBuild finds a build by ID, by app+build-number+platform, or by latest.
// Returns the build response or an error. Callers use this to avoid duplicating
// build lookup logic across commands (dsyms, wait, find, etc.).
func ResolveBuild(ctx context.Context, client *asc.Client, opts ResolveBuildOptions) (*asc.BuildResponse, error) {
	// Direct build ID — just wrap it.
	if opts.BuildID != "" {
		resp, err := client.GetBuild(ctx, opts.BuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch build %s: %w", opts.BuildID, err)
		}
		return resp, nil
	}

	appID := strings.TrimSpace(opts.AppID)
	buildNumber := strings.TrimSpace(opts.BuildNumber)

	// Reject conflicting selectors early, before any API calls.
	if opts.Latest && buildNumber != "" {
		return nil, fmt.Errorf("--latest and --build-number are mutually exclusive")
	}

	if appID == "" {
		return nil, fmt.Errorf("--app is required when --build is not provided (or set ASC_APP_ID)")
	}

	resolvedAppID, err := shared.ResolveAppIDWithLookup(ctx, client, appID)
	if err != nil {
		return nil, err
	}

	platform := strings.TrimSpace(opts.Platform)
	version := strings.TrimSpace(opts.Version)

	// Latest mode: find the most recently uploaded build.
	if opts.Latest {
		buildOpts := []asc.BuildsOption{
			asc.WithBuildsSort("-uploadedDate"),
			asc.WithBuildsLimit(1),
		}
		if platform != "" {
			buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
		}
		if version != "" {
			buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionVersion(version))
		}

		buildsResp, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest build: %w", err)
		}
		if len(buildsResp.Data) == 0 {
			return nil, fmt.Errorf("no builds found for app %s", resolvedAppID)
		}
		return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
	}

	// Build number mode: find by app + build number + platform.
	if buildNumber == "" {
		return nil, fmt.Errorf("--build, --latest, or --build-number is required")
	}

	buildOpts := []asc.BuildsOption{
		asc.WithBuildsBuildNumber(buildNumber),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(1),
	}
	if platform != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}
	if version != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionVersion(version))
	}

	buildsResp, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
	if err != nil {
		return nil, err
	}
	if len(buildsResp.Data) == 0 {
		return nil, fmt.Errorf("no build found for app %s with build number %q", resolvedAppID, buildNumber)
	}

	return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
}
