package payment

import (
	"context"
	"fmt"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CaptureQueryShortcut defines the "payment +capture-query" shortcut.
// GET /g2/v1/payment/mer/{sid}/capture?merchantTransID=<id>
func CaptureQueryShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+capture-query",
		Description: "Query capture status by merchant transaction ID",
		Risk:        shortcuts.RiskRead,
		Flags: []shortcuts.Flag{
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID of the original payment", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/capture", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("merchant-tx-id")
			return shortcuts.DryRunOutput(rt.IO, "GET", url, nil, nil)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/capture", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("merchant-tx-id")}
			data, err := rt.DoJSON("GET", path, params, nil)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}
