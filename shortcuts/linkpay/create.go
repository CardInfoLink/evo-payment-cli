package linkpay

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CreateShortcut defines the "linkpay +create" shortcut.
// POST /g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay
func CreateShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "linkpay",
		Command:     "+create",
		Description: "Create a LinkPay order",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "amount", Desc: "Payment amount", Required: true},
			{Name: "currency", Desc: "Currency code", Required: true},
			{Name: "order-id", Desc: "Merchant order ID", Required: true},
			{Name: "return-url", Desc: "Return URL after payment"},
			{Name: "webhook", Desc: "Webhook notification URL"},
			{Name: "valid-time", Desc: "Link validity time in minutes"},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpay", rt.Config.MerchantSid)
			url := rt.Config.ResolveLinkPayBaseURL("") + path
			body := buildCreateBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v0/payment/mer/%s/evo.e-commerce.linkpay", rt.Config.MerchantSid)
			body := buildCreateBody(rt)
			data, err := rt.DoLinkPayJSON("POST", path, nil, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildCreateBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	body := map[string]interface{}{
		"merchantOrderInfo": map[string]interface{}{
			"merchantOrderID":   rt.Str("order-id"),
			"merchantOrderTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
	}
	if v := rt.Str("return-url"); v != "" {
		body["returnURL"] = v
	}
	if v := rt.Str("webhook"); v != "" {
		body["webhook"] = v
	}
	if v := rt.Str("valid-time"); v != "" {
		body["validTime"] = v
	}
	return body
}
