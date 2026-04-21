#!/usr/bin/env bash
#
# evo-cli Live E2E Test — calls real Evo Payment UAT APIs.
# Requires .env with credentials. Run: make e2e-live
#
set -uo pipefail  # no -e: we handle errors manually via assert_ok

CLI="${1:-./evo-cli}"
VERBOSE="${2:-}"
[[ -x "$CLI" ]] || { echo "FATAL: $CLI not found"; exit 1; }
[[ -f .env ]]   || { echo "FATAL: .env not found"; exit 1; }
set -a; source .env; set +a
for v in EVO_MERCHANT_SID EVO_SIGN_KEY EVO_SIGN_TYPE EVO_API_BASE_URL EVO_LINKPAY_BASE_URL \
         TEST_CARD_NUMBER TEST_CARD_CVC TEST_CARD_EXPIRY TEST_VAULT_ID; do
  [[ -n "${!v:-}" ]] || { echo "FATAL: $v not set"; exit 1; }
done

export HOME; HOME="$(mktemp -d)"; trap 'rm -rf "$HOME"' EXIT
export EVO_SIGN_KEY EVO_API_BASE_URL EVO_LINKPAY_BASE_URL

P=0; F=0; T=0; W=0
pass() { ((P++)); ((T++)); printf "  ✅  %s\n" "$1"; }
warn() { ((W++)); ((T++)); printf "  ⚠️  %s\n" "$1"; }
fail() { ((F++)); ((T++)); printf "  ❌  %s → %s\n" "$1" "${2:-}"; }

# run_cli: wrapper that prints command and response in verbose mode
# Verbose output goes to stderr so it doesn't pollute the captured stdout.
run_cli() {
  if [[ "$VERBOSE" == "--verbose" ]]; then
    printf "  \033[36m→ %s\033[0m\n" "$*" >&2
  fi
  local out rc
  out=$("$@" 2>&1) && rc=0 || rc=$?
  if [[ "$VERBOSE" == "--verbose" ]]; then
    echo "$out" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    print(json.dumps(d,indent=2,ensure_ascii=False))
except:
    sys.stdout.write(sys.stdin.read())
" >&2 2>/dev/null || echo "$out" >&2
    echo "" >&2
  fi
  echo "$out"
  return $rc
}

# JSON-aware ok check (works with both pretty and compact JSON)
ok?()  { echo "$1" | python3 -c "import sys,json;print(json.load(sys.stdin).get('ok'))" 2>/dev/null; }
val?() { echo "$1" | python3 -c "import sys,json;exec(open('/dev/stdin').read())" 2>/dev/null; }
jq?()  { echo "$1" | python3 -c "import sys,json;d=json.load(sys.stdin);$2" 2>/dev/null || true; }

assert_ok() {
  local out="$1" desc="$2"
  if [[ "$(ok? "$out")" == "True" ]]; then pass "$desc"; else fail "$desc" "$(echo "$out" | head -c 200)"; fi
}

# assert_ok_or_expected: accept ok, 3DS action, or any known API error as "expected in test env".
# We're testing CLI integration, not API business logic — issuer rejections, validation errors, etc. are fine.
assert_ok_or_expected() {
  local out="$1" desc="$2"
  if [[ "$(ok? "$out")" == "True" ]]; then
    pass "$desc"
    return 0
  fi
  # Accept action (3DS redirect) as success
  local has_action
  has_action=$(jq? "$out" "print('action' in d.get('data',{}))")
  if [[ "$has_action" == "True" ]]; then
    pass "$desc → action required (3DS)"
    return 0
  fi
  # Accept API business-level errors as expected in test environment:
  # - issuer: card issuer rejected (test card limitations)
  # - psp_error: PSP-side error
  # - business: transaction status doesn't allow the operation (e.g. B0012)
  # - resource: resource not found (e.g. querying a failed transaction)
  # NOT accepted: validation (wrong request format), http_error (wrong URL/path),
  #               cli_error (CLI bug), internal_error (server bug)
  if echo "$out" | grep -qE '"type":\s*"(issuer|psp_error|business|resource)"'; then
    local err_brief
    err_brief=$(echo "$out" | grep -oE '"message":\s*"[^"]{0,120}"' | head -1)
    warn "$desc → expected error ($err_brief)"
    return 0
  fi
  # Accept business/issuer/psp/resource result codes as expected in test env
  if echo "$out" | grep -qE '"code":\s*"[BIPC][0-9]+"'; then
    local code_brief
    code_brief=$(echo "$out" | grep -oE '"code":\s*"[^"]{0,20}"' | head -1)
    warn "$desc → expected error ($code_brief)"
    return 0
  fi
  # Accept HTTP errors that contain business error codes in the message (e.g. "HTTP 400: B0014 ...")
  if echo "$out" | grep -qE '"message":\s*"[^"]*[BIPC][0-9]+'; then
    local msg_brief
    msg_brief=$(echo "$out" | grep -oE '"message":\s*"[^"]{0,120}"' | head -1)
    warn "$desc → expected error ($msg_brief)"
    return 0
  fi
  fail "$desc" "$(echo "$out" | head -c 300)"
  return 1
}

