# payment +refund

Refund a payment transaction. This is a **high-risk** operation.

## Command

```bash
evo-cli payment +refund \
  --original-merchant-tx-id <id> \
  --amount <amount> \
  --currency <currency> \
  [--yes]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--original-merchant-tx-id` | Original payment transaction ID |
| `--amount` | Refund amount |
| `--currency` | Currency code |

## Tips

- Partial refund: set amount less than original
- Use `--dry-run` to preview before executing
- B0013 error means refund amount exceeds original
