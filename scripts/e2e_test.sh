#!/usr/bin/env bash
#
# evo-cli End-to-End Test Suite
#
# Runs from an AI Agent's perspective: builds the binary, exercises every
# user-facing command, and validates structured JSON output / exit codes.
#
# Usage:
#   make e2e            # build + run
#   bash scripts/e2e_test.sh ./evo-cli   # run against an existing binary
#
set -euo pipefail

# ── Resolve binary path ──────────────────────────────────────────────
CLI="${1:-./evo-cli}"
if [[ ! -x "$CLI" ]]; then
  echo "FATAL: binary not found or not executable: $CLI"
  exit 1
fi

# ── Temp workspace (isolated HOME so we don't touch real config) ─────
export HOME
HOME="$(mktemp -d)"
trap 'rm -rf "$HOME"' EXIT

PASS=0
FAIL=0
TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); printf "  ✅  %s\n" "$1"; }
fail() { ((FAIL++)); ((TOTAL++)); printf "  ❌  %s\n" "$1"; }

assert_exit() {
  local want="$1"; shift
  local desc="$1"; shift
  set +e
  "$@" >/dev/null 2>&1
  local got=$?
  set -e
  if [[ "$got" == "$want" ]]; then pass "$desc (exit=$got)"; else fail "$desc (want exit=$want, got=$got)"; fi
}

assert_json_field() {
  local json="$1" field="$2" want="$3" desc="$4"
  local got
  got=$(echo "$json" | python3 -c "import sys,json; print(json.load(sys.stdin)$field)" 2>/dev/null || echo "__PARSE_ERROR__")
  if [[ "$got" == "$want" ]]; then pass "$desc"; else fail "$desc (want=$want, got=$got)"; fi
}

assert_contains() {
  local haystack="$1" needle="$2" desc="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then pass "$desc"; else fail "$desc (missing: $needle)"; fi
}

assert_not_contains() {
  local haystack="$1" needle="$2" desc="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then fail "$desc (should not contain: $needle)"; else pass "$desc"; fi
}

echo "═══════════════════════════════════════════════════════════════"
echo " evo-cli E2E Test Suite"
echo " Binary: $CLI"
echo " HOME:   $HOME"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo "▸ 1. Version & Help"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" --version 2>&1)
assert_contains "$OUT" "evo-cli version" "--version outputs version string"

OUT=$("$CLI" --help 2>&1)
assert_contains "$OUT" "config" "--help lists config command"
assert_contains "$OUT" "api" "--help lists api command"
assert_contains "$OUT" "schema" "--help lists schema command"
assert_contains "$OUT" "doctor" "--help lists doctor command"
assert_contains "$OUT" "completion" "--help lists completion command"
assert_contains "$OUT" "payment" "--help lists payment command"
assert_contains "$OUT" "linkpay" "--help lists linkpay command"
assert_contains "$OUT" "token" "--help lists token command"
assert_contains "$OUT" "cryptogram" "--help lists cryptogram command"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 2. Config — before init (no config file)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# config show should fail with config_missing
OUT=$("$CLI" config show 2>&1 || true)
assert_contains "$OUT" "config_missing" "config show → config_missing error"
assert_contains "$OUT" "evo-cli config init" "config show → hint mentions config init"

# doctor should report config failure
OUT=$("$CLI" doctor --offline 2>&1)
assert_contains "$OUT" "config_file" "doctor → has config_file check"
assert_contains "$OUT" "fail" "doctor → config_file fails before init"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 3. Config — init (without keychain, no --api-key-stdin)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" config init --merchant-sid S024116 --sign-type SHA256 --env test 2>&1)
assert_contains "$OUT" '"ok": true' "config init → ok=true"
assert_contains "$OUT" "S024116" "config init → output contains merchantSid"

# Verify config file exists
if [[ -f "$HOME/.evo-cli/config.json" ]]; then
  pass "config file created at ~/.evo-cli/config.json"
else
  fail "config file not created"
