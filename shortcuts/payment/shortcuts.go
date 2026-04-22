// Package payment provides payment shortcut commands for evo-cli.
package payment

import (
	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/shortcuts"
	"github.com/spf13/cobra"
)

// RegisterShortcuts registers all payment shortcuts under the "payment" parent command.
// If a "payment" command already exists (e.g., from service registration), shortcuts
// are mounted onto it. Otherwise a new parent command is created.
func RegisterShortcuts(rootCmd *cobra.Command, f cmdutil.Factory) {
	parent := findOrCreateSubcommand(rootCmd, "payment", "Payment commands (+pay, +query, +capture, +capture-query, +cancel, +cancel-query, +refund, +refund-query)")
	shortcuts.Mount(parent, f, AllShortcuts())
}

// findOrCreateSubcommand looks for an existing subcommand by name, or creates one.
func findOrCreateSubcommand(parent *cobra.Command, name, short string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	cmd := &cobra.Command{Use: name, Short: short}
	parent.AddCommand(cmd)
	return cmd
}

// AllShortcuts returns all payment shortcut definitions.
func AllShortcuts() []shortcuts.Shortcut {
	return []shortcuts.Shortcut{
		PayShortcut(),
		QueryShortcut(),
		CaptureShortcut(),
		CaptureQueryShortcut(),
		CancelShortcut(),
		CancelQueryShortcut(),
		RefundShortcut(),
		RefundQueryShortcut(),
	}
}
