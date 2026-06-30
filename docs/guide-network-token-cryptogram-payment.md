# Pay with Network Token + Cryptogram

This guide explains how to use the Evo Payment API to make a payment using a **Network Token** combined with a **Cryptogram**. This is a three-step process:

1. **Tokenize** a card to obtain a Network Token
2. **Generate a Cryptogram** from the Network Token
3. **Submit Payment** using the Network Token value + Cryptogram

## Prerequisites

- A valid Evo Payment merchant account (Store ID / `sid`)
- A signature key assigned by Evo Payment
- A Vault ID for token storage
- HTTPS client supporting TLS v1.2+

## Overview Flow

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│  Step 1      │      │  Step 2      │      │  Step 3      │
│  POST        │─────▶│  POST        │─────▶│  POST        │
│  /paymentMethod│     │  /cryptogram │      │  /payment    │
│              │      │              │      │              │
│  Get Network │      │  Get         │      │  Pay with    │
│  Token ID    │      │  Cryptogram  │      │  Token+Crypto│
└──────────────┘      └──────────────┘      └──────────────┘
```

## Common HTTP Headers

All API requests require these headers:

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json; charset=utf-8` |
| `DateTime` | ISO 8601 timestamp, e.g. `2025-06-28T12:00:00+08:00` |
| `MsgID` | Unique trace ID (UUID recommended, max 32 chars) |
| `SignType` | `SHA256`, `SHA512`, `HMAC-SHA256`, or `HMAC-SHA512` |
| `Authorization` | Message signature value (see [Signing](#message-signing)) |

## Message Signing

Each request must be signed. Construct a signing string with these lines (each ending with `\n`, except the last):

```
{HTTP_METHOD}
{REQUEST_PATH_WITH_QUERY}
{DateTime_HEADER_VALUE}
{SIGNATURE_KEY}
{MsgID_HEADER_VALUE}
{HTTP_BODY}
```

Then compute SHA256 (or your chosen algorithm) over this string. Place the hex result in the `Authorization` header.

---

## Step 1: Create Network Token

Tokenize a card with `networkTokenOnly=true` to get a Network Token from the card scheme (Visa/Mastercard).

### Request

```
POST /g2/v1/payment/mer/{sid}/paymentMethod
```

### Request Body

```json
{
  "merchantTransInfo": {
    "merchantTransID": "tok_unique_id_001",
    "merchantTransTime": "2025-06-28T12:00:00Z"
  },
  "paymentMethod": {
    "type": "card",
    "card": {
      "cardInfo": {
        "cardNumber": "5120350100064594",
        "expiryDate": "2810",
        "cvc": "123"
      },
      "vaultID": "90451900"
    }
  },
  "userInfo": {
    "reference": "user-ref-001",
    "email": "user@example.com",
    "locale": "en_US"
  },
  "transInitiator": {
    "platform": "WEB"
  },
  "networkTokenOnly": true
}
```

### Key Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantTransInfo.merchantTransID` | String(32) | Yes | Unique transaction ID |
| `merchantTransInfo.merchantTransTime` | DateTime | Yes | ISO 8601 timestamp |
| `paymentMethod.type` | String | Yes | Must be `"card"` |
| `paymentMethod.card.cardInfo.cardNumber` | String | Yes | Full card number |
| `paymentMethod.card.cardInfo.expiryDate` | String(4) | Yes | Format: MMYY |
| `paymentMethod.card.cardInfo.cvc` | String | Yes | Card CVC/CVV |
| `paymentMethod.card.vaultID` | String(36) | Yes | Vault ID for token storage |
| `userInfo.reference` | String(64) | Yes | Merchant user identifier |
| `userInfo.email` | String | Yes | Required for network token |
| `networkTokenOnly` | Boolean | Yes | Must be `true` |

### Response (Success)

```json
{
  "result": {
    "code": "S0000",
    "message": "Success"
  },
  "paymentMethod": {
    "networkToken": {
      "tokenID": "NT_abc123def456",
      "value": "5120350199991234",
      "expiryDate": "1228",
      "first6No": "512035",
      "last4No": "1234",
      "paymentBrand": "Mastercard",
      "status": "enabled"
    }
  }
}
```

Save the following from the response for the next steps:
- `paymentMethod.networkToken.tokenID` — needed for Step 2
- `paymentMethod.networkToken.value` — needed for Step 3
- `paymentMethod.networkToken.expiryDate` — needed for Step 3
- `paymentMethod.networkToken.paymentBrand` — needed for Step 3

---

## Step 2: Generate Cryptogram

Request a payment cryptogram from the card network using the Network Token ID.

### Request

```
POST /g2/v1/payment/mer/{sid}/cryptogram?merchantTransID={originalTokenizationTxID}
```

The `merchantTransID` query parameter refers to the `merchantTransID` used in Step 1 (the original tokenization request).

### Request Body

```json
{
  "merchantTransInfo": {
    "merchantTransID": "crypto_unique_id_001",
    "merchantTransTime": "2025-06-28T12:01:00Z"
  },
  "paymentMethod": {
    "type": "networkToken",
    "networkToken": {
      "tokenID": "NT_abc123def456"
    }
  }
}
```

### Key Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantTransInfo.merchantTransID` | String(32) | Yes | New unique ID for this cryptogram request |
| `merchantTransInfo.merchantTransTime` | DateTime | Yes | ISO 8601 timestamp |
| `paymentMethod.type` | String | Yes | Must be `"networkToken"` |
| `paymentMethod.networkToken.tokenID` | String(64) | Yes | Token ID from Step 1 response |

### Response (Success)

```json
{
  "result": {
    "code": "S0000",
    "message": "Success"
  },
  "cryptogram": {
    "status": "Success",
    "merchantTransInfo": {
      "merchantTransID": "crypto_unique_id_001",
      "merchantTransTime": "2025-06-28T12:01:00Z"
    }
  },
  "paymentMethod": {
    "networkToken": {
      "tokenID": "NT_abc123def456",
      "value": "5120350199991234",
      "expiryDate": "1228",
      "paymentBrand": "Mastercard",
      "tokenCryptogram": "AAKQJPQZFWufAAJ5j4JeAAADFA==",
      "eci": "06",
      "status": "enabled"
    }
  }
}
```

Save the following from the response for Step 3:
- `paymentMethod.networkToken.tokenCryptogram` — the cryptogram value
- `paymentMethod.networkToken.eci` — ECI (Electronic Commerce Indicator)
- `paymentMethod.networkToken.value` — network token value (card number form)
- `paymentMethod.networkToken.expiryDate` — token expiry (MMYY)
- `paymentMethod.networkToken.paymentBrand` — payment brand

### Cryptogram Status Values

| Status | Description |
|--------|-------------|
| `Success` | Cryptogram generated successfully |
| `Failed` | Cryptogram generation failed (check `failureCode` and `failureReason`) |
| `Received` | Still processing — poll with GET /cryptogram |

### Query Cryptogram Status (Optional)

If the status is `Received`, poll the result:

```
GET /g2/v1/payment/mer/{sid}/cryptogram?merchantTransID={cryptogramTxID}
```

---

## Step 3: Pay with Network Token + Cryptogram

Submit a payment using the network token value and the cryptogram obtained in Step 2.

### Request

```
POST /g2/v1/payment/mer/{sid}/payment
```

### Request Body

```json
{
  "merchantTransInfo": {
    "merchantTransID": "pay_unique_id_001",
    "merchantTransTime": "2025-06-28T12:02:00Z"
  },
  "transAmount": {
    "currency": "USD",
    "value": "10.00"
  },
  "paymentMethod": {
    "type": "token",
    "token": {
      "type": "networkToken",
      "value": "5120350199991234",
      "expiryDate": "1228",
      "tokenCryptogram": "AAKQJPQZFWufAAJ5j4JeAAADFA==",
      "eci": "06",
      "paymentBrand": "Mastercard",
      "walletIdentifiers": "MDESForMerchants"
    }
  },
  "transInitiator": {
    "platform": "WEB"
  }
}
```

### Key Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantTransInfo.merchantTransID` | String(32) | Yes | Unique ID for this payment |
| `merchantTransInfo.merchantTransTime` | DateTime | Yes | ISO 8601 timestamp |
| `transAmount.currency` | String(3) | Yes | ISO 4217 currency code |
| `transAmount.value` | String(12) | Yes | Amount in major units (e.g. "10.00") |
| `paymentMethod.type` | String | Yes | Must be `"token"` |
| `paymentMethod.token.type` | String | Yes | Must be `"networkToken"` |
| `paymentMethod.token.value` | String(64) | Yes | Network token value from Step 2 |
| `paymentMethod.token.expiryDate` | String(4) | Yes | Token expiry date (MMYY) |
| `paymentMethod.token.tokenCryptogram` | String(64) | Yes | Cryptogram from Step 2 |
| `paymentMethod.token.eci` | String(2) | Yes | ECI from Step 2 |
| `paymentMethod.token.paymentBrand` | String(32) | Yes | `"Visa"` or `"Mastercard"` |
| `paymentMethod.token.walletIdentifiers` | String | No | Default: `"MDESForMerchants"` for Mastercard |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `returnURL` | String(300) | Redirect URL after payment |
| `webhook` | String(300) | Async notification URL |
| `captureAfterHours` | String | Set to `"0"` for auto-capture after authorization |

### Response (Success)

```json
{
  "result": {
    "code": "S0000",
    "message": "Success"
  },
  "payment": {
    "status": "Captured",
    "merchantTransInfo": {
      "merchantTransID": "pay_unique_id_001",
      "merchantTransTime": "2025-06-28T12:02:00Z"
    }
  }
}
```

---

## Wallet Identifiers

The `walletIdentifiers` field identifies the token service provider. Defaults:

| Payment Brand | Default Value |
|---------------|---------------|
| Mastercard | `MDESForMerchants` |
| Visa | (empty — not required) |

---

## Idempotency

POST requests support the `Idempotency-Key` header (max 64 chars). It is recommended to use the `merchantTransID` as the idempotency key to prevent duplicate payments.

```
Idempotency-Key: pay_unique_id_001
```

---

## Error Handling

All responses include a `result` object:

```json
{
  "result": {
    "code": "S0000",
    "message": "Success"
  }
}
```

| Code Pattern | Meaning |
|-------------|---------|
| `S0000` | Success |
| `B****` | Business error |
| `V****` | Validation error |
| `E****` | System/network error |
| `P****` | PSP error |

---

## Using evo-cli

The `evo-cli` tool provides shortcuts for the entire flow:

```bash
# Step 1: Create network token
evo-cli token +create --payment-type card --vault-id 90451900 \
  --user-reference user@example.com --email user@example.com \
  --network-token-only true \
  --card-number 5120350100064594 --card-expiry 2810 --card-cvc 123

# Step 2: Generate cryptogram
evo-cli cryptogram +create \
  --network-token-id <tokenID_from_step1> \
  --original-merchant-tx-id <merchantTransID_from_step1>

# Step 3: Pay with cryptogram
evo-cli cryptogram +pay \
  --network-token-value <value_from_step2> \
  --token-expiry-date <expiryDate_from_step2> \
  --token-cryptogram <tokenCryptogram_from_step2> \
  --eci <eci_from_step2> \
  --payment-brand Mastercard \
  --amount 10.00 --currency USD

# Optional: auto-capture, custom tx ID, webhook
evo-cli cryptogram +pay \
  --network-token-value 5120350199991234 \
  --token-expiry-date 1228 \
  --token-cryptogram "AAKQJPQZFWufAAJ5j4JeAAADFA==" \
  --eci 06 \
  --payment-brand Mastercard \
  --amount 10.00 --currency USD \
  --auto-capture true \
  --merchant-tx-id my_custom_tx_001 \
  --webhook https://example.com/webhook
```

### Dry-Run Mode

Preview the request without sending:

```bash
evo-cli cryptogram +pay \
  --network-token-value 5120350199991234 \
  --token-expiry-date 1228 \
  --token-cryptogram "AAKQJPQZFWufAAJ5j4JeAAADFA==" \
  --eci 06 \
  --payment-brand Mastercard \
  --amount 10.00 --currency USD \
  --dry-run
```

Output:
```json
{
  "method": "POST",
  "url": "https://hkg-online-uat.everonet.com/g2/v1/payment/mer/S024116/payment",
  "headers": null,
  "body": {
    "merchantTransInfo": {
      "merchantTransID": "ntpay_1719576000",
      "merchantTransTime": "2025-06-28T12:00:00Z"
    },
    "transAmount": {
      "currency": "USD",
      "value": "10.00"
    },
    "paymentMethod": {
      "type": "token",
      "token": {
        "type": "networkToken",
        "value": "5120350199991234",
        "expiryDate": "1228",
        "tokenCryptogram": "AAKQJPQZFWufAAJ5j4JeAAADFA==",
        "eci": "06",
        "paymentBrand": "Mastercard",
        "walletIdentifiers": "MDESForMerchants"
      }
    },
    "transInitiator": {
      "platform": "WEB"
    }
  }
}
```

---

## Cryptogram Webhook Notification

If you provide a `webhook` in the cryptogram request, Evo Payment will send an async notification when the cryptogram is ready:

```json
{
  "eventCode": "Cryptogram",
  "result": {
    "code": "S0000",
    "message": "Success"
  },
  "cryptogram": {
    "status": "Success",
    "merchantTransInfo": {
      "merchantTransID": "crypto_unique_id_001",
      "merchantTransTime": "2025-06-28T12:01:00Z"
    }
  },
  "paymentMethod": {
    "networkToken": {
      "tokenID": "NT_abc123def456",
      "tokenCryptogram": "AAKQJPQZFWufAAJ5j4JeAAADFA==",
      "eci": "06",
      "value": "5120350199991234",
      "expiryDate": "1228",
      "paymentBrand": "Mastercard",
      "status": "enabled"
    }
  }
}
```

Respond with plain text `SUCCESS` (HTTP 200) to acknowledge the notification.
