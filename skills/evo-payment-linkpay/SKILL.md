---
name: evo-payment-linkpay
version: 1.0.0
description: "LinkPay operations with Evo Payment CLI. Use when the user needs to create payment links, query orders, or refund LinkPay orders."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — LinkPay Operations

> **Prerequisites:** Read [`../evo-payment-shared/SKILL.md`](../evo-payment-shared/SKILL.md) first.

## Shortcuts

| Shortcut | Description |
|----------|-------------|
| `+create` | Create a LinkPay order and get payment link |
| `+query` | Query LinkPay order status |
| `+refund` | Refund a LinkPay order (high-risk) |

## Usage

```bash
evo-cli linkpay +create --amount 100 --currency USD --order-id ORD001
evo-cli linkpay +query --merchant-order-id ORD001
evo-cli linkpay +refund --merchant-order-id ORD001 --amount 50 --currency USD --yes
```

## API Resources

```bash
evo-cli schema linkpay.order.<method>
evo-cli linkpay order <method> [flags]
```