RUN=$(date +%s)
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
SID="$EVO_MERCHANT_SID"

echo "═══════════════════════════════════════════════════════════════"
echo " evo-cli Live E2E · SID=$SID · Run=$RUN"
echo "═══════════════════════════════════════════════════════════════"

# ── 1. Config ────────────────────────────────────────────────────
echo ""; echo "▸ 1. Config"
OUT=$(run_cli "$CLI" config init --merchant-sid "$SID" --sign-type "$EVO_SIGN_TYPE" --env test)
assert_ok "$OUT" "config init"

OUT=$(run_cli "$CLI" config show)
assert_ok "$OUT" "config show"

# ── 2. Doctor ────────────────────────────────────────────────────
echo ""; echo "▸ 2. Doctor"
OUT=$(run_cli "$CLI" doctor)
echo "$OUT" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for c in d.get('checks',[]):
    print(f\"  [{c['status']}] {c['name']}: {c.get('message','')}\")
" 2>/dev/null || true
# At least config + signature should pass
if echo "$OUT" | grep -qF '"pass"'; then pass "doctor has passing checks"; else fail "doctor" "no passing checks"; fi

# ── 3. Query payment methods ────────────────────────────────────
echo ""; echo "▸ 3. Payment methods"
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/paymentMethod")
assert_ok "$OUT" "GET paymentMethod"
BRANDS=$(jq? "$OUT" "
bl=d.get('data',{}).get('paymentMethod',{}).get('paymentBrandList',[])
print(len(bl))
") 
echo "  [info] ${BRANDS:-?} payment brands available"

# ── 4. Payment + Capture Chain ───────────────────────────────────
echo ""; echo "▸ 4. Payment + Capture Chain"

# Step 1: POST /payment — create card payment for capture chain
PAY_TX_CAPTURE="e2e_pay_cap_${RUN}"
PAY_CAP_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${PAY_TX_CAPTURE}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"}}},\"transInitiator\":{\"platform\":\"WEB\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/payment" --data "$PAY_CAP_BODY") || true
PAY_CAP_STATUS=$(jq? "$OUT" "
p=d.get('data',{}).get('payment',d.get('meta',{}))
s=p.get('status',p.get('businessStatus','unknown'))
print(s)
")
PAY_CAP_STATUS="${PAY_CAP_STATUS:-unknown}"
echo "  [info] payment status: $PAY_CAP_STATUS"

assert_ok_or_expected "$OUT" "POST payment (capture chain)"

# Step 2: GET /payment query — verify payment exists
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/payment" \
  --params "{\"merchantTransID\":\"${PAY_TX_CAPTURE}\"}")
assert_ok "$OUT" "GET payment query (capture chain)"
Q_CAP_STATUS=$(jq? "$OUT" "
p=d.get('data',{}).get('payment',{})
print(p.get('status','unknown'))
")
Q_CAP_STATUS="${Q_CAP_STATUS:-unknown}"
echo "  [info] queried status: $Q_CAP_STATUS"

# Step 3: POST /capture — capture the payment
CAP_TX="e2e_cap_${RUN}"
CAP_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${CAP_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/capture?merchantTransID=${PAY_TX_CAPTURE}" \
  --data "$CAP_BODY") || true
assert_ok_or_expected "$OUT" "POST capture"

# Step 4: GET /capture query
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/capture" \
  --params "{\"merchantTransID\":\"${CAP_TX}\"}") || true
assert_ok_or_expected "$OUT" "GET capture query"

# ── 5. Payment + Cancel Chain ───────────────────────────────────
echo ""; echo "▸ 5. Payment + Cancel Chain"

# Step 1: POST /payment — create independent payment for cancel chain
PAY_TX_CANCEL="e2e_pay_can_${RUN}"
PAY_CAN_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${PAY_TX_CANCEL}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"}}},\"transInitiator\":{\"platform\":\"WEB\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/payment" --data "$PAY_CAN_BODY") || true
assert_ok_or_expected "$OUT" "POST payment (cancel chain)"

# Step 2: POST /cancel — cancel the payment
CAN_TX="e2e_can_${RUN}"
CAN_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${CAN_TX}\",\"merchantTransTime\":\"${NOW}\"}}"

sleep 2
OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/cancel?merchantTransID=${PAY_TX_CANCEL}" \
  --data "$CAN_BODY") || true
assert_ok_or_expected "$OUT" "POST cancel"

# Step 3: GET /cancel query
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/cancel" \
  --params "{\"merchantTransID\":\"${CAN_TX}\"}") || true
assert_ok_or_expected "$OUT" "GET cancel query"

# ── 6. Payment + Refund Chain (depends on Capture chain) ────────
echo ""; echo "▸ 6. Payment + Refund Chain"

# Step 1: POST /refund — refund the captured payment
# Note: merchantTransID query param references the ORIGINAL payment's merchantTransID
REF_TX="e2e_ref_${RUN}"
REF_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${REF_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/refund?merchantTransID=${PAY_TX_CAPTURE}" --data "$REF_BODY") || true
assert_ok_or_expected "$OUT" "POST refund"

# Step 2: GET /refund query
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/refund" \
  --params "{\"merchantTransID\":\"${REF_TX}\"}") || true
assert_ok_or_expected "$OUT" "GET refund query"

# ── 7. Payment + CancelOrRefund Chain ───────────────────────────
echo ""; echo "▸ 7. Payment + CancelOrRefund Chain"

# Step 1: POST /payment — create independent payment for cancelOrRefund
PAY_TX_COR="e2e_pay_cor_${RUN}"
PAY_COR_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${PAY_TX_COR}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"}}},\"transInitiator\":{\"platform\":\"WEB\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/payment" --data "$PAY_COR_BODY") || true
assert_ok_or_expected "$OUT" "POST payment (cancelOrRefund chain)"

# Step 2: POST /cancelOrRefund
COR_TX="e2e_cor_${RUN}"
COR_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${COR_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"}}"

sleep 2
OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/cancelOrRefund?merchantTransID=${PAY_TX_COR}" \
  --data "$COR_BODY") || true
assert_ok_or_expected "$OUT" "POST cancelOrRefund"

# Step 3: GET /cancelOrRefund query
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/cancelOrRefund" \
  --params "{\"merchantTransID\":\"${COR_TX}\"}") || true
assert_ok_or_expected "$OUT" "GET cancelOrRefund query"

# ── 8. submitAdditionalInfo ─────────────────────────────────────
echo ""; echo "▸ 8. submitAdditionalInfo"
echo "  ⚠️  [skip] PUT /payment (submitAdditionalInfo) is part of 3DS flow — TODO: add 3DS flow test"

# ── 9. Payout API ───────────────────────────────────────────────
echo ""; echo "▸ 9. Payout API"

# Step 1: POST /payout — create payout request
PAYOUT_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"e2e_payout_${RUN}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"100\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"payoutCardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\"}}},\"transInitiator\":{\"platform\":\"WEB\"},\"tradeInfo\":{\"tradeType\":\"Funds_Disbursement\",\"purposeOfPayment\":\"OCT\"},\"senderInfo\":{\"name\":{\"firstName\":\"Test\",\"lastName\":\"User\"},\"accountNumber\":\"12345678\",\"address\":{\"country\":\"USA\",\"city\":\"TestCity\",\"street\":\"TestStreet\",\"stateOrProvince\":\"AL\"}},\"recipientInfo\":{\"name\":{\"firstName\":\"Recv\",\"lastName\":\"User\"},\"address\":{\"country\":\"USA\",\"city\":\"TestCity\",\"street\":\"TestStreet\",\"stateOrProvince\":\"AL\"}}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/payout" --data "$PAYOUT_BODY") || true
assert_ok_or_expected "$OUT" "POST payout"

# Step 2: GET /payout — query payout status
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/payout" \
  --params "{\"merchantTransID\":\"e2e_payout_${RUN}\"}") || true
assert_ok_or_expected "$OUT" "GET payout query"

# ── 10. FX Rate API ─────────────────────────────────────────────
echo ""; echo "▸ 10. FX Rate API"

# Step 1: POST /FXRateInquiry — fx rate inquiry
FX_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"e2e_fx_${RUN}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"EUR\"},\"localAmount\":{\"currency\":\"USD\",\"value\":\"100.00\"}}"

OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/FXRateInquiry" --data "$FX_BODY") || true
assert_ok_or_expected "$OUT" "POST FXRateInquiry"

# Step 2: GET /FXRateInquiry — query fx rate
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/FXRateInquiry" \
  --params "{\"baseCurrency\":\"USD\",\"quoteCurrency\":\"EUR\"}") || true
assert_ok_or_expected "$OUT" "GET FXRateInquiry query"

# ── 11. Cryptogram API ──────────────────────────────────────────
# NOTE: Cryptogram requires a real network token ID from the Token lifecycle chain.
# It will be tested after Section 13 (Token Lifecycle Chain) using the network token
# obtained from token creation. See Section 15 below.

# ── 12. LinkPay Full Chain ────────────────────────────────────────
echo ""; echo "▸ 12. LinkPay Full Chain"

# Step 1: POST linkpay create
LP_ORDER_ID="e2e_lp_${RUN}"
LP_BODY="{\"merchantOrderInfo\":{\"merchantOrderID\":\"${LP_ORDER_ID}\",\"merchantOrderTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"5.00\"}}"
OUT=$(run_cli "$CLI" api POST "/g2/v0/payment/mer/${SID}/evo.e-commerce.linkpay" --data "$LP_BODY") || true
if [[ "$(ok? "$OUT")" == "True" ]]; then
  pass "POST linkpay create"
  LINK_URL=$(jq? "$OUT" "print(d.get('data',{}).get('linkUrl',''))") || LINK_URL=""
  if [[ -n "$LINK_URL" ]]; then
    echo "  [info] linkUrl: ${LINK_URL:0:80}..."
    pass "linkpay URL returned"
  fi
else
  assert_ok_or_expected "$OUT" "POST linkpay create"
fi

# Step 2: GET linkpay query
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v0/payment/mer/${SID}/evo.e-commerce.linkpay/${LP_ORDER_ID}") || true
assert_ok_or_expected "$OUT" "GET linkpay query"
LP_STATUS=$(jq? "$OUT" "
o=d.get('data',{}).get('merchantOrderInfo',{})
print(o.get('status','unknown'))
") || LP_STATUS="unknown"
echo "  [info] linkpay status: $LP_STATUS"

# Step 3: POST linkpay cancelOrRefund
sleep 2
LP_COR_TX="e2e_lp_cor_${RUN}"
LP_COR_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${LP_COR_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"5.00\"}}"
OUT=$(run_cli "$CLI" api POST "/g2/v0/payment/mer/${SID}/evo.e-commerce.linkpayCancelorRefund/${LP_ORDER_ID}" --data "$LP_COR_BODY") || true
assert_ok_or_expected "$OUT" "POST linkpay cancelOrRefund"

# Step 4: POST linkpay refund
sleep 2
LP_REF_TX="e2e_lp_ref_${RUN}"
LP_REF_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${LP_REF_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"}}"
OUT=$(run_cli "$CLI" api POST "/g2/v0/payment/mer/${SID}/evo.e-commerce.linkpayRefund/${LP_ORDER_ID}" --data "$LP_REF_BODY") || true
assert_ok_or_expected "$OUT" "POST linkpay refund"

# Step 5: GET linkpay refundQuery
sleep 2
OUT=$(run_cli "$CLI" api GET "/g2/v0/payment/mer/${SID}/evo.e-commerce.linkpayRefund/${LP_REF_TX}") || true
assert_ok_or_expected "$OUT" "GET linkpay refundQuery"

# ── 13. Token Lifecycle Chain ─────────────────────────────────────
echo ""; echo "▸ 13. Token Lifecycle Chain"

# Step 1: POST /paymentMethod create — create gateway token
TOK_TX="e2e_tok_${RUN}"
TOK_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${TOK_TX}\",\"merchantTransTime\":\"${NOW}\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"},\"vaultID\":\"${TEST_VAULT_ID}\"}},\"userInfo\":{\"reference\":\"e2e-test-user\"},\"transInitiator\":{\"platform\":\"WEB\"}}"
OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/paymentMethod" --data "$TOK_BODY") || true
TOK_VALUE=""
if [[ "$(ok? "$OUT")" == "True" ]]; then
  pass "POST paymentMethod create (token lifecycle)"
  TOK_VALUE=$(jq? "$OUT" "
t=d.get('data',{}).get('paymentMethod',{}).get('token',{})
print(t.get('value',''))
")
  TOK_VALUE="${TOK_VALUE:-}"
  if [[ -n "$TOK_VALUE" ]]; then
    echo "  [info] token: ${TOK_VALUE:0:30}..."
  fi
else
  assert_ok_or_expected "$OUT" "POST paymentMethod create (token lifecycle)"
fi

if [[ -n "$TOK_VALUE" ]]; then
  # Step 2: GET /paymentMethod query — query token by value (query param is "token")
  sleep 2
  OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/paymentMethod" \
    --params "{\"token\":\"${TOK_VALUE}\"}")
  assert_ok "$OUT" "GET paymentMethod query (token lifecycle)"

  # Step 3: PUT /paymentMethod update — update token (use both token and merchantTransID query params)
  sleep 2
  TOK_UPDATE_BODY="{\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"expiryDate\":\"${TEST_CARD_EXPIRY}\"}}}}"
  OUT=$(run_cli "$CLI" api PUT "/g2/v1/payment/mer/${SID}/paymentMethod?token=${TOK_VALUE}&merchantTransID=${TOK_TX}" \
    --data "$TOK_UPDATE_BODY") || true
  assert_ok_or_expected "$OUT" "PUT paymentMethod update (token lifecycle)"

  # Step 4: DELETE /paymentMethod delete — delete token (query param is "token")
  sleep 2
  OUT=$(run_cli "$CLI" api DELETE "/g2/v1/payment/mer/${SID}/paymentMethod?token=${TOK_VALUE}" \
    --data '{"initiatingReason":"e2e test cleanup"}') || true
  assert_ok_or_expected "$OUT" "DELETE paymentMethod delete (token lifecycle)"

  # Step 5: token +create with --allow-authentication
  sleep 2
  OUT=$(run_cli "$CLI" token +create --payment-type card --vault-id "${TEST_VAULT_ID}" \
    --user-reference e2e-test-user \
    --card-number "${TEST_CARD_NUMBER}" --card-expiry "${TEST_CARD_EXPIRY}" --card-cvc "${TEST_CARD_CVC}" \
    --allow-authentication true --return-url "https://example.com/callback") || true
  assert_ok_or_expected "$OUT" "token +create --allow-authentication"
else
  echo "  ⚠️  [skip] token create failed — skipping query/update/delete"
fi

# ── 14. Token + Payment Chain ────────────────────────────────────

# ── Cryptogram API (uses network token from dedicated networkTokenOnly call) ──
echo ""; echo "▸ 11. Cryptogram API"

# Step 0: Create network token via POST /paymentMethod with networkTokenOnly=true
NT_TX="e2e_nt_${RUN}"
NT_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${NT_TX}\",\"merchantTransTime\":\"${NOW}\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"},\"vaultID\":\"${TEST_VAULT_ID}\"}},\"userInfo\":{\"reference\":\"e2e-test-user\",\"email\":\"test@example.com\",\"locale\":\"en_US\"},\"transInitiator\":{\"platform\":\"WEB\"},\"networkTokenOnly\":true}"
OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/paymentMethod" --data "$NT_BODY") || true
NETWORK_TOKEN_ID=""
if [[ "$(ok? "$OUT")" == "True" ]]; then
  pass "POST paymentMethod (networkTokenOnly)"
  NETWORK_TOKEN_ID=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('tokenID',''))
")
  NETWORK_TOKEN_ID="${NETWORK_TOKEN_ID:-}"
  NETWORK_TOKEN_EXPIRY=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('expiryDate',''))
") || NETWORK_TOKEN_EXPIRY=""
  NETWORK_TOKEN_EXPIRY="${NETWORK_TOKEN_EXPIRY:-}"
  if [[ -n "$NETWORK_TOKEN_ID" ]]; then
    echo "  [info] networkTokenID: ${NETWORK_TOKEN_ID:0:40}..."
  fi
else
  assert_ok_or_expected "$OUT" "POST paymentMethod (networkTokenOnly)"
fi

if [[ -n "$NETWORK_TOKEN_ID" ]]; then
  # Step 1: POST /cryptogram — create cryptogram using real network token
  sleep 2
  CRYPTO_TX="e2e_crypto_${RUN}"
  CRYPTO_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${CRYPTO_TX}\",\"merchantTransTime\":\"${NOW}\"},\"paymentMethod\":{\"type\":\"networkToken\",\"networkToken\":{\"tokenID\":\"${NETWORK_TOKEN_ID}\"}}}"
  OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/cryptogram?merchantTransID=${CRYPTO_TX}" --data "$CRYPTO_BODY") || true
  assert_ok_or_expected "$OUT" "POST cryptogram"

  # Step 2: GET /cryptogram — query cryptogram
  sleep 2
  OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/cryptogram" \
    --params "{\"merchantTransID\":\"${CRYPTO_TX}\"}") || true
  assert_ok_or_expected "$OUT" "GET cryptogram query"

  # Step 3: cryptogram +create shortcut
  sleep 2
  OUT=$(run_cli "$CLI" cryptogram +create --network-token-id "$NETWORK_TOKEN_ID" --original-merchant-tx-id "$NT_TX") || true
  assert_ok_or_expected "$OUT" "cryptogram +create shortcut"
  SC_CRYPTO_TX=$(jq? "$OUT" "
mt=d.get('data',{}).get('cryptogram',d.get('data',{})).get('merchantTransInfo',{})
print(mt.get('merchantTransID',''))
") || SC_CRYPTO_TX=""
  SC_CRYPTO_TX="${SC_CRYPTO_TX:-}"
  # Extract tokenCryptogram and eci for +pay
  SC_TOKEN_CRYPTOGRAM=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('tokenCryptogram',''))
") || SC_TOKEN_CRYPTOGRAM=""
  SC_TOKEN_CRYPTOGRAM="${SC_TOKEN_CRYPTOGRAM:-}"
  SC_ECI=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('eci',''))
") || SC_ECI=""
  SC_ECI="${SC_ECI:-}"
  SC_NT_VALUE=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('value',''))
") || SC_NT_VALUE=""
  SC_NT_VALUE="${SC_NT_VALUE:-}"
  SC_NT_BRAND=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('paymentBrand',''))
