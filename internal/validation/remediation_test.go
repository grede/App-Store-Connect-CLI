package validation

import "testing"

func TestRemediationStepsOrdersBlockingErrorsBeforeWarningsAndInfos(t *testing.T) {
	checks := []CheckResult{
		{
			ID:          "warning.first",
			Severity:    SeverityWarning,
			Message:     "warning",
			Remediation: "fix warning",
		},
		{
			ID:          "error.second",
			Severity:    SeverityError,
			Message:     "error",
			Remediation: "fix error",
		},
		{
			ID:          "info.third",
			Severity:    SeverityInfo,
			Message:     "info",
			Remediation: "review info",
		},
	}

	steps := RemediationSteps(checks, false)
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].CheckID != "error.second" {
		t.Fatalf("expected error first, got %+v", steps)
	}
	if steps[1].CheckID != "warning.first" {
		t.Fatalf("expected warning second, got %+v", steps)
	}
	if steps[2].CheckID != "info.third" {
		t.Fatalf("expected info third, got %+v", steps)
	}
	if !steps[0].Blocking {
		t.Fatalf("expected error step to be blocking, got %+v", steps[0])
	}
	if steps[1].Blocking {
		t.Fatalf("expected warning step to be non-blocking without strict mode, got %+v", steps[1])
	}
}

func TestBuildRemediationReportNextLimitsToSingleStep(t *testing.T) {
	report := Report{
		AppID:     "app-1",
		VersionID: "ver-1",
		Summary: Summary{
			Errors:   1,
			Warnings: 1,
			Blocking: 1,
		},
		Checks: []CheckResult{
			{
				ID:          "metadata.required.description",
				Severity:    SeverityError,
				Message:     "description is required",
				Remediation: "Provide a description",
			},
			{
				ID:          "metadata.required.whats_new",
				Severity:    SeverityWarning,
				Message:     "what's new is empty",
				Remediation: "Provide release notes",
			},
		},
	}

	next := BuildRemediationReport(report, RemediationModeNext)
	if next.Mode != RemediationModeNext {
		t.Fatalf("expected next mode, got %q", next.Mode)
	}
	if next.TotalActionable != 2 {
		t.Fatalf("expected total actionable 2, got %d", next.TotalActionable)
	}
	if len(next.Steps) != 1 {
		t.Fatalf("expected one selected step, got %d", len(next.Steps))
	}
	if next.Steps[0].CheckID != "metadata.required.description" {
		t.Fatalf("expected first step to be description remediation, got %+v", next.Steps[0])
	}
}
