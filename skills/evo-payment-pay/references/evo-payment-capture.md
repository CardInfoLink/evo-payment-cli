# payment +capture

Capture a pre-authorized payment.

## Command

```bash
evo-cli payment +capture \
  --original-merchant-tx-id <id> \
  --amount <amount> \
  --currency <currency> \
  [--merchant-tx-id <id>]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--original-merchant-tx-id` | Original payment transaction ID |
| `--amount` | Capture amount |
| `--currency` | Currency code |

## Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--merchant-tx-id` | auto-generated | This capture's transaction ID (also used as Idempotency-Key) |