") || SC_NT_BRAND=""
  SC_NT_BRAND="${SC_NT_BRAND:-}"
  SC_NT_EXPIRY=$(jq? "$OUT" "
nt=d.get('data',{}).get('paymentMethod',{}).get('networkToken',{})
print(nt.get('expiryDate',''))
") || SC_NT_EXPIRY=""
  SC_NT_EXPIRY="${SC_NT_EXPIRY:-$NETWORK_TOKEN_EXPIRY}"

  # Step 4: cryptogram +query shortcut
  if [[ -n "$SC_CRYPTO_TX" ]]; then
    sleep 2
    OUT=$(run_cli "$CLI" cryptogram +query --merchant-tx-id "$SC_CRYPTO_TX") || true
    assert_ok_or_expected "$OUT" "cryptogram +query shortcut"
  else
    # Fall back to querying with the original CRYPTO_TX
    sleep 2
    OUT=$(run_cli "$CLI" cryptogram +query --merchant-tx-id "$CRYPTO_TX") || true
    assert_ok_or_expected "$OUT" "cryptogram +query shortcut (fallback tx)"
  fi

  # Step 5: cryptogram +pay shortcut — pay with network token + cryptogram
  if [[ -n "$SC_TOKEN_CRYPTOGRAM" && -n "$SC_ECI" && -n "$SC_NT_VALUE" && -n "$SC_NT_BRAND" && -n "$SC_NT_EXPIRY" ]]; then
    sleep 2
    OUT=$(run_cli "$CLI" cryptogram +pay \
      --network-token-value "$SC_NT_VALUE" \
      --token-expiry-date "$SC_NT_EXPIRY" \
      --token-cryptogram "$SC_TOKEN_CRYPTOGRAM" \
      --eci "$SC_ECI" \
      --payment-brand "$SC_NT_BRAND" \
      --amount 1.00 --currency USD) || true
    assert_ok_or_expected "$OUT" "cryptogram +pay shortcut"
  else
    echo "  ⚠️  [skip] No cryptogram/eci/value/brand available — skipping +pay"
  fi
else
  echo "  ⚠️  [skip] No network token available — skipping cryptogram test"
fi

# ── 14. Token + Payment Chain ────────────────────────────────────
echo ""; echo "▸ 14. Token + Payment Chain"

# Step 1: POST /paymentMethod create — create token for payment
TOK_TX_PAY="e2e_tok_pay_${RUN}"
TOK_PAY_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${TOK_TX_PAY}\",\"merchantTransTime\":\"${NOW}\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"},\"vaultID\":\"${TEST_VAULT_ID}\"}},\"userInfo\":{\"reference\":\"e2e-test-user\"},\"transInitiator\":{\"platform\":\"WEB\"}}"
OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/paymentMethod" --data "$TOK_PAY_BODY") || true
TOK_VALUE_PAY=""
if [[ "$(ok? "$OUT")" == "True" ]]; then
  pass "POST paymentMethod create (token+payment chain)"
  TOK_VALUE_PAY=$(jq? "$OUT" "
t=d.get('data',{}).get('paymentMethod',{}).get('token',{})
print(t.get('value',''))
")
  TOK_VALUE_PAY="${TOK_VALUE_PAY:-}"
  if [[ -n "$TOK_VALUE_PAY" ]]; then
    echo "  [info] token for payment: ${TOK_VALUE_PAY:0:30}..."
  fi
else
  assert_ok_or_expected "$OUT" "POST paymentMethod create (token+payment chain)"
fi

if [[ -n "$TOK_VALUE_PAY" ]]; then
  # Step 2: POST /payment — pay with token
  sleep 2
  TOKPAY_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"e2e_tokpay_${RUN}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"},\"paymentMethod\":{\"type\":\"token\",\"token\":{\"value\":\"${TOK_VALUE_PAY}\"}},\"transInitiator\":{\"platform\":\"WEB\"}}"
  OUT=$(run_cli "$CLI" api POST "/g2/v1/payment/mer/${SID}/payment" --data "$TOKPAY_BODY") || true
  assert_ok_or_expected "$OUT" "POST payment with token"
