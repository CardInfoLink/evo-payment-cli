# 使用 Network Token + Cryptogram 发起支付

本文档说明如何通过 Evo Payment Merchant API，使用 **Network Token + Cryptogram** 发起一笔支付请求。

## 前置条件

在调用支付接口之前，你需要通过前置步骤（例如 `agenzo-token-cli`）获取以下参数：

| 参数 | 说明 | 来源 |
|------|------|------|
| Network Token Value | 网络令牌值（卡号形式） | Cryptogram 生成步骤返回的 `paymentMethod.networkToken.value` |
| Token Expiry Date | 令牌有效期（MMYY 格式） | Cryptogram 生成步骤返回的 `paymentMethod.networkToken.expiryDate` |
| Token Cryptogram | 一次性密文 | Cryptogram 生成步骤返回的 `paymentMethod.networkToken.tokenCryptogram` |
| ECI | ECI | Cryptogram 生成步骤返回的 `paymentMethod.networkToken.eci` |

此外还需要：
- 有效的 Evo Payment 商户账号（Store ID / `sid`）
- Evo Payment 分配的签名密钥
- 支持 TLS v1.2+ 的 HTTPS 客户端

## API 签名

请求签名规则详见官方文档：
https://docs.everonet.com/merchant/api-integration/api-reference/merchant-api/api-rules

## 请求

```
POST /g2/v1/payment/mer/{sid}/payment
```

### HTTP Headers

| Header | 值 |
|--------|-----|
| `Content-Type` | `application/json; charset=utf-8` |
| `DateTime` | ISO 8601 时间戳，如 `2025-06-28T12:00:00+08:00` |
| `MsgID` | 请求追踪 ID（建议 UUID，最长 32 字符） |
| `SignType` | `SHA256` / `SHA512` / `HMAC-SHA256` / `HMAC-SHA512` |
| `Authorization` | 消息签名值 |
| `Idempotency-Key` | （可选）幂等键，建议使用 `merchantTransID`，最长 64 字符 |

### 请求体

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

### 字段说明

#### merchantTransInfo（必填）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `merchantTransID` | String(32) | 是 | 商户唯一交易 ID，同一 sid 下不可重复 |
| `merchantTransTime` | DateTime | 是 | 交易发起时间，ISO 8601 格式 |

#### transAmount（必填）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `currency` | String(3) | 是 | ISO 4217 货币代码，如 `USD`、`HKD`、`JPY` |
| `value` | String(12) | 是 | 金额。如 `"10.00"` 表示 10 美元；`"1234"` 表示 1234 日元 |

#### paymentMethod（必填）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | String | 是 | 固定为 `"token"` |
| `token.type` | String | 是 | 固定为 `"networkToken"` |
| `token.value` | String(64) | 是 | Network Token 值（前置步骤获取） |
| `token.expiryDate` | String(4) | 是 | 令牌有效期，格式 MMYY（前置步骤获取） |
| `token.tokenCryptogram` | String(64) | 是 | 一次性密文（前置步骤获取） |
| `token.eci` | String(2) | 是 | ECI（前置步骤获取） |
| `token.paymentBrand` | String(32) | 是 | 支付品牌：`"Visa"` 或 `"Mastercard"` |
| `token.walletIdentifiers` | String | 否 | 令牌服务提供方标识。Mastercard 默认 `"MDESForMerchants"`，Visa 可留空 |

#### 其他可选字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `transInitiator.platform` | String | 交易发起平台：`"WEB"`、`"APP"` 等 |
| `returnURL` | String(300) | 支付完成后跳转地址 |
| `webhook` | String(300) | 异步通知地址 |
| `captureAfterHours` | String | 设为 `"0"` 表示授权后立即扣款（auto capture） |

## 响应

### 响应 Headers

| Header | 说明 |
|--------|------|
| `Content-Type` | `application/json` |
| `DateTime` | Evo Payment 处理时间（ISO 8601） |
| `MsgID` | Evo Payment 返回的消息 ID |
| `SignType` | 签名算法（与请求一致） |
| `Authorization` | 响应签名值（商户应验签） |

### 成功响应示例

