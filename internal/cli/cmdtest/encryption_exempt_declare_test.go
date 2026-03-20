package cmdtest

import (
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestEncryptionDeclarationsExemptDeclare_RejectsPositionalArgs(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"encryption", "declarations", "exempt-declare",
			"extra",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "encryption declarations exempt-declare does not accept positional arguments") {
		t.Fatalf("expected positional-args error, got %q", stderr)
	}
}
