# access-toolkit (Go)

Reroute Money 接入点 REST `/v2/*` 的 Go 客户端 | `typescript/` 版的同能力移植，只用标准库（`net/http` + `crypto/rsa`），无第三方依赖。

## 安装

```bash
go get github.com/reroute-money/access-toolkit/golang
```

module path 指向仓库的 `golang/` 子目录：Go 对 `github.com` 是硬编码解析（不会去问你自己域名的 `?go-get=1`），所以这条依赖本仓库是 **public** 的 `github.com/reroute-money/access-toolkit`，且本目录是仓库里唯一的 Go module（`typescript/` 不参与解析）。

发版打 tag 两种都行：优先 `golang/v0.1.0`（子目录前缀，语义明确）；仓库级 `v0.1.0` 也有效，Go 在找不到前缀标签时回落到仓库级标签。本仓库当前未打 tag，`@latest` 会解析到默认分支的伪版本，商户 `go get` 依然可用，只是想锁版本时得先打 tag。

点对点交付（离线、或商户不便访问 GitHub）仍可：

```bash
# 1) 直接给源码
go mod vendor    # 或打包本目录发给对方

# 2) 对方 go.mod 里指向本地 clone
replace github.com/reroute-money/access-toolkit/golang => ../access-toolkit/golang
```

## 用法

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	access "github.com/reroute-money/access-toolkit/golang"
)

