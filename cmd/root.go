// Package cmd contains the root command and global flag registration for evo-cli.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	apiCmd "github.com/evopayment/evo-cli/cmd/api"
	completionCmd "github.com/evopayment/evo-cli/cmd/completion"
	configCmd "github.com/evopayment/evo-cli/cmd/config"
	doctorCmd "github.com/evopayment/evo-cli/cmd/doctor"
	schemaCmd "github.com/evopayment/evo-cli/cmd/schema"
	serviceCmd "github.com/evopayment/evo-cli/cmd/service"
	"github.com/evopayment/evo-cli/internal/build"
	"github.com/evopayment/evo-cli/internal/cmdutil"
	cryptogramShortcuts "github.com/evopayment/evo-cli/shortcuts/cryptogram"
	linkpayShortcuts "github.com/evopayment/evo-cli/shortcuts/linkpay"
	paymentShortcuts "github.com/evopayment/evo-cli/shortcuts/payment"
	tokenShortcuts "github.com/evopayment/evo-cli/shortcuts/token"
)

// NewRootCmd creates the root cobra command with all global flags registered.
func NewRootCmd(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evo-cli",
		Short: "Evo Payment CLI — AI Agent interface for Evo Payment APIs",
		Long: `evo-cli wraps the full Evo Payment API suite into a structured CLI
for AI Agents and developers. It handles message signing, HTTP headers,
error classification, and structured JSON output automatically.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (%s)", build.Version, build.Date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			if format != "" {
				switch format {
				case "json", "table", "csv", "pretty":
					// valid
				default:
					return fmt.Errorf("unsupported format %q: must be one of [json, table, csv, pretty]", format)
				}
			}
			return nil
		},
	}

	// Global flags
	cmd.PersistentFlags().String("format", "json", "Output format: json|table|csv|pretty")
	cmd.PersistentFlags().Bool("dry-run", false, "Preview the request without sending")
	cmd.PersistentFlags().String("env", "", "Override environment: test|production")
	cmd.PersistentFlags().StringP("output", "o", "", "Save response to file path")
	cmd.PersistentFlags().Bool("yes", false, "Skip confirmation for high-risk operations")

	// Register sub-commands.
	cmd.AddCommand(configCmd.NewCmdConfig(f))
	cmd.AddCommand(apiCmd.NewCmdAPI(f))
	cmd.AddCommand(schemaCmd.NewCmdSchema(f))
	cmd.AddCommand(doctorCmd.NewCmdDoctor(f))
	cmd.AddCommand(completionCmd.NewCmdCompletion())

	// Register service commands from Registry (auto-generated from meta_data.json).
	_ = serviceCmd.RegisterServiceCommands(cmd, f)

	// Register all shortcut commands (payment, linkpay, token, cryptogram).
	paymentShortcuts.RegisterShortcuts(cmd, f)
	linkpayShortcuts.RegisterShortcuts(cmd, f)
	tokenShortcuts.RegisterShortcuts(cmd, f)
	cryptogramShortcuts.RegisterShortcuts(cmd, f)

	return cmd
}

// Execute runs the root command. Called from main.go.
func Execute() {
	io := cmdutil.DefaultIOStreams()
	f := cmdutil.NewFactory(io)
	rootCmd := NewRootCmd(f)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
