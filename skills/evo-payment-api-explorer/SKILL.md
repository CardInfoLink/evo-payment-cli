---
name: evo-payment-api-explorer
version: 1.0.0
description: "Discover and call Evo Payment APIs not yet wrapped as shortcuts. Use when the user needs to access APIs beyond payment, linkpay, and token."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — API Explorer

> **Prerequisites:** Read [`../evo-payment-shared/SKILL.md`](../evo-payment-shared/SKILL.md) first.

## When to Use

Use this skill when the user needs to call Evo Payment APIs that don't have dedicated shortcuts, such as:
- Payout operations
- FX rate inquiry
- Cryptogram operations
- Future APIs (onboarding, settlement, disputes)

## Discovery Flow

1. List all available services:
   ```bash
   evo-cli schema
   ```

2. Explore a service's resources:
   ```bash
   evo-cli schema payment
   ```

3. Check method parameters:
   ```bash
   evo-cli schema payment.payout.create
   ```

4. Call via service command:
   ```bash
   evo-cli payment payout create --data '{"amount":"100","currency":"USD"}'
   ```

5. Or use raw API call:
   ```bash
   evo-cli api POST /g2/v1/payment/mer/{sid}/payout --data '{"amount":"100"}'
   ```

## Tips

- Always check `evo-cli schema` first to discover available APIs
- Use `--dry-run` to preview requests before sending
- Use `--format pretty` for human-readable output
