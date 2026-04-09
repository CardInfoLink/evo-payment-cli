package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CancelShortcut defines the "payment +cancel" shortcut.
// POST /g2/v1/payment/mer/{sid}/cancel
// Risk: high-risk-write
func CancelShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+cancel",
		Description: "Cancel a payment transaction",
		Risk:        shortcuts.RiskHighRiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "original-merchant-tx-id", Desc: "Original merchant transaction ID", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cancel", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("original-merchant-tx-id")
			body := buildCancelBody()
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cancel", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("original-merchant-tx-id")}
			body := buildCancelBody()
			data, err := rt.DoJSON("POST", path, params, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildCancelBody() map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   fmt.Sprintf("can_%d", time.Now().Unix()),
			"merchantTransTime": now,
		},
	}
}
