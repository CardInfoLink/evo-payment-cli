package cryptogram

import (
	"context"
	"fmt"

	"github.com/evopayment/evo-cli/shortcuts"
)

// QueryShortcut defines the "cryptogram +query" shortcut.
// GET /g2/v1/payment/mer/{sid}/cryptogram?merchantTransID=<id>
func QueryShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "cryptogram",
		Command:     "+query",
		Description: "Query cryptogram status by merchant transaction ID",
		Risk:        shortcuts.RiskRead,
		Flags: []shortcuts.Flag{
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID of the cryptogram request", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cryptogram", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("merchant-tx-id")
			return shortcuts.DryRunOutput(rt.IO, "GET", url, nil, nil)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cryptogram", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("merchant-tx-id")}
			data, err := rt.DoJSON("GET", path, params, nil)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}
