---
name: evo-payment-token
version: 1.0.0
description: "Token management with Evo Payment CLI. Use when the user needs to create, query, or delete gateway tokens."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — Token Management

> **Prerequisites:** Read [`../evo-payment-shared/SKILL.md`](../evo-payment-shared/SKILL.md) first.

## Shortcuts

| Shortcut | Description |
|----------|-------------|
| `+create` | Create a gateway token |
| `+query` | Query token status |
| `+delete` | Delete a token (high-risk) |

## Usage

```bash
evo-cli token +create --payment-type card --vault-id V001 --user-reference user@example.com
evo-cli token +query --merchant-tx-id TX001
evo-cli token +delete --token-id TK001 --yes
```
