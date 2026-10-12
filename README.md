# access-toolkit

Reroute Money 接入点 REST `/v2/*` 的官方客户端 | 同一套签名鉴权与实体口径的两个语言实现，能力对齐。

| 目录 | 包名 | 技术栈 |
|------|------|--------|
| [`typescript/`](typescript) | `@reroute.money/access-toolkit` | Rslib 构建，运行时依赖 `axios` + `es-toolkit`，Node 22 |
| [`golang/`](golang) | `github.com/reroute-money/access-toolkit/golang` | 包名 `access`，只用标准库（`net/http` + `crypto/rsa`），Go 1.23 |

接入点默认地址 `https://access-point.reroute.money`，可用 `BaseURL` / `_httpClient.baseURL` 覆盖（测试环境、灰度域名）。

## 安装

Go 版直接可 `go get`（本仓库 public，module path 指向 `golang/` 子目录）：

```bash
go get github.com/reroute-money/access-toolkit/golang
```

TypeScript 版尚未发布到 npm（`@reroute.money/access-toolkit` 当前 registry 404），先按源码方式接：

```bash
git clone https://github.com/reroute-money/access-toolkit.git
cd access-toolkit/typescript && pnpm install && pnpm run build
# 然后在商户项目里按路径引用
npm install file:../access-toolkit/typescript
```

发布后 `npm install @reroute.money/access-toolkit` 即可，`package.json` 的 `exports` 已指向 `dist/`。

## 快速上手

### TypeScript

```ts
import { RerouteMoneyClient } from '@reroute.money/access-toolkit';

const client = new RerouteMoneyClient({
  userKey: {
    uniqueIdentifier: '<usingKey 编号>',
    servicePrivateKey: '-----BEGIN RSA PRIVATE KEY-----\n...',
    platformPublicKey: '-----BEGIN PUBLIC KEY-----\n...',
  },
  tenantMember: { uniqueIdentifier: '<租户成员编号>' },
});

const value = 100; // 最小币种单位（分）
const response = await client.makePaymentOrder({
  order: {
    externalIdentifier: crypto.randomUUID(), // 商户单号 | 单据号与订单号都由服务端生成，请求里不用带
    paymentAmount: { currency: 'INR', value },
    receiptAmount: { currency: 'INR' }, // 只给币种 → 服务端按命中汇率补齐
    product: { type: 'ONLINE_PAY' },
  },
  receiptInformation: { type: 'UPI', identifier: 'payee@okhdfcbank' },
});

console.log(response.data.uniqueIdentifier, response.data.order.status);
```

业务失败（HTTP 200 但信封 `code !== 'OK'`）抛 `RerouteMoneyException`，`code` / `message` 在实例上。

### Go

```go
package main

import (
	"context"
	"errors"
	"log"

	access "github.com/reroute-money/access-toolkit/golang"
)

func main() {
	client, err := access.NewClient(access.Config{
		UserKey:      access.UserKey{UniqueIdentifier: "...", ServicePrivateKey: "-----BEGIN RSA PRIVATE KEY-----\n..."},
		TenantMember: access.TenantMember{UniqueIdentifier: "..."},
	})
	if err != nil {
		log.Fatal(err)
	}

	value := int64(100)
	response, err := client.MakePaymentOrder(context.Background(), access.PaymentOrder{
		Order: access.OrderRequest{
			PaymentAmount:      access.Amount{Currency: access.CurrencyINR, Value: &value},
			ReceiptAmount:      access.Amount{Currency: access.CurrencyINR},
			ExternalIdentifier: "...",
			Product:            access.Product{Type: "ONLINE_PAY"},
		},
		ReceiptInformation: access.NewUPIReceipt("payee@okhdfcbank"),
	})
	if err != nil {
		var businessError *access.Error
		if errors.As(err, &businessError) {
			log.Fatalf("下单被拒 [%s] %s", businessError.Code, businessError.Message)
		}
		log.Fatal(err)
	}
	_ = response
}
```

## 端点

| 方法 | TS | Go | 说明 |
|------|----|----|------|
| `PUT /v2/payment-order` | `makePaymentOrder` | `MakePaymentOrder` | 代付下单，收款信息必传（`ReceiptInformation`） |
| `PUT /v2/collection-order` | `makeCollectionOrder` | `MakeCollectionOrder` | 代收下单，无收款信息，可带 `returnURL` |
| `GET /v2/payment-order/{identifier}` | `queryPaymentOrder` | `QueryPaymentOrder` | 查单，`identifier` 可传单据号（响应顶层 `uniqueIdentifier`）或商户外部单号；订单层的 `order.uniqueIdentifier` 不参与匹配 |
| `GET /v2/collection-order/{identifier}` | `queryCollectionOrder` | `QueryCollectionOrder` | 同上 |
| `GET /v2/account` | `queryAccounts` | `QueryAccounts` | 列签名 `actAs` 成员名下的余额户与保证金户，无入参 |
| `/v2/*` 之外的端点 | `call<T>(config)` | `access.Call[T](ctx, client, method, path, payload)` | 逃生入口，签名与解信封复用同一条链路 |

