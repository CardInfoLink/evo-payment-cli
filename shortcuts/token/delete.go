package token

import (
	"context"
	"fmt"

	"github.com/evopayment/evo-cli/shortcuts"
)

// DeleteShortcut defines the "token +delete" shortcut.
// DELETE /g2/v1/payment/mer/{sid}/paymentMethod
// Risk: high-risk-write
func DeleteShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "token",
		Command:     "+delete",
		Description: "Delete a gateway token",
		Risk:        shortcuts.RiskHighRiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "token-id", Desc: "Token ID to delete", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/paymentMethod", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?token=" + rt.Str("token-id")
			body := map[string]interface{}{"initiatingReason": "deleted via CLI"}
			return shortcuts.DryRunOutput(rt.IO, "DELETE", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/paymentMethod", rt.Config.MerchantSid)
			params := map[string]string{"token": rt.Str("token-id")}
			body := map[string]interface{}{"initiatingReason": "deleted via CLI"}
			data, err := rt.DoJSON("DELETE", path, params, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}
