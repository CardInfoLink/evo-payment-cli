package linkpay

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// RefundShortcut defines the "linkpay +refund" shortcut.
// POST /g2/v0/payment/mer/{sid}/evo.e-commerce.linkpayRefund/{merchantOrderID}
// Risk: high-risk-write
func RefundShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "linkpay",
		Command:     "+refund",
		Description: "Refund a LinkPay order",
		Risk:        shortcuts.RiskHighRiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "merchant-order-id", Desc: "Merchant order ID", Required: true},
			{Name: "amount", Desc: "Refund amount", Required: true},
			{Name: "currency", Desc: "Currency code", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpayRefund/%s",
				rt.Config.MerchantSid, rt.Str("merchant-order-id"))
			url := rt.Config.ResolveLinkPayBaseURL("") + path
			body := buildRefundBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpayRefund/%s",
				rt.Config.MerchantSid, rt.Str("merchant-order-id"))
			body := buildRefundBody(rt)
			data, err := rt.DoLinkPayJSON("POST", path, nil, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildRefundBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   fmt.Sprintf("lp_ref_%d", time.Now().Unix()),
			"merchantTransTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
	}
}
