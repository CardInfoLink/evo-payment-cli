// Package token provides token management shortcut commands for evo-cli.
package token

import (
	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/shortcuts"
	"github.com/spf13/cobra"
)

// RegisterShortcuts registers all token shortcuts under the "token" parent command.
func RegisterShortcuts(rootCmd *cobra.Command, f cmdutil.Factory) {
	parent := findOrCreateSubcommand(rootCmd, "token", "Token management shortcut commands (+create, +query, +delete)")
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

// AllShortcuts returns all token shortcut definitions.
func AllShortcuts() []shortcuts.Shortcut {
	return []shortcuts.Shortcut{
		CreateShortcut(),
		QueryShortcut(),
		DeleteShortcut(),
	}
}
