// Package completion implements the `evo-cli completion` command that generates
// shell completion scripts for bash, zsh, fish, and powershell.
package completion

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmdCompletion creates the `evo-cli completion <shell>` cobra command.
// It uses cobra's built-in completion generators for each supported shell.
func NewCmdCompletion() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		Long:      "Generate shell completion scripts for bash, zsh, fish, or powershell.",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, args[0])
		},
	}
	return cmd
}

// runCompletion generates the completion script for the specified shell.
func runCompletion(cmd *cobra.Command, shell string) error {
	rootCmd := cmd.Root()
	out := cmd.OutOrStdout()
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletionV2(out, true)
	case "zsh":
		return rootCmd.GenZshCompletion(out)
	case "fish":
		return rootCmd.GenFishCompletion(out, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}