fi

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 4. Config — show (after init)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" config show 2>&1)
assert_contains "$OUT" '"ok": true' "config show → ok=true"
assert_contains "$OUT" "S024116" "config show → merchantSid present"
assert_contains "$OUT" "SHA256" "config show → signType present"
# SignKey should be masked
assert_contains "$OUT" "****" "config show → signKey is masked"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 5. Config — init validation"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Missing --merchant-sid
assert_exit 1 "config init without --merchant-sid fails" "$CLI" config init
# Invalid sign type
assert_exit 1 "config init with invalid sign-type fails" "$CLI" config init --merchant-sid S001 --sign-type MD5
# Invalid env
assert_exit 1 "config init with invalid env fails" "$CLI" config init --merchant-sid S001 --env staging

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 6. Config — env var override"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$(EVO_MERCHANT_SID=S_ENV "$CLI" config show 2>&1)
assert_contains "$OUT" "S_ENV" "EVO_MERCHANT_SID overrides config file"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 7. Doctor — after init"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" doctor --offline 2>&1)
assert_contains "$OUT" "config_file" "doctor → config_file check present"
assert_contains "$OUT" "pass" "doctor → at least one check passes"
assert_contains "$OUT" "skip" "doctor --offline → connectivity skipped"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 8. Schema — introspection"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# List all services
OUT=$("$CLI" schema 2>&1)
assert_contains "$OUT" "payment" "schema → lists payment service"
assert_contains "$OUT" "linkpay" "schema → lists linkpay service"

# List resources under payment
OUT=$("$CLI" schema payment 2>&1)
assert_contains "$OUT" "online" "schema payment → lists online resource"

# List methods under payment.online
OUT=$("$CLI" schema payment.online 2>&1)
assert_contains "$OUT" "pay" "schema payment.online → lists pay method"
assert_contains "$OUT" "query" "schema payment.online → lists query method"

# Show method detail
OUT=$("$CLI" schema payment.online.pay 2>&1)
assert_contains "$OUT" "POST" "schema payment.online.pay → shows POST method"
assert_contains "$OUT" "sid" "schema payment.online.pay → shows sid parameter"

# Error: nonexistent service
assert_exit 1 "schema nonexistent → error" "$CLI" schema nonexistent

# Error: too many segments
assert_exit 1 "schema a.b.c.d → error" "$CLI" schema a.b.c.d

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 9. API command — dry-run"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" api POST /g2/v1/payment/mer/S024116/payment --data '{"amount":"100"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "api dry-run → method=POST"
assert_contains "$OUT" "/g2/v1/payment" "api dry-run → URL contains path"
assert_contains "$OUT" "amount" "api dry-run → body contains amount"