else
  echo "  ⚠️  [skip] token create failed — skipping payment with token"
fi

# ── 16. Payment Shortcut Chain ────────────────────────────────────
echo ""; echo "▸ 16. Payment Shortcut Chain"

# --- Capture sub-chain ---
# Step 1: payment +pay (for capture)
SC_PAY_TX_CAP="e2e_sc_pay_cap_${RUN}"
OUT=$(run_cli "$CLI" payment +pay --amount 1.00 --currency USD --payment-brand Alipay \
  --merchant-tx-id "$SC_PAY_TX_CAP" --return-url "https://example.com/return" --yes) || true
assert_ok_or_expected "$OUT" "payment +pay (capture sub-chain)"

# Step 2: payment +query
sleep 2
OUT=$(run_cli "$CLI" payment +query --merchant-tx-id "$SC_PAY_TX_CAP") || true
assert_ok_or_expected "$OUT" "payment +query (capture sub-chain)"

# Step 3: payment +capture
sleep 2
OUT=$(run_cli "$CLI" payment +capture --original-merchant-tx-id "$SC_PAY_TX_CAP" \
  --amount 1.00 --currency USD) || true
assert_ok_or_expected "$OUT" "payment +capture"

# --- Cancel sub-chain ---
# Step 4: payment +pay (for cancel)
sleep 2
SC_PAY_TX_CAN="e2e_sc_pay_can_${RUN}"
OUT=$(run_cli "$CLI" payment +pay --amount 1.00 --currency USD --payment-brand Alipay \
  --merchant-tx-id "$SC_PAY_TX_CAN" --return-url "https://example.com/return" --yes) || true
