package validation

import "sort"

// RemediationMode identifies the remediation-oriented validate output shape.
type RemediationMode string

const (
	RemediationModeNext    RemediationMode = "next"
	RemediationModeFixPlan RemediationMode = "fix-plan"
)

// RemediationStep represents one actionable item derived from a validation check.
type RemediationStep struct {
	Order        int      `json:"order"`
	Blocking     bool     `json:"blocking"`
	Severity     Severity `json:"severity"`
	CheckID      string   `json:"checkId"`
	Message      string   `json:"message"`
	Remediation  string   `json:"remediation"`
	Locale       string   `json:"locale,omitempty"`
	Field        string   `json:"field,omitempty"`
	ResourceType string   `json:"resourceType,omitempty"`
	ResourceID   string   `json:"resourceId,omitempty"`
}

// RemediationReport is the top-level output for validate remediation modes.
type RemediationReport struct {
	AppID           string            `json:"appId"`
	VersionID       string            `json:"versionId"`
	VersionString   string            `json:"versionString,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	Summary         Summary           `json:"summary"`
	Strict          bool              `json:"strict,omitempty"`
	Mode            RemediationMode   `json:"mode"`
	TotalActionable int               `json:"totalActionable"`
	Steps           []RemediationStep `json:"steps"`
}

// BuildRemediationReport derives a remediation-oriented view from a validation report.
func BuildRemediationReport(report Report, mode RemediationMode) RemediationReport {
	allSteps := RemediationSteps(report.Checks, report.Strict)
	steps := allSteps
	if mode == RemediationModeNext && len(steps) > 1 {
		steps = steps[:1]
	}

	return RemediationReport{
		AppID:           report.AppID,
		VersionID:       report.VersionID,
		VersionString:   report.VersionString,
		Platform:        report.Platform,
		Summary:         report.Summary,
		Strict:          report.Strict,
		Mode:            mode,
		TotalActionable: len(allSteps),
		Steps:           steps,
	}
}

// RemediationSteps orders actionable remediation steps from most urgent to least urgent.
func RemediationSteps(checks []CheckResult, strict bool) []RemediationStep {
	type candidate struct {
		check CheckResult
		index int
	}

	candidates := make([]candidate, 0, len(checks))
	for index, check := range checks {
		if check.Remediation == "" {
			continue
		}
		candidates = append(candidates, candidate{check: check, index: index})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := remediationPriority(candidates[i].check, strict)
		right := remediationPriority(candidates[j].check, strict)
		if left != right {
			return left < right
		}
		return candidates[i].index < candidates[j].index
	})

	steps := make([]RemediationStep, 0, len(candidates))
	for index, candidate := range candidates {
		check := candidate.check
		steps = append(steps, RemediationStep{
			Order:        index + 1,
			Blocking:     isBlockingSeverity(check.Severity, strict),
			Severity:     check.Severity,
			CheckID:      check.ID,
			Message:      check.Message,
			Remediation:  check.Remediation,
			Locale:       check.Locale,
			Field:        check.Field,
			ResourceType: check.ResourceType,
			ResourceID:   check.ResourceID,
		})
	}

	return steps
}

func remediationPriority(check CheckResult, strict bool) int {
	switch check.Severity {
	case SeverityError:
		return 0
	case SeverityWarning:
		if strict {
			return 1
		}
		return 2
	case SeverityInfo:
		return 3
	default:
		return 4
	}
}

func isBlockingSeverity(severity Severity, strict bool) bool {
	switch severity {
	case SeverityError:
		return true
	case SeverityWarning:
		return strict
	default:
		return false
	}
}
