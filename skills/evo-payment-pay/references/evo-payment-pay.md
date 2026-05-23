# payment +pay

Create a payment transaction.

## Command

```bash
evo-cli payment +pay \
  --amount <amount> \
  --currency <currency> \
  [--payment-brand <brand>] \
  [--payment-type card|e-wallet|onlineBanking|bankTransfer|token] \
  [--gateway-token <token_value>] \
  [--platform WEB|APP|WAP|MINI] \
  [--merchant-tx-id <id>] \
  [--auto-capture true] \
  [--allow-authentication true] \
  [--webhook <url>] \
  [--return-url <url>]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--amount` | Payment amount (e.g. "100.00") |
| `--currency` | ISO 4217 currency code (e.g. USD, EUR, CNY) |

## Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--payment-brand` | — | Payment brand (e.g. Alipay, VISA, WeChat_Pay) |
| `--payment-type` | e-wallet | Payment method type |
| `--gateway-token` | — | Gateway token value (auto-sets payment-type to token) |
| `--platform` | WEB | Transaction platform |
| `--merchant-tx-id` | auto-generated | Merchant transaction ID (also used as Idempotency-Key) |
| `--auto-capture` | — | Set to "true" for immediate capture (captureAfterHours=0) |
| `--allow-authentication` | — | Set to "true" to enable 3DS authentication |
| `--webhook` | — | Webhook notification URL |
| `--return-url` | — | Redirect URL after payment |

## Example

```bash
evo-cli payment +pay --amount 10.00 --currency USD --payment-brand Alipay
```

## Tips

- Use `--dry-run` to preview the request before sending
- Check response `action` object — may require user redirect
- If result.code is S0003, poll with `payment +query`
