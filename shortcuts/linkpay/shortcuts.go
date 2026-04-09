// Package linkpay provides LinkPay shortcut commands for evo-cli.
package linkpay

import (
	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/shortcuts"
	"github.com/spf13/cobra"
)

// RegisterShortcuts registers all linkpay shortcuts under the "linkpay" parent command.
func RegisterShortcuts(rootCmd *cobra.Command, f cmdutil.Factory) {
	parent := findOrCreateSubcommand(rootCmd, "linkpay", "LinkPay shortcut commands (+create, +query, +refund)")
	shortcuts.Mount(parent, f, AllShortcuts())
}

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

// AllShortcuts returns all linkpay shortcut definitions.
func AllShortcuts() []shortcuts.Shortcut {
	return []shortcuts.Shortcut{
		CreateShortcut(),
		QueryShortcut(),
		RefundShortcut(),
	}
}
