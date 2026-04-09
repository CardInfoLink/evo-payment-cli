package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// RefundShortcut defines the "payment +refund" shortcut.
// POST /g2/v1/payment/mer/{sid}/refund
// Risk: high-risk-write
func RefundShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+refund",
		Description: "Refund a payment transaction",
		Risk:        shortcuts.RiskHighRiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "original-merchant-tx-id", Desc: "Original merchant transaction ID", Required: true},
			{Name: "amount", Desc: "Refund amount", Required: true},
			{Name: "currency", Desc: "Currency code", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/refund", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("original-merchant-tx-id")
			body := buildRefundBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/refund", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("original-merchant-tx-id")}
			body := buildRefundBody(rt)
			data, err := rt.DoJSON("POST", path, params, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildRefundBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   fmt.Sprintf("ref_%d", time.Now().Unix()),
			"merchantTransTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
	}
}
