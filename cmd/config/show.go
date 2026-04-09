package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
)

// NewCmdConfigShow creates the "config show" command.
func NewCmdConfigShow(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display current configuration",
		Long:  "Show the current evo-cli configuration. SignKey is displayed in masked form.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(f)
		},
	}
	return cmd
}

// configShowOutput is the JSON structure for config show output.
type configShowOutput struct {
	MerchantSid    string `json:"merchantSid"`
	SignKey        string `json:"signKey"`
	SignType       string `json:"signType"`
	Env            string `json:"env"`
	BaseURL        string `json:"baseUrl"`
	LinkPayBaseURL string `json:"linkPayBaseUrl"`
	KeyID          string `json:"keyID,omitempty"`
}

func runConfigShow(f cmdutil.Factory) error {
	io := f.IOStreams()

	cfg, err := f.Config()
	if err != nil {
		if cmErr, ok := err.(*core.ConfigMissingError); ok {
			return writeShowError(io, cmErr.Type(), cmErr.Error(), cmErr.Hint())
		}
		return writeShowError(io, "cli_error", fmt.Sprintf("failed to load config: %v", err), "")
	}

	kc := keychain.New()
	maskedKey := cfg.MaskSignKey(kc)

	output := configShowOutput{
		MerchantSid:    cfg.MerchantSid,
		SignKey:        maskedKey,
		SignType:       cfg.SignType,
		Env:            cfg.Env,
		BaseURL:        cfg.ResolveBaseURL(""),
		LinkPayBaseURL: cfg.ResolveLinkPayBaseURL(""),
		KeyID:          cfg.KeyID,
	}

	envelope := map[string]interface{}{
		"ok":   true,
		"data": output,
	}
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func writeShowError(io *cmdutil.IOStreams, errType, msg, hint string) error {
	errObj := map[string]interface{}{
		"type":    errType,
		"message": msg,
	}
	if hint != "" {
		errObj["hint"] = hint
	}
	envelope := map[string]interface{}{
		"ok":    false,
		"error": errObj,
	}
	enc := json.NewEncoder(io.ErrOut)
	enc.SetIndent("", "  ")
	_ = enc.Encode(envelope)
	return fmt.Errorf("%s", msg)
}
