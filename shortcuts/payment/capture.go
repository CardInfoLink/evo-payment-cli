package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CaptureShortcut defines the "payment +capture" shortcut.
// POST /g2/v1/payment/mer/{sid}/capture
func CaptureShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+capture",
		Description: "Capture a pre-authorized payment",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "original-merchant-tx-id", Desc: "Original merchant transaction ID", Required: true},
			{Name: "amount", Desc: "Capture amount", Required: true},
			{Name: "currency", Desc: "Currency code", Required: true},
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID for this capture (auto-generated if omitted)"},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/capture", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("original-merchant-tx-id")
			body := buildCaptureBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/capture", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("original-merchant-tx-id")}
			body := buildCaptureBody(rt)
			data, err := rt.DoJSON("POST", path, params, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildCaptureBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	txID := rt.Str("merchant-tx-id")
	if txID == "" {
		txID = fmt.Sprintf("cap_%d", time.Now().Unix())
		rt.Flags["merchant-tx-id"] = txID
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   txID,
			"merchantTransTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
	}
}
