// Package config provides the config command group for evo-cli.
package config

import (
	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
)

// NewCmdConfig creates the "config" command group with init, show, and remove subcommands.
func NewCmdConfig(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage evo-cli configuration",
		Long:  "Initialize, display, and remove the evo-cli configuration and stored credentials.",
	}

	cmd.AddCommand(NewCmdConfigInit(f))
	cmd.AddCommand(NewCmdConfigShow(f))
	cmd.AddCommand(NewCmdConfigRemove(f))

	return cmd
}
