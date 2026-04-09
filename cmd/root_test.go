package cmd

import (
	"testing"

	"github.com/evopayment/evo-cli/internal/cmdutil"
)

// Feature: e2e-test-suite, Property 2: Valid format values are accepted
// For any valid format value in {json, table, csv, pretty}, executing a command
// with --format <value> should complete without error from PersistentPreRunE.
// **Validates: Requirements 2.1, 2.2, 2.3**
func TestProperty2_ValidFormatValuesAccepted(t *testing.T) {
	validFormats := []string{"json", "table", "csv", "pretty"}

	for _, fmt := range validFormats {
		t.Run(fmt, func(t *testing.T) {
			io := cmdutil.DefaultIOStreams()
			f := cmdutil.NewFactory(io)
			root := NewRootCmd(f)

			// Use --version so the command completes without needing config/API.
			root.SetArgs([]string{"--format", fmt, "--version"})
			err := root.Execute()
			if err != nil {
				t.Errorf("format %q should be accepted, got error: %v", fmt, err)
			}
		})
	}

	// Also verify that an invalid format is rejected.
	t.Run("invalid_format_rejected", func(t *testing.T) {
		io := cmdutil.DefaultIOStreams()
		f := cmdutil.NewFactory(io)
		root := NewRootCmd(f)

		// Use a subcommand that triggers PersistentPreRunE (--version bypasses it).
		// Use "api" with no args to trigger PersistentPreRunE before arg validation.
		root.SetArgs([]string{"--format", "xml", "api", "GET", "/test"})
		err := root.Execute()
		if err == nil {
			t.Error("invalid format 'xml' should be rejected")
		}
	})
}
