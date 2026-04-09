package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
)

// NewCmdConfigRemove creates the "config remove" command.
func NewCmdConfigRemove(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove configuration and stored credentials",
		Long:  "Delete ~/.evo-cli/config.json and remove the SignKey from the OS keychain.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigRemove(f)
		},
	}
	return cmd
}

func runConfigRemove(f cmdutil.Factory) error {
	io := f.IOStreams()

	configPath, err := core.DefaultConfigPath()
	if err != nil {
		return writeRemoveError(io, fmt.Sprintf("failed to determine config path: %v", err))
	}

	// Try to load config first to find keychain reference.
	cfg, loadErr := core.LoadConfig(configPath)
	if loadErr == nil && cfg.SignKey.IsRef() && cfg.SignKey.Ref.Source == "keychain" {
		kc := keychain.New()
		if err := kc.Remove(cfg.SignKey.Ref.ID); err != nil {
			// Log but don't fail — config file removal is more important.
			fmt.Fprintf(io.ErrOut, "warning: failed to remove keychain entry: %v\n", err)
		}
	}

	// Remove config file.
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return writeRemoveError(io, fmt.Sprintf("failed to remove config file: %v", err))
	}

	envelope := map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"message": "Configuration and credentials removed",
		},
	}
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func writeRemoveError(io *cmdutil.IOStreams, msg string) error {
	envelope := map[string]interface{}{
		"ok": false,
		"error": map[string]interface{}{
			"type":    "cli_error",
			"message": msg,
		},
	}
	enc := json.NewEncoder(io.ErrOut)
	enc.SetIndent("", "  ")
	_ = enc.Encode(envelope)
	return fmt.Errorf("%s", msg)
}
