---
name: evo-payment-skill-maker
version: 1.0.0
description: "Create new evo-payment skills. Use when the user wants to create a skill for a new API domain."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — Skill Maker

## When to Use

Use this skill when you need to create a new skill for an Evo Payment API domain that doesn't have one yet.

## Skill Structure

```
skills/<skill-name>/
├── SKILL.md              # Main skill file
└── references/           # Optional: per-shortcut reference docs
    ├── <shortcut-1>.md
    └── <shortcut-2>.md
```

## SKILL.md Template

```markdown
---
name: evo-payment-<domain>
version: 1.0.0
description: "<description>. Use when <trigger scenario>."
metadata:
  requires:
    bins: ["evo-cli"]
---

# Evo Payment — <Domain> Operations

> **Prerequisites:** Read [`../evo-payment-shared/SKILL.md`](../evo-payment-shared/SKILL.md) first.

## Shortcuts

| Shortcut | Description |
|----------|-------------|
| `+verb` | Description |

## API Resources

\`\`\`bash
evo-cli schema <service>.<resource>.<method>
evo-cli <service> <resource> <method> [flags]
\`\`\`

## Error Handling

- List common error codes and recovery actions
```

## Principles

1. Always reference evo-payment-shared as prerequisite
2. Prefer shortcuts over raw API calls
3. Include `--dry-run` tips for write operations
4. Document error codes specific to the domain
5. Keep examples minimal and actionable