## 鉴权：接口签名

两个版本产出**逐字节一致**的规范串（与后端 `IdentityVerificationUtility.buildSignatureContent` 对齐）：

```
{METHOD} {path}\r\n\r\nGeneration-Time: {毫秒时间戳}\r\nAct-As: {租户成员编号}[\r\n\r\n{请求体原始字节}]
```

- `path` 不含 host 与 query，但**含路径参数**：查单签的是 `/v2/payment-order/{单号}` 整段。
- 请求体段只在带体时拼，且必须与实际发送字节一致 —— Go 侧先 `json.Marshal` 再签名，TS 侧在 `transformRequest` 里对同一份 `JSON.stringify` 结果签名。
- RSA-SHA512 + PKCS#1 v1.5，base64 后拼成请求头：

```
Authorization: Signature usingKey=..&actAs=..&generationTime=..&requestSignature=..
```

服务端还校验 `generationTime` 与服务器时间相差 ≤5 分钟，本机时钟漂了会直接被拒。

## 关键语义

- **四个单号别混**：**单据号** `uniqueIdentifier` 在响应顶层（代收是收款单号、代付是付款单号，各是 `collection_orders` / `payment_orders` 行自己的 uuid）——查单接口认它，商户要存的、订单推送带的都是它；`order.externalIdentifier` 是**商户自己的单号**（下单带进来，成员级唯一）；`order.uniqueIdentifier` 是**订单号**（`orders` 行自己的 uuid，平台下发给渠道当 `externalId` 用的就是它，**查单接口不认它**，拿它去查必然查空）；`order.internalIdentifier` 是**渠道单号**（平台拿到渠道结论才落得上，下单被渠道明确拒绝时为空，落定后不再变化，可用它去渠道侧核对这笔交易）。
- **两个地址别混**：`redirectURL` 是**去**渠道收银台的跳转地址（渠道给的，商户据此把付款人导过去；渠道没给则一直为空）；`returnURL` 是**回**商户的跳转地址（商户下单时登记，平台原样下发渠道，付款人付完后由渠道导回这里）。平台不校验 `returnURL` 归属哪个域名，也不把它拼到 `redirectURL` 上。
- **金额**：两侧币种必传，至少一侧金额完整，缺失侧由服务端按命中汇率补齐（找不到汇率直接报错）。`value` 一律为最小币种单位（分）。
- **幂等**：同一 `tenantMember` + `externalIdentifier` 重复下单复用库内单据；金额、收款信息、`returnURL` 不一致时报业务错——换了回跳地址就是另一笔导流口径，静默复用旧单会把付款人导回第一次登记的那个页面。

## 订单推送验签（平台 → 商户）

单据进终态（`SUCCEEDED` / `FAILED`）时平台按 `order.callbackURL` 推一份**业务单据**（代收推收款单、代付推付款单，订单信息包在 `order` 里，形状与查单响应同构），商户**必须先验签再落库**——回调路由对全网开放，谁都能构造一个 `status: SUCCEEDED` 的 POST。

当前只有 Go 版提供入口（`ParseMerchantNotify` + `VerifyMerchantNotify`，规范串与出站签名同源，`Act-As` 固定为 `-`，密钥换成平台的 `platformPrivateKey`），细节见 [`golang/README.md`](golang/README.md)；TypeScript 版把 `platformPublicKey` 传进配置但暂无验签入口。

要点：验签用的路径取**下单时登记的 `callbackURL` 的 pathname**，不是请求到达自己服务时的 `URL.Path`（挂在反代子路径下两者会错开）；报文先拿原始字节验签再解析；推送为至少一次投递，商户要按 `order.externalIdentifier`（商户自己的单号）+ `order.status` 幂等去重，并留主动查单作掉单兜底——查单传的是推送顶层的**单据号**或自己的外部单号，**不是** `order.uniqueIdentifier`。

## 开发

```bash
# TypeScript（在 typescript/ 内）
pnpm install && pnpm run build   # rslib 出 ESM + CJS + d.ts
pnpm run test                    # rstest
pnpm run lint && pnpm run format

# Go（在 golang/ 内）
go build ./... && go vet ./... && go test ./...
gofmt -l .                       # 无输出即已格式化
```

Go 侧测试不依赖网络：`TestSignMatchesNode` / `TestAuthorizationHeaderMatchesNode` 拿 Node `createSign('sha512')` + `URLSearchParams` 的产出做基准值比对（RSA PKCS#1 v1.5 是确定性签名，可直接比字节）；`client_test.go` 里的假接入点按后端中间件口径重建规范串验签。TypeScript 侧 `tests/index.test.ts` 指向本地接入点 `127.0.0.1:4001`，需要起服务才能跑通。

两个版本的差异（可选金额表达、收款信息构造方式、错误消费方式、币种取值等）见 [`golang/README.md`](golang/README.md) 末尾的对照表。
