package linkpay

import (
	"context"
	"fmt"

	"github.com/evopayment/evo-cli/shortcuts"
)

// QueryShortcut defines the "linkpay +query" shortcut.
// GET /g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay/{merchantOrderID}
func QueryShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "linkpay",
		Command:     "+query",
		Description: "Query a LinkPay order by merchant order ID",
		Risk:        shortcuts.RiskRead,
		Flags: []shortcuts.Flag{
			{Name: "merchant-order-id", Desc: "Merchant order ID", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpay/%s",
				rt.Config.MerchantSid, rt.Str("merchant-order-id"))
			url := rt.Config.ResolveLinkPayBaseURL("") + path
			return shortcuts.DryRunOutput(rt.IO, "GET", url, nil, nil)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpay/%s",
				rt.Config.MerchantSid, rt.Str("merchant-order-id"))
			data, err := rt.DoLinkPayJSON("GET", path, nil, nil)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}
