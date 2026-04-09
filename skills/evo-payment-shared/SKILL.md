---
name: evo-payment-shared
version: 1.0.0
description: "Shared rules for Evo Payment CLI: configuration, signing, error handling, security."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment CLI — Shared Rules

> Read this skill first before using any other evo-payment skill.

## Prerequisites

1. Install: `npm install -g @evopayment/cli` or `npx evo-cli`
2. Initialize: `echo "<your-sign-key>" | evo-cli config init --merchant-sid <SID> --sign-type SHA256 --api-key-stdin`
3. Verify: `evo-cli doctor`

## Configuration

- Config file: `~/.evo-cli/config.json`
- SignKey stored in OS keychain (never plaintext in config)
- Environment: `--env test|production` (default: test)
- Override via env vars: `EVO_MERCHANT_SID`, `EVO_SIGN_KEY`, `EVO_SIGN_TYPE`, `EVO_API_BASE_URL`

## Command Layers

| Layer | Use | Example |
|-------|-----|---------|
| `api` | Raw API call (any path) | `evo-cli api POST /g2/v1/payment/mer/{sid}/payment --data '{...}'` |
| `<service> <resource> <method>` | Auto-generated from registry | `evo-cli payment online pay --data '{...}'` |
| `<domain> +<verb>` | Simplified shortcuts | `evo-cli payment +pay --amount 100 --currency USD --payment-brand Alipay` |

Always prefer shortcuts > service commands > raw api calls.

## Error Handling

| result.code prefix | Meaning | Exit Code | Action |
|---------------------|---------|-----------|--------|
| S | Success | 0 | Proceed (S0003 = processing, query later) |
| V | Validation error | 2 | Fix request parameters |
| V0010 | Signature error | 3 | Check sign key: `evo-cli config show` |
| B, C | Business/resource error | 1 | Check business logic |
| P, I | PSP/issuer error | 6 | Retry or contact PSP |
| E | System error | 4 | Retry later |

## Security Rules

- Never pass SignKey as a CLI argument
- Use `--dry-run` before write operations to preview
- Confirm high-risk operations (refund, cancel, delete) or use `--yes`
- All connections use HTTPS (TLS 1.2+)

## Introspection

```bash
evo-cli schema                          # List all services
evo-cli schema payment                  # List resources
evo-cli schema payment.online.pay       # Show parameter details
```
