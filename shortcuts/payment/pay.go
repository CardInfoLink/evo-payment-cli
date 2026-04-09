package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// PayShortcut defines the "payment +pay" shortcut.
// POST /g2/v1/payment/mer/{sid}/payment
func PayShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "payment",
		Command:     "+pay",
		Description: "Create a payment transaction",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "amount", Desc: "Payment amount", Required: true},
			{Name: "currency", Desc: "Currency code (e.g. USD, EUR)", Required: true},
			{Name: "payment-brand", Desc: "Payment brand (e.g. VISA, Alipay)", Required: true},
			{Name: "payment-type", Desc: "Payment type", Default: "e-wallet", Enum: []string{"card", "e-wallet", "onlineBanking", "bankTransfer"}},
			{Name: "platform", Desc: "Platform type", Default: "WEB", Enum: []string{"WEB", "APP", "WAP", "MINI"}},
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID"},
			{Name: "webhook", Desc: "Webhook notification URL"},
			{Name: "return-url", Desc: "Return URL after payment"},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/payment", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path
			body := buildPayBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/payment", rt.Config.MerchantSid)
			body := buildPayBody(rt)
			data, err := rt.DoJSON("POST", path, nil, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildPayBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	txID := rt.Str("merchant-tx-id")
	if txID == "" {
		txID = fmt.Sprintf("sc_%d", time.Now().Unix())
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	paymentType := rt.Str("payment-type")
	brand := rt.Str("payment-brand")

	pm := map[string]interface{}{"type": paymentType}
	switch paymentType {
	case "e-wallet":
		pm["e-wallet"] = map[string]interface{}{"paymentBrand": brand}
	case "card":
		pm["card"] = map[string]interface{}{"cardInfo": map[string]interface{}{}}
	case "onlineBanking":
		pm["onlineBanking"] = map[string]interface{}{"paymentBrand": brand}
	case "bankTransfer":
		pm["bankTransfer"] = map[string]interface{}{"paymentBrand": brand}
	}

	body := map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   txID,
			"merchantTransTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
		"paymentMethod":  pm,
		"transInitiator": map[string]interface{}{"platform": rt.Str("platform")},
	}
	if v := rt.Str("webhook"); v != "" {
		body["webhook"] = v
	}
	if v := rt.Str("return-url"); v != "" {
		body["returnURL"] = v
	}
	// tradeInfo is required when paymentBrand is Alipay
	if brand == "Alipay" {
		body["tradeInfo"] = map[string]interface{}{
			"tradeType": "Sale of goods",
		}
	}
	return body
}
