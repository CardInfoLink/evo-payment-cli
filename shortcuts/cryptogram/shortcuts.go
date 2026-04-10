// Package cryptogram provides cryptogram shortcut commands for evo-cli.
package cryptogram

import (
	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/shortcuts"
	"github.com/spf13/cobra"
)

// RegisterShortcuts registers all cryptogram shortcuts under the "cryptogram" parent command.
func RegisterShortcuts(rootCmd *cobra.Command, f cmdutil.Factory) {
	parent := findOrCreateSubcommand(rootCmd, "cryptogram", "Cryptogram shortcut commands (+create, +query, +pay)")
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

// AllShortcuts returns all cryptogram shortcut definitions.
func AllShortcuts() []shortcuts.Shortcut {
	return []shortcuts.Shortcut{
		CreateShortcut(),
		QueryShortcut(),
		PayShortcut(),
	}
}
