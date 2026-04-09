# payment +cancel

Cancel a payment transaction. This is a **high-risk** operation.

## Command

```bash
evo-cli payment +cancel --original-merchant-tx-id <id> [--yes]
```

## Required Flags

| Flag | Description |
|------|-------------|
| `--original-merchant-tx-id` | Original payment transaction ID |

## Tips

- Use `--dry-run` to preview before executing
- Use `--yes` to skip confirmation prompt