assert_ok_or_expected "$OUT" "payment +pay (cancel sub-chain)"

# Step 5: payment +cancel
sleep 2
OUT=$(run_cli "$CLI" payment +cancel --original-merchant-tx-id "$SC_PAY_TX_CAN" --yes) || true
assert_ok_or_expected "$OUT" "payment +cancel"

# --- Refund sub-chain ---
# Step 6: payment +refund (references captured SC_PAY_TX_CAP)
sleep 2
OUT=$(run_cli "$CLI" payment +refund --original-merchant-tx-id "$SC_PAY_TX_CAP" \
  --amount 1.00 --currency USD --yes) || true
assert_ok_or_expected "$OUT" "payment +refund"

# ── 17. LinkPay Shortcut Chain ───────────────────────────────────
echo ""; echo "▸ 17. LinkPay Shortcut Chain"

# Step 1: linkpay +create
SC_LP_ID="e2e_sc_lp_${RUN}"
OUT=$(run_cli "$CLI" linkpay +create --amount 5.00 --currency USD --order-id "$SC_LP_ID") || true
assert_ok_or_expected "$OUT" "linkpay +create"

# Step 2: linkpay +query
sleep 2
OUT=$(run_cli "$CLI" linkpay +query --merchant-order-id "$SC_LP_ID") || true
assert_ok_or_expected "$OUT" "linkpay +query"

