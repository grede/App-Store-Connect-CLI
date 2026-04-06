package cmdtest

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRemediationFlagsRejectInvalidBooleanValues(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	binaryPath := filepath.Join(t.TempDir(), "asc")

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build asc binary: %v\n%s", err, string(output))
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "next invalid value",
			args: []string{"validate", "--app", "app-1", "--version-id", "ver-1", "--next=maybe"},
		},
		{
			name: "fix-plan invalid value",
			args: []string{"validate", "--app", "app-1", "--version-id", "ver-1", "--fix-plan=maybe"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, test.args...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected process exit error, got %v", err)
			}
			if exitErr.ExitCode() != 2 {
				t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
			}
			if stdout.String() != "" {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "invalid boolean value") {
				t.Fatalf("expected invalid boolean value error, got %q", stderr.String())
			}
		})
	}
}
