package cryptogram

import (
	"context"
	"fmt"
	"time"

	"github.com/evopayment/evo-cli/shortcuts"
)

// CreateShortcut defines the "cryptogram +create" shortcut.
// POST /g2/v1/payment/mer/{sid}/cryptogram?merchantTransID=<originalTxID>
func CreateShortcut() shortcuts.Shortcut {
	return shortcuts.Shortcut{
		Service:     "cryptogram",
		Command:     "+create",
		Description: "Request a network token cryptogram for payment",
		Risk:        shortcuts.RiskWrite,
		Flags: []shortcuts.Flag{
			{Name: "network-token-id", Desc: "Network token ID from card scheme", Required: true},
			{Name: "original-merchant-tx-id", Desc: "merchantTransID of the original tokenization request", Required: true},
		},
		DryRun: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cryptogram", rt.Config.MerchantSid)
			url := rt.Config.ResolveBaseURL("") + path + "?merchantTransID=" + rt.Str("original-merchant-tx-id")
			body := buildCreateBody(rt)
			return shortcuts.DryRunOutput(rt.IO, "POST", url, nil, body)
		},
		Execute: func(ctx context.Context, rt *shortcuts.RuntimeContext) error {
			path := fmt.Sprintf("/g2/v1/payment/mer/%s/cryptogram", rt.Config.MerchantSid)
			params := map[string]string{"merchantTransID": rt.Str("original-merchant-tx-id")}
			body := buildCreateBody(rt)
			data, err := rt.DoJSON("POST", path, params, body)
			rt.OutFormat(data, nil, err)
			return nil
		},
	}
}

func buildCreateBody(rt *shortcuts.RuntimeContext) map[string]interface{} {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	txID := fmt.Sprintf("crypto_%d", time.Now().Unix())
	return map[string]interface{}{
		"merchantTransInfo": map[string]interface{}{
			"merchantTransID":   txID,
			"merchantTransTime": now,
		},
		"paymentMethod": map[string]interface{}{
			"type": "networkToken",
			"networkToken": map[string]interface{}{
				"tokenID": rt.Str("network-token-id"),
			},
		},
	}
}
