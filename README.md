# evo-cli

Evo Payment CLI — a structured command-line interface for AI Agents and developers to interact with the full Evo Payment API suite.

`evo-cli` handles message signing, HTTP headers, error classification, and structured JSON output automatically, so callers only need to focus on business logic.

## Quick Start

```bash
# Install
npm install -g @evopayment/cli
# or build from source
make build

# Initialize configuration (SignKey read from stdin, stored in OS keychain)
echo "your-sign-key" | evo-cli config init --merchant-sid S024116 --sign-type SHA256 --api-key-stdin

# Verify setup
evo-cli doctor

# Create a payment
evo-cli payment +pay --amount 10.00 --currency USD --payment-brand Alipay

# Query payment status
evo-cli payment +query --merchant-tx-id TX001
```

## Command Layers

evo-cli provides three layers of commands, from flexible to convenient:

| Layer | Use Case | Example |
|-------|----------|---------|
| `api` | Raw API call to any path | `evo-cli api POST /g2/v1/payment/mer/{sid}/payment --data '{...}'` |
| `service` | Auto-generated from API registry | `evo-cli payment online pay --data '{...}'` |
| `shortcuts` | Simplified high-frequency operations | `evo-cli payment +pay --amount 100 --currency USD --payment-brand VISA` |

### Shortcuts

**Payment:**
```bash
evo-cli payment +pay --amount 100 --currency USD --payment-brand Alipay
evo-cli payment +query --merchant-tx-id TX001
evo-cli payment +capture --original-merchant-tx-id TX001 --amount 100 --currency USD
evo-cli payment +cancel --original-merchant-tx-id TX001 --yes
evo-cli payment +refund --original-merchant-tx-id TX001 --amount 50 --currency USD --yes
```

**LinkPay:**
```bash
evo-cli linkpay +create --amount 100 --currency USD --order-id ORD001
evo-cli linkpay +query --merchant-order-id ORD001
evo-cli linkpay +refund --merchant-order-id ORD001 --amount 50 --currency USD --yes
```

**Token:**
```bash
evo-cli token +create --payment-type card --vault-id V001 --user-reference user@example.com \
  --card-number 4111111111111111 --card-expiry 1226 --card-cvc 123
evo-cli token +query --merchant-tx-id TX001
evo-cli token +delete --token-id TK001 --yes
```

### API Introspection

```bash
evo-cli schema                          # List all services
evo-cli schema payment                  # List resources under payment
evo-cli schema payment.online.pay       # Show full parameter definition
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--format json\|table\|csv\|pretty` | Output format (default: json) |
| `--dry-run` | Preview request without sending |
| `--env test\|production` | Override environment |
| `-o, --output <path>` | Save response to file |
| `--yes` | Skip confirmation for high-risk operations |

## Output Format

All output follows a structured JSON envelope:

```json
// Success → stdout
{"ok": true, "data": {...}, "meta": {"businessStatus": "Captured"}}

// Error → stderr
{"ok": false, "error": {"type": "business", "code": "B0013", "message": "...", "hint": "..."}}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Evo Payment business error (B/C prefix) |
| 2 | Parameter validation failure (V prefix) |
| 3 | Authentication/signature error |
| 4 | Network/system error (E prefix) |
| 5 | CLI internal error |
| 6 | PSP/issuer error (P/I prefix) |

## Configuration

Config file: `~/.evo-cli/config.json`

```bash
evo-cli config init --merchant-sid S024116 --sign-type SHA256 --env test --api-key-stdin
evo-cli config init --merchant-sid S024116 --api-base-url https://custom.example.com --api-key-stdin
evo-cli config init --merchant-sid S024116 --linkpay-base-url https://custom-linkpay.example.com --api-key-stdin
evo-cli config show      # Display config (SignKey masked)
evo-cli config remove    # Remove config and keychain entry
evo-cli doctor           # Health check (config → SignKey → signature → connectivity)
```

Environment variable overrides (take precedence over config file):

| Variable | Overrides |
|----------|-----------|
| `EVO_MERCHANT_SID` | merchantSid |
| `EVO_SIGN_KEY` | signKey |
| `EVO_SIGN_TYPE` | signType |
| `EVO_API_BASE_URL` | base URL |
| `EVO_LINKPAY_BASE_URL` | LinkPay base URL |

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Shortcuts (+pay, +query, +refund, ...)         │  Layer 3: hand-written
│  Service Commands (payment online pay, ...)     │  Layer 2: auto-generated from Registry
│  API Command (api POST /g2/v1/...)              │  Layer 1: raw API call
├─────────────────────────────────────────────────┤
│  Factory (lazy-loaded Config, HttpClient, ...)  │
│  EvoClient (DoAPI / CallAPI — 5-step response)  │
│  Transport Chain:                               │
│    Signature → Retry → UserAgent → Default      │
├─────────────────────────────────────────────────┤
│  Signature (SHA256/SHA512/HMAC-SHA256/HMAC-512) │
│  Config Manager (~/.evo-cli/config.json)        │
│  Keychain (macOS/Windows/Linux secret storage)  │
│  Registry (embedded meta_data.json + cache)     │
│  Output (Envelope, ExitCode, Formatter)         │
│  Validate (path safety, ANSI sanitize, HTTPS)   │
└─────────────────────────────────────────────────┘
```