func main() {
	client, err := access.NewClient(access.Config{
		// BaseURL 留空取 https://access-point.reroute.money
		BaseURL:      "https://access-point.reroute.money",
		UserKey:      access.UserKey{UniqueIdentifier: "...", ServicePrivateKey: "-----BEGIN RSA PRIVATE KEY-----\n..."},
		TenantMember: access.TenantMember{UniqueIdentifier: "..."},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 代付下单 | 付款侧金额完整, 收款侧只给币种 → 服务端按命中汇率补齐
	value := int64(100)
	response, err := client.MakePaymentOrder(context.Background(), access.PaymentOrder{
		Order: access.OrderRequest{
			PaymentAmount:      access.Amount{Currency: access.CurrencyINR, Value: &value},
			ReceiptAmount:      access.Amount{Currency: access.CurrencyINR},
			ExternalIdentifier: "32730576-e8c8-415f-a648-9b1568f1e992",
			Product:            access.Product{Type: "ONLINE_PAY"},
		},
		ReceiptInformation: access.NewUPIReceipt("payee@okhdfcbank"),
	})
	if err != nil {
		// 业务失败（HTTP 200 但 code != OK）用 errors.As 取错误码
		var businessError *access.Error
		if errors.As(err, &businessError) {
			log.Fatalf("下单被拒 [%s] %s", businessError.Code, businessError.Message)
		}
		log.Fatal(err)
	}
	// -
	fmt.Println(response.Data.Order.UniqueIdentifier, response.Data.Order.Status)
}
```

- 代收下单：`client.MakeCollectionOrder(ctx, access.CollectionOrder{Order: ...})`，无收款信息。
- 查询：`client.QueryPaymentOrder(ctx, identifier)` / `client.QueryCollectionOrder(ctx, identifier)`，`identifier` 为平台单号（内部 uuid）或外部单号，命中别人的单返回业务错。
- 账户：`client.QueryAccounts(ctx)` 列签名 `actAs` 成员名下的余额户与保证金户，无入参。
- 幂等：同一 `tenantMember` + `externalIdentifier` 重复下单复用库内单据；金额/收款信息/`returnURL` 不一致时报业务错（换了回跳地址就是另一笔导流口径，静默复用旧单会把付款人导回第一次登记的那个页面）。
- `/v2/*` 之外的端点用 `access.Call[T](ctx, client, method, path, payload)`（对应 TS 的 `client.call<T>()`）。
- 三个单号别混：`order.externalIdentifier` 是**商户自己的单号**（下单带进来，成员级唯一），`order.uniqueIdentifier` 是**平台单号**（下发给渠道当 `externalId` 用的就是它），`order.internalIdentifier` 是**渠道单号**（渠道给出的订单编号，平台拿到渠道结论才落得上，下单被渠道明确拒绝时为空；落定后不会再变，可用它去渠道侧核对这笔交易）。
- 两个地址也别混：`redirectURL` 是**去**渠道收银台的跳转地址（渠道给的，商户据此把付款人导过去；渠道没给则一直为空）；`returnURL` 是**回**商户的跳转地址（商户下单时登记，平台原样下发渠道，付款人付完后由渠道导回这里）。平台都不校验 `returnURL` 归属哪个域名，也不把它拼到 `redirectURL` 上。

## 订单推送验签（我方→商户）

单据进终态（`SUCCEEDED` / `FAILED`）时平台按 `order.callbackURL` 推一份 Order 报文给商户，商户**必须先验签再落库**——回调路由对全网开放，谁都能构造一个 `status: SUCCEEDED` 的 POST。

```go
notify, err := access.ParseMerchantNotify(rawBody)  // 报文是 Order 实体本身，不带 {code,data} 信封
if err != nil { /* ErrNotifyBodyInvalid */ }

err = access.VerifyMerchantNotify(platformPublicKeyPEM, access.NotifyVerification{
	// 关键：取下单时登记的 callbackURL 的 pathname，不是请求到达自己服务时的 URL.Path
	// 平台拼规范串用的就是它（new URL(callbackURL).pathname），挂在反代子路径下两者会错开
	CallbackPath:  u.Pathname,
	Authorization: request.Header.Get("Authorization"),
	Body:          rawBody, // 原始字节，先验签再解报文
})
```

- 规范串与出站请求签名同源（同一个 `signatureContent`），只是 `Act-As` 当前固定为 `-`、签名密钥换成平台的 `platformPrivateKey`；商户验签用的公钥就是下单鉴权时拿到的那把 `platformPublicKey`，无需新增密钥。
- 时间戳窗口默认 5 分钟（`DefaultNotifyMaxSkew`，与平台校验入站签名的窗口一致），可用 `NotifyVerification.MaxSkew` 调整；`Now` 留零值按当前时间判。
- 失败原因用 `errors.Is` 区分：`ErrSignatureInvalid`（伪造/报文被改/公钥配错）、`ErrSignatureExpired`、`ErrAuthorizationInvalid`、`ErrPublicKeyInvalid`、`ErrNotifyBodyInvalid`。
- 推送至少一次投递，重投次数用尽即放弃，商户要按单号 + 状态幂等去重，并留主动查单（`QueryCollectionOrder` / `QueryPaymentOrder`）作掉单兜底。

## 鉴权：接口签名方法

规范串与后端 `IdentityVerificationUtility.buildSignatureContent` 逐字节一致：

```
{METHOD} {path}\r\n\r\nGeneration-Time: {毫秒时间戳}\r\nAct-As: {租户成员编号}[\r\n\r\n{请求体原始字节}]
```

- `path` 不含 host 与 query（带 query 时签名只取 `?` 之前，服务端验签同样只看 `request.baseUrl`），但**含路径参数**：查单签的是 `/v2/payment-order/{单号}` 整段。
- 请求体段仅带体时拼，且必须与实际发送字节一致 —— 所以客户端先 `json.Marshal` 再签名，发同一份字节。
- RSA-SHA512 + PKCS#1 v1.5，base64 后拼成：

```
Authorization: Signature usingKey=..&actAs=..&generationTime=..&requestSignature=..
```

服务端还校验 `generationTime` 与服务器时间相差 ≤5 分钟，本机时钟漂了会直接被拒。

## 与 typescript 版的差异

| 点 | typescript | golang |
|----|------------|--------|
| HTTP | axios + `transformRequest` 内签名 | `net/http`，先序列化再签名 |
| 可选金额 | `value?: number` | `Value *int64`（`nil` 即不下发该字段） |
| 收款信息 | 联合类型 | 单结构体 + `NewUPIReceipt` / `NewBankReceipt` / `NewBlockchainReceipt` 构造 |
| 下单入参 | `DeepPartial<T>` | 字段直接可选（`omitempty`） |
| 业务错误 | throw `RerouteMoneyException` | 返回 `*access.Error`（`Code` / `Message`），配 `errors.As` |
| 响应单据 | 复用请求类型 | `OrderResult`，只声明商户侧消费的字段，多余字段解析时忽略 |
| 币种 | `INR \| USD` | 另含后端已有的 `USDT`（链上出款） |
| `platformPublicKey` | 传入但未使用，没有验签入口 | `Config` 里保留，由 `VerifyMerchantNotify` 消费 |
| 订单推送验签 | 无 | `VerifyMerchantNotify` + `ParseMerchantNotify` + `MerchantNotify` |

## 命令

```bash
go build ./...   # 编译
go vet ./...     # 静态检查
gofmt -l .       # 格式（无输出即已格式化）
go test ./...    # 测试
```

测试不依赖网络：`TestSignMatchesNode` / `TestAuthorizationHeaderMatchesNode` 拿 Node `createSign('sha512')` + `URLSearchParams` 的产出做基准值比对（RSA PKCS#1 v1.5 是确定性签名，可直接比字节）；`client_test.go` 里的假接入点按后端中间件口径重建规范串验签，覆盖请求体、签名参数、业务错误码与网关非 JSON 响应。
