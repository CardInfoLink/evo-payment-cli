package payment

import (
	"context"
	"fmt"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CancelQueryShortcut defines the "payment +cancel-query" shortcut.
// GET /g2/v1/payment/mer/{sid}/cancel?merchantTransID=<id>
func CancelQueryShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+cancel-query",
		Description: "Query cancel status by merchant transaction ID",
		Risk:        shortcuts.RiskRead,
		Flags: []shortcuts.Flag{
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID of the cancel request", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cancel", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("merchant-tx-id")
			return shortcuts.DryRunOutput(rt.IO, "GET", url, nil, nil)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cancel", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("merchant-tx-id")}
			data, err := rt.DoJSON("GET", path, params, nil)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}
