# payment +pay

Create a payment transaction.

## Command

```bash
evo-cli payment +pay \
  --amount <amount> \
  --currency <currency> \
  --payment-brand <brand> \
  [--payment-type card|e-wallet|onlineBanking|bankTransfer] \
  [--platform WEB|APP|WAP|MINI] \
  [--merchant-tx-id <id>] \
  [--webhook <url>] \
  [--return-url <url>]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--amount` | Payment amount (e.g. "100.00") |
| `--currency` | ISO 4217 currency code (e.g. USD, EUR, CNY) |
| `--payment-brand` | Payment brand (e.g. Alipay, VISA, WeChat_Pay) |

## Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--payment-type` | e-wallet | Payment method type |
| `--platform` | WEB | Transaction platform |
| `--merchant-tx-id` | auto-generated | Merchant transaction ID |
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