```json
{
  "result": {
    "code": "S0000",
    "message": "Success",
    "pspMessage": "Approved or completed successfully",
    "pspResponseCode": "00"
  },
  "payment": {
    "status": "Authorised",
    "transAmount": {
      "currency": "USD",
      "value": "10.00"
    },
    "merchantTransInfo": {
      "merchantTransID": "test_ntpay_1782622199",
      "merchantTransTime": "2026-06-28T04:49:59Z"
    },
    "evoTransInfo": {
      "evoTransID": "pay2606281250010001000067564695",
      "evoTransTime": "2026-06-28T04:50:01Z",
      "retrievalReferenceNum": "617912683620",
      "traceNum": "683620"
    },
    "pspTransInfo": {
      "authorizationCode": "546762",
      "cvcCheckResult": "M",
      "cvcCheckResultRaw": "M",
      "pspTransID": "MCS00019Y3012",
      "pspTransTime": "2026-06-28T04:49:59Z",
      "retrievalReferenceNum": "617912683620"
    }
  },
  "paymentMethod": {
    "card": {
      "productID": "MCS"
    },
    "isNetworkToken": true,
    "networkToken": {
      "paymentBrand": "Mastercard",
      "supportDeviceBinding": false
    },
    "token": {
      "eci": "05",
      "first6No": "512345",
      "last4No": "2345",
      "value": "512345******2345"
    }
  }
}
```

### 响应字段说明

#### result

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | String(8) | 结果码。`S0000` 表示成功 |
| `message` | String(1024) | 结果描述 |
| `pspResponseCode` | String(1024) | PSP/发卡行原始响应码 |
| `pspMessage` | String(1024) | PSP/发卡行原始响应信息 |

#### payment

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | String | 交易状态：`Authorised`（已授权）、`Captured`（已扣款）、`Failed`（拒绝） |
| `transAmount.currency` | String(3) | 交易货币 |
| `transAmount.value` | String(12) | 交易金额 |
| `merchantTransInfo.merchantTransID` | String(32) | 商户交易 ID（回显） |
| `merchantTransInfo.merchantTransTime` | DateTime | 商户交易时间（回显） |

#### payment.evoTransInfo

| 字段 | 类型 | 说明 |
|------|------|------|
| `evoTransID` | String | Evo Payment 平台交易 ID |
| `evoTransTime` | DateTime | Evo Payment 处理时间 |
| `retrievalReferenceNum` | String | 检索参考号 |
| `traceNum` | String | 交易跟踪号 |

#### payment.pspTransInfo

| 字段 | 类型 | 说明 |
|------|------|------|
| `authorizationCode` | String | 发卡行授权码 |
| `cvcCheckResult` | String | CVC 校验结果 |
| `cvcCheckResultRaw` | String | PSP 原始 CVC 校验结果 |
| `pspTransID` | String | PSP 交易 ID |
| `pspTransTime` | DateTime | PSP 处理时间 |
| `retrievalReferenceNum` | String | PSP 检索参考号 |

#### paymentMethod

| 字段 | 类型 | 说明 |
|------|------|------|
| `card.productID` | String | 卡产品标识 |
| `isNetworkToken` | Boolean | 是否为 Network Token 交易 |
| `networkToken.paymentBrand` | String | Network Token 支付品牌 |
| `networkToken.supportDeviceBinding` | Boolean | 是否支持设备绑定 |
| `token.eci` | String(2) | ECI 值（回显） |
| `token.first6No` | String(6) | Token 前 6 位 |
| `token.last4No` | String(4) | Token 后 4 位 |
| `token.value` | String | Token 掩码值（如 `512345******2345`） |

### 错误响应示例

```json
{
  "result": {
    "code": "B0013",
    "message": "Transaction declined",
    "pspResponseCode": "05",
    "pspMessage": "Do not honor"
  }
}
```

### 结果码说明

| 结果码前缀 | 含义 |
|-----------|------|
| `S0000` | 成功 |
| `B****` | 业务错误（拒绝、余额不足等） |
| `V****` | 参数校验错误 |
| `E****` | 系统/网络错误 |
| `P****` | PSP/发卡行错误 |

完整应答码列表详见官方文档：
https://docs.everonet.com/merchant/api-integration/api-reference/merchant-api/appendix/result-code

## 完整示例（cURL）

```bash
curl -X POST "https://hkg-online-uat.everonet.com/g2/v1/payment/mer/S005898/payment" \
  -H "Content-Type: application/json; charset=utf-8" \
  -H "DateTime: 2025-06-28T12:02:00+08:00" \
  -H "MsgID: a1b2c3d4e5f6" \
  -H "SignType: SHA256" \
  -H "Authorization: <签名值>" \
  -H "Idempotency-Key: pay_unique_id_001" \
  -d '{
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
  }'
```

## 注意事项

- 每笔支付必须使用新的 `merchantTransID`
- `tokenCryptogram` 为一次性使用，支付后即失效，不可复用
- 如需重试同一笔支付，使用相同的 `Idempotency-Key` 保证幂等
- `DateTime` 字段的时间与实际发送时间偏差不宜过大
- 响应中 JSON 键值对的顺序不保证，解析时不应依赖顺序
- 空值字段可能在响应中被省略
