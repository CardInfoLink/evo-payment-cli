package cryptogram

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// PayShortcut defines the "cryptogram +pay" shortcut.
// POST /g2/v1/payment/mer/{sid}/payment using network token + cryptogram.
func PayShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "cryptogram",
		Command:     "+pay",
		Description: "Pay with network token and cryptogram",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "network-token-value", Desc: "Network token value (card number form)", Required: true},
			{Name: "token-expiry-date", Desc: "Token expiry date (MMYY)", Required: true},
			{Name: "token-cryptogram", Desc: "Cryptogram value from +create", Required: true},
			{Name: "eci", Desc: "ECI value from +create response", Required: true},
			{Name: "payment-brand", Desc: "Payment brand (e.g. Visa, Mastercard)", Required: true},
			{Name: "amount", Desc: "Payment amount", Required: true},
			{Name: "currency", Desc: "Currency code (e.g. USD)", Required: true},
			{Name: "wallet-identifiers", Desc: "Wallet identifiers (default: MDESForMerchants for Mastercard)"},
			{Name: "merchant-tx-id", Desc: "Merchant transaction ID"},
			{Name: "return-url", Desc: "Return URL after payment"},
			{Name: "webhook", Desc: "Webhook notification URL"},
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
		txID = fmt.Sprintf("ntpay_%d", time.Now().Unix())
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	body := map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   txID,
			"merchantTransTime": now,
		},
		"transAmount": map[string]interface{}{
			"currency": rt.Str("currency"),
			"value":    rt.Str("amount"),
		},
		"paymentMethod": map[string]interface{}{
			"type": "token",
			"token": map[string]interface{}{
				"type":              "networkToken",
				"value":             rt.Str("network-token-value"),
				"expiryDate":        rt.Str("token-expiry-date"),
				"tokenCryptogram":   rt.Str("token-cryptogram"),
				"eci":               rt.Str("eci"),
				"paymentBrand":      rt.Str("payment-brand"),
				"walletIdentifiers": resolveWalletIdentifiers(rt.Str("wallet-identifiers"), rt.Str("payment-brand")),
			},
		},
		"transInitiator": map[string]interface{}{"platform": "WEB"},
	}
	if v := rt.Str("return-url"); v != "" {
		body["returnURL"] = v
	}
	if v := rt.Str("webhook"); v != "" {
		body["webhook"] = v
	}
	return body
}

// resolveWalletIdentifiers returns the explicit value if provided, otherwise a default based on brand.
func resolveWalletIdentifiers(explicit, brand string) string {
	if explicit != "" {
		return explicit
	}
	switch brand {
	case "Mastercard":
		return "MDESForMerchants"
	default:
		return ""
	}
}
