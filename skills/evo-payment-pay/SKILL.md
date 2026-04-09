---
name: evo-payment-pay
version: 1.0.0
description: "Payment operations with Evo Payment CLI. Use when the user needs to create, query, capture, cancel, or refund payments."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — Payment Operations

> **Prerequisites:** Read [`../evo-payment-shared/SKILL.md`](../evo-payment-shared/SKILL.md) first.

## Shortcuts (prefer these)

| Shortcut | Description |
|----------|-------------|
| [`+pay`](references/evo-payment-pay.md) | Create a payment |
| [`+query`](references/evo-payment-query.md) | Query payment status |
| [`+capture`](references/evo-payment-capture.md) | Capture pre-authorized payment |
| [`+cancel`](references/evo-payment-cancel.md) | Cancel a payment (high-risk) |
| [`+refund`](references/evo-payment-refund.md) | Refund a payment (high-risk) |

## API Resources

```bash
evo-cli schema payment.online.<method>   # Check parameters first
evo-cli payment online <method> [flags]  # Call API
```

## Error Handling

- S0000: Success
- S0003: User paying — poll with `+query`
- B0012: Invalid transaction status — check payment.status before operating
- B0013: Refund amount exceeded — check original payment amount