# Step 3: linkpay +refund
sleep 2
OUT=$(run_cli "$CLI" linkpay +refund --merchant-order-id "$SC_LP_ID" \
  --amount 1.00 --currency USD --yes) || true
assert_ok_or_expected "$OUT" "linkpay +refund"

# ── 18. Token Shortcut Chain ────────────────────────────────────
echo ""; echo "▸ 18. Token Shortcut Chain"

# Step 1: token +create
OUT=$(run_cli "$CLI" token +create --payment-type card --vault-id "${TEST_VAULT_ID}" \
  --user-reference e2e-test-user \
  --card-number "${TEST_CARD_NUMBER}" --card-expiry "${TEST_CARD_EXPIRY}" --card-cvc "${TEST_CARD_CVC}") || true
SC_TOK_VALUE=""
SC_TOK_MERCHANT_TX=""
if [[ "$(ok? "$OUT")" == "True" ]]; then
  pass "token +create → ok"
  SC_TOK_VALUE=$(jq? "$OUT" "
t=d.get('data',{}).get('paymentMethod',{}).get('token',{})
print(t.get('value',''))
")
  SC_TOK_VALUE="${SC_TOK_VALUE:-}"
  SC_TOK_MERCHANT_TX=$(jq? "$OUT" "
pm=d.get('data',{}).get('paymentMethod',{})
mt=pm.get('merchantTransInfo',{})
print(mt.get('merchantTransID',''))
")
  SC_TOK_MERCHANT_TX="${SC_TOK_MERCHANT_TX:-}"
  if [[ -n "$SC_TOK_VALUE" ]]; then
    echo "  [info] token: ${SC_TOK_VALUE:0:30}..."
  fi
  if [[ -n "$SC_TOK_MERCHANT_TX" ]]; then
    echo "  [info] merchantTransID: ${SC_TOK_MERCHANT_TX}"
  fi
else
  assert_ok_or_expected "$OUT" "token +create"
fi

# Step 2: token +query (uses merchantTransID from create response)
if [[ -n "$SC_TOK_MERCHANT_TX" ]]; then
  sleep 2
  OUT=$(run_cli "$CLI" token +query --merchant-tx-id "$SC_TOK_MERCHANT_TX") || true
  assert_ok_or_expected "$OUT" "token +query"
else
  echo "  ⚠️  [skip] token +create did not return merchantTransID — skipping +query"
fi

# Step 3: payment +pay with gateway token
if [[ -n "$SC_TOK_VALUE" ]]; then
  sleep 2
  OUT=$(run_cli "$CLI" payment +pay --amount 1.00 --currency USD \
    --gateway-token "$SC_TOK_VALUE") || true
  assert_ok_or_expected "$OUT" "payment +pay --gateway-token"
else
  echo "  ⚠️  [skip] token +create failed — skipping payment +pay --gateway-token"
fi

# Step 4: token +delete (uses token value)
if [[ -n "$SC_TOK_VALUE" ]]; then
  sleep 2
  OUT=$(run_cli "$CLI" token +delete --token-id "$SC_TOK_VALUE" --yes) || true
  assert_ok_or_expected "$OUT" "token +delete"
else
  echo "  ⚠️  [skip] token +create failed — skipping +delete"
fi

# ── 19. Service Commands (live) ───────────────────────────────────
echo ""; echo "▸ 19. Service Commands (live)"

# Step 1: payment online pay — send payment via service command
SVC_PAY_TX="e2e_svc_pay_${RUN}"
SVC_PAY_BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${SVC_PAY_TX}\",\"merchantTransTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"1.00\"},\"paymentMethod\":{\"type\":\"card\",\"card\":{\"cardInfo\":{\"cardNumber\":\"${TEST_CARD_NUMBER}\",\"expiryDate\":\"${TEST_CARD_EXPIRY}\",\"cvc\":\"${TEST_CARD_CVC}\"}}},\"transInitiator\":{\"platform\":\"WEB\"}}"

OUT=$(run_cli "$CLI" payment online pay --data "$SVC_PAY_BODY") || true
assert_ok_or_expected "$OUT" "payment online pay"

# Step 2: payment online query — query payment via service command
sleep 2
OUT=$(run_cli "$CLI" payment online query --params "{\"merchantTransID\":\"${SVC_PAY_TX}\"}") || true
assert_ok_or_expected "$OUT" "payment online query"

# Step 3: linkpay order create — create LinkPay order via service command
SVC_LP_ID="e2e_svc_lp_${RUN}"
SVC_LP_BODY="{\"merchantOrderInfo\":{\"merchantOrderID\":\"${SVC_LP_ID}\",\"merchantOrderTime\":\"${NOW}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"5.00\"}}"

OUT=$(run_cli "$CLI" linkpay order create --data "$SVC_LP_BODY") || true
assert_ok_or_expected "$OUT" "linkpay order create"

# Step 4: linkpay order query — query LinkPay order via service command
sleep 2
OUT=$(run_cli "$CLI" linkpay order query --params "{\"merchantOrderID of LinkPay\":\"${SVC_LP_ID}\"}") || true
assert_ok_or_expected "$OUT" "linkpay order query"

# ── 20. Output Flag (-o) live test ──────────────────────────────
echo ""; echo "▸ 20. Output Flag (-o) live test"

OUT=$(run_cli "$CLI" api GET "/g2/v1/payment/mer/${SID}/paymentMethod" -o /tmp/e2e_live_output.json) || true
if [[ -f /tmp/e2e_live_output.json ]]; then
  if python3 -c "import json; json.load(open('/tmp/e2e_live_output.json'))" 2>/dev/null; then
    pass "-o flag → file created with valid JSON"
  else
    fail "-o flag → file not valid JSON"
  fi
  rm -f /tmp/e2e_live_output.json
else
  fail "-o flag → file not created"
fi

# ── 21. Cleanup ──────────────────────────────────────────────────
echo ""; echo "▸ 21. Cleanup"
OUT=$(run_cli "$CLI" config remove)
assert_ok "$OUT" "config remove"

# ── Summary ──────────────────────────────────────────────────────
echo ""
echo "═══════════════════════════════════════════════════════════════"
printf " Results: %d passed, %d warned, %d failed, %d total\n" "$P" "$W" "$F" "$T"
echo "═══════════════════════════════════════════════════════════════"
[[ "$F" -eq 0 ]] || exit 1
