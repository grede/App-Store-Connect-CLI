package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsIntroductoryOffersImport_MissingRequiredFlagsReturnUsage(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing subscription id",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--input", "offers.csv"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "missing input",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --input is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidDefaultDatesReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid start date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--start-date", "2026-99-99",
			},
			wantErr: "--start-date must be in YYYY-MM-DD format",
		},
		{
			name: "invalid end date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--end-date", "2026-13-01",
			},
			wantErr: "--end-date must be in YYYY-MM-DD format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidDefaultOfferFlagsReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid offer duration",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-duration", "bad",
			},
			wantErr: "--offer-duration must be one of:",
		},
		{
			name: "invalid offer mode",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-mode", "bad",
			},
			wantErr: "--offer-mode must be one of:",
		},
		{
			name: "negative number of periods",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--number-of-periods", "-1",
			},
			wantErr: "--number-of-periods must be greater than or equal to 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_UnknownCSVColumnReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := filepath.Join(t.TempDir(), "offers.csv")
	if err := os.WriteFile(csvPath, []byte("territory,unknown\nUSA,value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unknown CSV column") {
		t.Fatalf("expected unknown column error, got %q", stderr)
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidCSVSchemaReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "empty file",
			body:    "",
			wantErr: "CSV file is empty",
		},
		{
			name:    "missing territory header",
			body:    "offer_mode\nFREE_TRIAL\n",
			wantErr: `CSV header must include required column "territory"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}
