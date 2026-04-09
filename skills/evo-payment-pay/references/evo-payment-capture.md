# payment +capture

Capture a pre-authorized payment.

## Command

```bash
evo-cli payment +capture \
  --original-merchant-tx-id <id> \
  --amount <amount> \
  --currency <currency>
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--original-merchant-tx-id` | Original payment transaction ID |
| `--amount` | Capture amount |
| `--currency` | Currency code |
