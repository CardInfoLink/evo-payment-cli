package token

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CreateShortcut defines the "token +create" shortcut.
// POST /g2/v1/payment/mer/{sid}/paymentMethod
func CreateShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "token",
		Command:     "+create",
		Description: "Create a gateway token for a payment method",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "payment-type", Desc: "Payment type (e.g. card)", Required: true},
			{Name: "vault-id", Desc: "Vault ID", Required: true},
			{Name: "user-reference", Desc: "User reference", Required: true},
			{Name: "card-number", Desc: "Card number (required for card type)"},
			{Name: "card-expiry", Desc: "Card expiry date (MMYY)"},
			{Name: "card-cvc", Desc: "Card CVC/CVV"},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/paymentMethod", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path
			body := buildCreateBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/paymentMethod", rt.Config.MerchantSid)
			body := buildCreateBody(rt)
			data, err := rt.DoJSON("POST", path, nil, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildCreateBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	txID := fmt.Sprintf("tok_%d", time.Now().Unix())

	card := map[string]interface{}{
		"vaultID": rt.Str("vault-id"),
	}
	if cn := rt.Str("card-number"); cn != "" {
		cardInfo := map[string]interface{}{"cardNumber": cn}
		if exp := rt.Str("card-expiry"); exp != "" {
			cardInfo["expiryDate"] = exp
		}
		if cvc := rt.Str("card-cvc"); cvc != "" {
			cardInfo["cvc"] = cvc
		}
		card["cardInfo"] = cardInfo
	}

	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   txID,
			"merchantTransTime": now,
		},
		"paymentMethod": map[string]interface{}{
			"type": rt.Str("payment-type"),
			"card": card,
		},
		"userInfo": map[string]interface{}{
			"reference": rt.Str("user-reference"),
		},
		"transInitiator": map[string]interface{}{
			"platform": "WEB",
		},
	}
}
