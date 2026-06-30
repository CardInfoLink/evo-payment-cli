#!/bin/bash
# Test: Pay with Network Token + Cryptogram via raw API call
# Based on guide-network-token-cryptogram-payment-zh.md

set -e

SID="S005898"
SIGN_KEY="sk_sandbox_4724d7d7713645549754c64132bc8148"
BASE_URL="https://hkg-online-uat.everonet.com"
PATH_URL="/g2/v1/payment/mer/${SID}/payment"

# Generate unique IDs
MERCHANT_TX_ID="test_ntpay_$(date +%s)"
MSG_ID=$(uuidgen | tr -d '-' | tr '[:upper:]' '[:lower:]' | cut -c1-32)
DT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Request body
BODY="{\"merchantTransInfo\":{\"merchantTransID\":\"${MERCHANT_TX_ID}\",\"merchantTransTime\":\"${DT}\"},\"transAmount\":{\"currency\":\"USD\",\"value\":\"10.00\"},\"paymentMethod\":{\"type\":\"token\",\"token\":{\"type\":\"networkToken\",\"value\":\"5123456789012345\",\"expiryDate\":\"1230\",\"tokenCryptogram\":\"ABCDEFGHIJKLMN==\",\"eci\":\"05\",\"paymentBrand\":\"Mastercard\",\"walletIdentifiers\":\"MDESForMerchants\"}},\"transInitiator\":{\"platform\":\"WEB\"}}"

# Signature: SHA256(method\npath\nDateTime\nkey\nMsgID\nbody)
SIGN_STRING="POST
${PATH_URL}
${DT}
${SIGN_KEY}
${MSG_ID}
${BODY}"

SIGNATURE=$(printf '%s' "$SIGN_STRING" | openssl dgst -sha256 | awk '{print $NF}')

echo "============================================================"
echo "REQUEST"
echo "============================================================"
echo "URL:             ${BASE_URL}${PATH_URL}"
echo "DateTime:        ${DT}"
echo "MsgID:           ${MSG_ID}"
echo "MerchantTransID: ${MERCHANT_TX_ID}"
echo "SignType:         SHA256"
echo "Authorization:   ${SIGNATURE}"
echo ""
echo "Body (formatted):"
echo "$BODY" | python3 -m json.tool 2>/dev/null || echo "$BODY"
echo ""
echo "============================================================"
echo "RESPONSE"
echo "============================================================"

curl -s -w "\n\nHTTP_STATUS: %{http_code}\n" \
  -X POST "${BASE_URL}${PATH_URL}" \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "DateTime: ${DT}" \
  -H "MsgID: ${MSG_ID}" \
  -H "SignType: SHA256" \
  -H "Authorization: ${SIGNATURE}" \
  --connect-timeout 10 \
  --max-time 20 \
  -d "${BODY}" | python3 -m json.tool 2>/dev/null || true
