# payment +refund

Refund a payment transaction. This is a **high-risk** operation.

## Command

```bash
evo-cli payment +refund \
  --original-merchant-tx-id <id> \
  --amount <amount> \
  --currency <currency> \
  [--merchant-tx-id <id>] \
  [--yes]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--original-merchant-tx-id` | Original payment transaction ID |
| `--amount` | Refund amount |
| `--currency` | Currency code |

## Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--merchant-tx-id` | auto-generated | This refund's transaction ID (also used as Idempotency-Key) |

## Tips

- Partial refund: set amount less than original
- Use `--dry-run` to preview before executing
- B0013 error means refund amount exceeds original
