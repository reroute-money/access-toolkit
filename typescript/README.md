# @reroute.money/access-toolkit

Reroute Money 接入点 REST `/v2/*` 的 TypeScript 客户端 | 签名鉴权、下单/查单/账户、信封解包与业务错误都在这一层。能力口径与仓库根的 [`golang/`](../golang) 版对齐，语义细节见[根 README](../README.md)。

## 安装

包尚未发布到 npm（`@reroute.money/access-toolkit` 当前 registry 404），先按源码方式接：

```bash
git clone https://github.com/reroute-money/access-toolkit.git
cd access-toolkit/typescript && pnpm install && pnpm run build
npm install file:../access-toolkit/typescript   # 在商户项目里按路径引用
```

发布后直接 `npm install @reroute.money/access-toolkit`，`exports` 已指向 `dist/`（ESM + CJS + `.d.ts`）。

## 用法

```ts
import { RerouteMoneyClient } from '@reroute.money/access-toolkit';

const client = new RerouteMoneyClient({
  userKey: {
    uniqueIdentifier: '<usingKey 编号>',
    servicePrivateKey: '-----BEGIN RSA PRIVATE KEY-----\n...',
    platformPublicKey: '-----BEGIN PUBLIC KEY-----\n...',
  },
  tenantMember: { uniqueIdentifier: '<租户成员编号>' },
  _httpClient: { baseURL: 'https://access-point.reroute.money' }, // 可省略，默认即此地址
});

const response = await client.makePaymentOrder({
  order: {
    externalIdentifier: crypto.randomUUID(), // 商户单号 | 平台单号留空由服务端生成
    paymentAmount: { currency: 'INR', value: 100 },
    receiptAmount: { currency: 'INR' }, // 只给币种 → 服务端按命中汇率补齐
    product: { type: 'ONLINE_PAY' },
  },
  receiptInformation: { type: 'UPI', identifier: 'payee@okhdfcbank' },
});

console.log(response.data.order.status, response.data.order.internalIdentifier);
```

`platformPublicKey` 目前在 TS 版只登记不消费（没有回调验签入口，Go 版有）；`_httpClient` 是逃生入口，除 `baseURL` 外的 axios 配置同样生效。

## 出口

| 导出 | 说明 |
|------|------|
| `RerouteMoneyClient` | 客户端：`makePaymentOrder` / `makeCollectionOrder` / `queryPaymentOrder` / `queryCollectionOrder` / `queryAccounts` / `call<T>` / `getHTTPClient` |
| `RerouteMoneyException` | 业务错误 | HTTP 可能是 200，但信封 `code !== 'OK'` 即抛，`code` / `message` 挂在实例上 |
| `RerouteMoneyUtility` | `signRequest` / `sign` / `throwIfNotOk` | 自定义 http 客户端时可复用同一条签名链路 |
| `PaymentOrder`, `CollectionOrder`, `ReceiptInformation`, `Amount`, `Currency`, `OrderResult`, `PaymentOrderResult`, `CollectionOrderResult`, `OrderStatus`, `OrderType`, `Account`, `RerouteMoneyResponse` | 实体类型 | 与后端 `/v2/*` 请求/响应口径对齐 |

下单入参按 `DeepPartial<T>` 接收，`uniqueIdentifier` 之类的服务端生成字段可不传。

## 签名

规范串与后端 `IdentityVerificationUtility.buildSignatureContent` 逐字节一致，请求在 axios 的 `transformRequest` 里对**同一份** `JSON.stringify` 结果签名后发出：

```
{METHOD} {path}\r\n\r\nGeneration-Time: {毫秒时间戳}\r\nAct-As: {租户成员编号}[\r\n\r\n{请求体}]
```

RSA-SHA512 + PKCS#1 v1.5，base64 后写入 `Authorization: Signature usingKey=..&actAs=..&generationTime=..&requestSignature=..`；`generationTime` 与服务器时间相差 >5 分钟会被拒。

## 命令

```bash
pnpm install
pnpm run build      # rslib 出 ESM + CJS + d.ts
pnpm run dev        # watch 模式重建
pnpm run test       # rstest
pnpm run test:watch
pnpm run lint       # eslint
pnpm run format     # prettier
```

`tests/index.test.ts` 指向本地接入点 `http://127.0.0.1:4001`，需要先把后端跑起来；仓库内的测试密钥仅用于本地联调。