## Development

```bash
# Build
make build

# Run unit tests
make test

# Run offline E2E tests (no API calls, uses --dry-run)
make e2e

# Run live E2E tests (calls real Evo Payment UAT APIs, requires .env)
make e2e-live

# Run live E2E tests with verbose HTTP request/response output
make e2e-live-verbose

# Run all tests (unit + offline E2E)
make test-all

# Regenerate meta_data.json from swagger files
make gen_meta

# Install locally
make install
```

### Debugging

Set `EVO_DEBUG=1` to print full HTTP request and response details (headers + body) to stderr:

```bash
EVO_DEBUG=1 evo-cli api GET /g2/v1/payment/mer/{sid}/payment --params '{"merchantTransID":"TX001"}'
```

### Project Structure

```
cmd/                    CLI commands (config, api, schema, service, doctor, completion)
shortcuts/              Shortcut framework + payment/linkpay/token shortcuts
internal/
├── build/              Version injection (ldflags)
├── cmdutil/            Factory, Transport chain, EvoClient, IOStreams
├── core/               Config manager, SecretInput, Keychain resolver
├── keychain/           Cross-platform secret storage (macOS/Windows/Linux)
├── output/             Envelope, ExitCode, error classification, formatter
├── registry/           API metadata loading (embedded + cache)
├── signature/          Message signing (SHA256/SHA512/HMAC)
└── validate/           Input security (path traversal, ANSI, HTTPS)
scripts/                gen_meta.py, e2e_test.sh, e2e_live_test.sh, install.js, run.js
skills/                 AI Agent skills (Markdown operation manuals)
```

## Distribution

**npm:**
```bash
npm install -g @evopayment/cli
npx evo-cli --version
```

**From source:**
```bash
make build
./evo-cli --version
```

**Cross-platform releases** via goreleaser: macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64).

## Skills (for AI Agents)

Skills are Markdown documents that teach AI Agents how to use the CLI:

| Skill | Description |
|-------|-------------|
| `evo-payment-shared` | Configuration, signing, error handling rules |
| `evo-payment-pay` | Payment operations (+pay, +query, +capture, +cancel, +refund) |
| `evo-payment-linkpay` | LinkPay operations (+create, +query, +refund) |
| `evo-payment-token` | Token management (+create, +query, +delete) |
| `evo-payment-api-explorer` | Discover and call APIs beyond shortcuts |
| `evo-payment-skill-maker` | Create new skills for new API domains |

## Shell Completion

```bash
# Bash
evo-cli completion bash > /etc/bash_completion.d/evo-cli

# Zsh
evo-cli completion zsh > "${fpath[1]}/_evo-cli"

# Fish
evo-cli completion fish > ~/.config/fish/completions/evo-cli.fish

# PowerShell
evo-cli completion powershell | Out-String | Invoke-Expression
```

## License

MIT