# dry-run with params
OUT=$("$CLI" api GET /g2/v1/payment --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "api GET dry-run → method=GET"
assert_contains "$OUT" "merchantTransID" "api GET dry-run → URL contains param"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 10. API command — validation"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Invalid method
assert_exit 1 "api PATCH → unsupported method" "$CLI" api PATCH /test
# Missing args
assert_exit 1 "api no args → error" "$CLI" api
# Invalid JSON data
assert_exit 1 "api invalid --data → error" "$CLI" api POST /test --data "not-json"
# Invalid JSON params
assert_exit 1 "api invalid --params → error" "$CLI" api GET /test --params "not-json"
# Path traversal in --data @file
assert_exit 1 "api @../../etc/passwd → path traversal rejected" "$CLI" api POST /test --data "@../../etc/passwd"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 11. Payment shortcuts — dry-run"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment +pay --amount 100 --currency USD --payment-brand Alipay --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "payment +pay dry-run → POST"
assert_contains "$OUT" "payment" "payment +pay dry-run → path contains payment"
assert_contains "$OUT" "Alipay" "payment +pay dry-run → body contains Alipay"

# payment +pay with gateway token
OUT=$("$CLI" payment +pay --amount 10 --currency USD --gateway-token pmt_abc123 --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "payment +pay gateway token dry-run → POST"
assert_contains "$OUT" '"type": "token"' "payment +pay gateway token dry-run → paymentMethod.type=token"
assert_contains "$OUT" "pmt_abc123" "payment +pay gateway token dry-run → body contains token value"

OUT=$("$CLI" payment +query --merchant-tx-id TX001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "payment +query dry-run → GET"
assert_contains "$OUT" "TX001" "payment +query dry-run → URL contains TX001"

OUT=$("$CLI" payment +capture --original-merchant-tx-id TX001 --amount 50 --currency USD --dry-run 2>&1)
assert_contains "$OUT" "capture" "payment +capture dry-run → path contains capture"

OUT=$("$CLI" payment +cancel --original-merchant-tx-id TX001 --yes --dry-run 2>&1)
assert_contains "$OUT" "cancel" "payment +cancel dry-run → path contains cancel"

OUT=$("$CLI" payment +refund --original-merchant-tx-id TX001 --amount 25 --currency USD --yes --dry-run 2>&1)
assert_contains "$OUT" "refund" "payment +refund dry-run → path contains refund"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 12. Payment shortcuts — validation"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

assert_exit 1 "+pay missing required → error" "$CLI" payment +pay --amount 100
assert_exit 1 "+pay invalid enum → error" "$CLI" payment +pay --amount 100 --currency USD --payment-brand V --payment-type invalid
assert_exit 1 "+query missing required → error" "$CLI" payment +query
assert_exit 1 "+capture missing required → error" "$CLI" payment +capture --amount 50
assert_exit 1 "+cancel missing required → error" "$CLI" payment +cancel --yes
assert_exit 1 "+refund missing required → error" "$CLI" payment +refund --amount 25 --yes

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 13. LinkPay shortcuts — dry-run"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" linkpay +create --amount 200 --currency EUR --order-id ORD001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "linkpay +create dry-run → POST"
assert_contains "$OUT" "linkpay" "linkpay +create dry-run → path contains linkpay"
assert_contains "$OUT" "ORD001" "linkpay +create dry-run → body contains order ID"

OUT=$("$CLI" linkpay +query --merchant-order-id ORD001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "linkpay +query dry-run → GET"
assert_contains "$OUT" "ORD001" "linkpay +query dry-run → URL contains ORD001"

OUT=$("$CLI" linkpay +refund --merchant-order-id ORD001 --amount 50 --currency USD --yes --dry-run 2>&1)
assert_contains "$OUT" "linkpayRefund" "linkpay +refund dry-run → path contains linkpayRefund"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 14. LinkPay shortcuts — validation"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

assert_exit 1 "+create missing required → error" "$CLI" linkpay +create --amount 100
assert_exit 1 "+query missing required → error" "$CLI" linkpay +query
assert_exit 1 "+refund missing required → error" "$CLI" linkpay +refund --amount 50 --yes

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 15. Token shortcuts — dry-run"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" token +create --payment-type card --vault-id V001 --user-reference user@test.com --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "token +create dry-run → POST"
assert_contains "$OUT" "paymentMethod" "token +create dry-run → path contains paymentMethod"

OUT=$("$CLI" token +query --merchant-tx-id TX001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "token +query dry-run → GET"

OUT=$("$CLI" token +delete --token-id TK001 --yes --dry-run 2>&1)
assert_contains "$OUT" '"method": "DELETE"' "token +delete dry-run → DELETE"

# token +create with new flags (network-token-only, email)
OUT=$("$CLI" token +create --payment-type card --vault-id V001 --user-reference user@test.com --network-token-only true --email test@test.com --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "token +create new flags dry-run → POST"
assert_contains "$OUT" "paymentMethod" "token +create new flags dry-run → path contains paymentMethod"
assert_contains "$OUT" "networkTokenOnly" "token +create new flags dry-run → body contains networkTokenOnly"

# token +create with --allow-authentication
OUT=$("$CLI" token +create --payment-type card --vault-id V001 --user-reference user@test.com --allow-authentication true --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "token +create --allow-authentication dry-run → POST"
assert_contains "$OUT" "allowAuthentication" "token +create --allow-authentication dry-run → body contains allowAuthentication"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 16. Token shortcuts — validation"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

assert_exit 1 "+create missing required → error" "$CLI" token +create --payment-type card
assert_exit 1 "+query missing required → error" "$CLI" token +query
assert_exit 1 "+delete missing required → error" "$CLI" token +delete --yes

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 17. Shell completion"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

for shell in bash zsh fish powershell; do
  OUT=$("$CLI" completion "$shell" 2>&1)
  if [[ -n "$OUT" ]]; then
    pass "completion $shell → non-empty output"
  else
    fail "completion $shell → empty output"
  fi
done

assert_exit 1 "completion invalid-shell → error" "$CLI" completion tcsh

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 18. Output format — dry-run with --format"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# JSON (default) — already tested above
# Verify --format flag is accepted (dry-run output is always JSON, but the flag should not error)
OUT=$("$CLI" payment +pay --amount 100 --currency USD --payment-brand V --dry-run --format json 2>&1)
assert_contains "$OUT" "method" "--format json accepted"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 19. Config — remove"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" config remove 2>&1)
assert_contains "$OUT" '"ok": true' "config remove → ok=true"

# Verify config file is gone
if [[ ! -f "$HOME/.evo-cli/config.json" ]]; then
  pass "config file removed after config remove"
else
  fail "config file still exists after config remove"
fi

# config show should fail again
assert_exit 1 "config show after remove → error" "$CLI" config show

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 20. Service commands — dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Re-init config for service command tests
"$CLI" config init --merchant-sid S024116 --sign-type SHA256 --env test >/dev/null 2>&1

# payment online pay --help should show --data flag
OUT=$("$CLI" payment online pay --help 2>&1)
assert_contains "$OUT" "--data" "service cmd payment online pay has --data flag"
assert_contains "$OUT" "--params" "service cmd payment online pay has --params flag"

# linkpay order create --help
OUT=$("$CLI" linkpay order create --help 2>&1)
assert_contains "$OUT" "--data" "service cmd linkpay order create has --data flag"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 21. Payout dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment payout create --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "payout create dry-run → POST"
assert_contains "$OUT" "/payout" "payout create dry-run → path contains /payout"

OUT=$("$CLI" payment payout query --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "payout query dry-run → GET"
assert_contains "$OUT" "/payout" "payout query dry-run → path contains /payout"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 22. FX Rate dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment fxRate inquiry --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "fxRate inquiry dry-run → POST"
assert_contains "$OUT" "/FXRateInquiry" "fxRate inquiry dry-run → path contains /FXRateInquiry"

OUT=$("$CLI" payment fxRate query --params '{"baseCurrency":"USD","quoteCurrency":"EUR"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "fxRate query dry-run → GET"
assert_contains "$OUT" "/FXRateInquiry" "fxRate query dry-run → path contains /FXRateInquiry"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 23. Cryptogram dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment cryptogram create --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "cryptogram create dry-run → POST"
assert_contains "$OUT" "/cryptogram" "cryptogram create dry-run → path contains /cryptogram"

OUT=$("$CLI" payment cryptogram query --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "cryptogram query dry-run → GET"
assert_contains "$OUT" "/cryptogram" "cryptogram query dry-run → path contains /cryptogram"

# Cryptogram shortcut dry-run tests
OUT=$("$CLI" cryptogram +create --network-token-id NTK001 --original-merchant-tx-id TX001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "cryptogram +create dry-run → POST"
assert_contains "$OUT" "/cryptogram" "cryptogram +create dry-run → path contains /cryptogram"

OUT=$("$CLI" cryptogram +query --merchant-tx-id CRYPTO001 --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "cryptogram +query dry-run → GET"
assert_contains "$OUT" "/cryptogram" "cryptogram +query dry-run → path contains /cryptogram"

OUT=$("$CLI" cryptogram +pay --network-token-value 2222030194871591 --token-expiry-date 1226 --token-cryptogram AAA --eci 06 --payment-brand Mastercard --amount 10 --currency USD --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "cryptogram +pay dry-run → POST"
assert_contains "$OUT" "/payment" "cryptogram +pay dry-run → path contains /payment"
assert_contains "$OUT" "networkToken" "cryptogram +pay dry-run → body contains networkToken"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 24. PaymentMethod dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment paymentMethod create --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "paymentMethod create dry-run → POST"
assert_contains "$OUT" "/paymentMethod" "paymentMethod create dry-run → path contains /paymentMethod"

OUT=$("$CLI" payment paymentMethod list --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "paymentMethod list dry-run → GET"
assert_contains "$OUT" "/paymentMethod" "paymentMethod list dry-run → path contains /paymentMethod"

OUT=$("$CLI" payment paymentMethod update --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "PUT"' "paymentMethod update dry-run → PUT"
assert_contains "$OUT" "/paymentMethod" "paymentMethod update dry-run → path contains /paymentMethod"

OUT=$("$CLI" payment paymentMethod delete --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "DELETE"' "paymentMethod delete dry-run → DELETE"
assert_contains "$OUT" "/paymentMethod" "paymentMethod delete dry-run → path contains /paymentMethod"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 25. Payment online full dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" payment online pay --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "online pay dry-run → POST"
assert_contains "$OUT" "/payment" "online pay dry-run → path contains /payment"

OUT=$("$CLI" payment online query --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "online query dry-run → GET"
assert_contains "$OUT" "/payment" "online query dry-run → path contains /payment"

OUT=$("$CLI" payment online cancel --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "online cancel dry-run → POST"
assert_contains "$OUT" "/cancel" "online cancel dry-run → path contains /cancel"

OUT=$("$CLI" payment online cancelQuery --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "online cancelQuery dry-run → GET"
assert_contains "$OUT" "/cancel" "online cancelQuery dry-run → path contains /cancel"

OUT=$("$CLI" payment online cancelOrRefund --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "online cancelOrRefund dry-run → POST"
assert_contains "$OUT" "/cancelOrRefund" "online cancelOrRefund dry-run → path contains /cancelOrRefund"

OUT=$("$CLI" payment online cancelOrRefundQuery --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "online cancelOrRefundQuery dry-run → GET"
assert_contains "$OUT" "/cancelOrRefund" "online cancelOrRefundQuery dry-run → path contains /cancelOrRefund"

OUT=$("$CLI" payment online capture --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "online capture dry-run → POST"
assert_contains "$OUT" "/capture" "online capture dry-run → path contains /capture"

OUT=$("$CLI" payment online captureQuery --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "online captureQuery dry-run → GET"
assert_contains "$OUT" "/capture" "online captureQuery dry-run → path contains /capture"

OUT=$("$CLI" payment online submitAdditionalInfo --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "PUT"' "online submitAdditionalInfo dry-run → PUT"
assert_contains "$OUT" "/payment" "online submitAdditionalInfo dry-run → path contains /payment"

OUT=$("$CLI" payment online refund --data '{"test":true}' --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "online refund dry-run → POST"
assert_contains "$OUT" "/refund" "online refund dry-run → path contains /refund"

OUT=$("$CLI" payment online refundQuery --params '{"merchantTransID":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "online refundQuery dry-run → GET"
assert_contains "$OUT" "/refund" "online refundQuery dry-run → path contains /refund"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 26. LinkPay order full dry-run (registry-generated)"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" linkpay order create --data '{"test":true}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "linkpay order create dry-run → POST"
assert_contains "$OUT" "evo.e-commerce.linkpay" "linkpay order create dry-run → path contains evo.e-commerce.linkpay"

OUT=$("$CLI" linkpay order query --params '{"merchantOrderID of LinkPay":"ORD001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "linkpay order query dry-run → GET"
assert_contains "$OUT" "evo.e-commerce.linkpay" "linkpay order query dry-run → path contains evo.e-commerce.linkpay"

OUT=$("$CLI" linkpay order cancelOrRefund --data '{"test":true}' --params '{"merchantOrderID of LinkPay":"ORD001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "linkpay order cancelOrRefund dry-run → POST"
assert_contains "$OUT" "linkpayCancelorRefund" "linkpay order cancelOrRefund dry-run → path contains linkpayCancelorRefund"

OUT=$("$CLI" linkpay order refund --data '{"test":true}' --params '{"merchantOrderID of LinkPay":"ORD001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "POST"' "linkpay order refund dry-run → POST"
assert_contains "$OUT" "linkpayRefund" "linkpay order refund dry-run → path contains linkpayRefund"

OUT=$("$CLI" linkpay order refundQuery --params '{"merchantTransID of LinkPay Refund":"TX001"}' --dry-run 2>&1)
assert_contains "$OUT" '"method": "GET"' "linkpay order refundQuery dry-run → GET"
assert_contains "$OUT" "linkpayRefund" "linkpay order refundQuery dry-run → path contains linkpayRefund"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 27. --format Flag tests"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

assert_exit 0 "--format table accepted" "$CLI" payment +pay --amount 100 --currency USD --payment-brand V --dry-run --format table
assert_exit 0 "--format csv accepted" "$CLI" payment +pay --amount 100 --currency USD --payment-brand V --dry-run --format csv
assert_exit 0 "--format pretty accepted" "$CLI" payment +pay --amount 100 --currency USD --payment-brand V --dry-run --format pretty
# Invalid format: verify it is rejected (non-zero exit code)
assert_exit 1 "--format invalid_format → rejected" "$CLI" payment +pay --amount 100 --currency USD --payment-brand V --dry-run --format invalid_format

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 28. -o file output Flag test"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

E2E_OUT_FILE="/tmp/e2e_output.json"
rm -f "$E2E_OUT_FILE"
"$CLI" api POST /test --data '{"a":1}' --dry-run -o "$E2E_OUT_FILE" >/dev/null 2>&1 || true
if [[ -f "$E2E_OUT_FILE" ]]; then
  pass "-o flag → file created"
  # Validate JSON content
  if python3 -c "import sys,json; json.load(open('$E2E_OUT_FILE'))" 2>/dev/null; then
    pass "-o flag → file contains valid JSON"
  else
    fail "-o flag → file does not contain valid JSON"
  fi
else
  fail "-o flag → file not created"
fi
rm -f "$E2E_OUT_FILE"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 29. --idempotency-key Flag test"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OUT=$("$CLI" api PUT /test --data '{"a":1}' --dry-run --idempotency-key test-key-001 2>&1)
assert_contains "$OUT" "Idempotency-Key" "PUT --idempotency-key → header present"

OUT=$("$CLI" api DELETE /test --dry-run --idempotency-key test-key-002 2>&1)
assert_contains "$OUT" "Idempotency-Key" "DELETE --idempotency-key → header present"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "▸ 30. Exit Code systematic verification"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

assert_exit 0 "--version → exit 0" "$CLI" --version
assert_exit 1 "api PATCH /test → non-zero exit" "$CLI" api PATCH /test
assert_exit 1 "api POST /test --data not-json → non-zero exit" "$CLI" api POST /test --data "not-json"

# config show with no config → non-zero exit
# Config was re-initialized in section 20, so remove it first
"$CLI" config remove >/dev/null 2>&1 || true
assert_exit 1 "config show (no config) → non-zero exit" "$CLI" config show

assert_exit 1 "config init (missing --merchant-sid) → non-zero exit" "$CLI" config init
assert_exit 1 "api POST /test --data @../../etc/passwd → non-zero exit" "$CLI" api POST /test --data "@../../etc/passwd"
assert_exit 1 "completion tcsh → non-zero exit" "$CLI" completion tcsh
assert_exit 1 "schema nonexistent → non-zero exit" "$CLI" schema nonexistent

# Re-init config so subsequent tests still work
"$CLI" config init --merchant-sid S024116 --sign-type SHA256 --env test >/dev/null 2>&1

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "═══════════════════════════════════════════════════════════════"
printf " Results: %d passed, %d failed, %d total\n" "$PASS" "$FAIL" "$TOTAL"
echo "═══════════════════════════════════════════════════════════════"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
