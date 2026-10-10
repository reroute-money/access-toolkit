// Package access 是 Reroute Money 接入点（REST `/v2/*`）的 Go 客户端 | 与同目录 typescript/ 版能力对齐
//
// 鉴权走「接口签名方法」: 用 servicePrivateKey 对规范串做 RSA-SHA512 签名,
// 结果拼进 `Authorization: Signature usingKey=..&actAs=..&generationTime=..&requestSignature=..`。
package access

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL 接入点默认地址
const DefaultBaseURL = "https://access-point.reroute.money"

// maxResponseSize 响应体读取上限 | 下单响应是单据, 正常远小于此, 上限只为挡住异常大响应
const maxResponseSize = 4 << 20

// UserKey 用户密钥 | 在管理后台创建用户密钥后拿到
type UserKey struct {
	// UniqueIdentifier 密钥编号
	UniqueIdentifier string
	// ServicePrivateKey 服务私钥 | PEM（PKCS#1 / PKCS#8）, 用于给请求签名
	ServicePrivateKey string
	// PlatformPublicKey 平台公钥 | 与 typescript 版入参对齐, 出站回调验签时交给调用方使用, 本客户端不消费
	PlatformPublicKey string
}

// TenantMember 租户成员 | 签名里的 actAs
type TenantMember struct {
	// UniqueIdentifier 成员编号
	UniqueIdentifier string
}

// Config 客户端配置
type Config struct {
	// BaseURL 接入点地址 | 留空取 DefaultBaseURL
	BaseURL string
	// HTTPClient HTTP 客户端 | 留空用带 30s 超时的默认客户端
	HTTPClient *http.Client
	// UserKey 用户密钥
	UserKey UserKey
	// TenantMember 租户成员
	TenantMember TenantMember
}

// Client 接入点客户端 | 并发安全, 一个进程复用一把密钥即可
type Client struct {
	// baseURL 已去掉末尾斜杠
	baseURL string
	// httpClient 发请求用
	httpClient *http.Client
	// signingKey 签名私钥
	signingKey *rsa.PrivateKey
	// usingKey 密钥编号
	usingKey string
	// actAs 扮演的角色
	actAs string
}

// NewClient 建客户端 | 私钥解析不了就直接失败, 不留到第一次请求
func NewClient(config Config) (*Client, error) {
	// 解析私钥
	signingKey, err := parsePrivateKey(config.UserKey.ServicePrivateKey)
	if err != nil {
		return nil, err
	}

	// 校验必填身份
	if config.UserKey.UniqueIdentifier == "" {
		return nil, fmt.Errorf("access: 缺少 userKey.uniqueIdentifier")
	}
	if config.TenantMember.UniqueIdentifier == "" {
		return nil, fmt.Errorf("access: 缺少 tenantMember.uniqueIdentifier")
	}

	// 处理地址 | 末尾斜杠去掉, 拼 path 时不会出现双斜杠
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		signingKey: signingKey,
		usingKey:   config.UserKey.UniqueIdentifier,
		actAs:      config.TenantMember.UniqueIdentifier,
	}, nil
}

// Call 发一个已签名的请求并解信封 | 泛型版对应 TS 的 `client.call<T>()`, 供 `/v2/*` 之外的端点复用
//
// payload 为 nil 时不发请求体（也不拼进规范串）；path 可带 query, 但签名只覆盖 '?' 之前的路径
// （服务端验签取的是 request.baseUrl, 同样不含 query）。
func Call[T any](ctx context.Context, client *Client, method, path string, payload any) (*Response[T], error) {
	body, statusCode, err := client.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}

	// 解信封 | 解不动就是传输层问题（网关错误页等）, 业务码还没轮到
	var response Response[T]
	if err := json.Unmarshal(body, &response); err != nil {
		if statusCode < 200 || statusCode > 299 {
			return nil, &HTTPStatusError{StatusCode: statusCode, Body: snippet(body)}
		}
		return nil, fmt.Errorf("access: 解析响应失败: %w", err)
	}
	if err := response.Err(); err != nil {
		return nil, err
	}
	return &response, nil
}

// MakePaymentOrder 代付下单
func (client *Client) MakePaymentOrder(ctx context.Context, order PaymentOrder) (*Response[PaymentOrderResult], error) {
	return Call[PaymentOrderResult](ctx, client, http.MethodPut, "/v2/payment-order", order)
}

// MakeCollectionOrder 代收下单
func (client *Client) MakeCollectionOrder(ctx context.Context, order CollectionOrder) (*Response[CollectionOrderResult], error) {
	return Call[CollectionOrderResult](ctx, client, http.MethodPut, "/v2/collection-order", order)
}

// QueryPaymentOrder 查询代付单 | identifier 为平台单号（内部 uuid）或外部单号
func (client *Client) QueryPaymentOrder(ctx context.Context, identifier string) (*Response[PaymentOrderResult], error) {
	// 单号拼进路径 | 服务端验签取 request.baseUrl, 它是含单号的完整路径, 所以规范串也必须带上
	return Call[PaymentOrderResult](ctx, client, http.MethodGet, "/v2/payment-order/"+identifier, nil)
}

// QueryCollectionOrder 查询代收单 | identifier 为平台单号（内部 uuid）或外部单号
func (client *Client) QueryCollectionOrder(ctx context.Context, identifier string) (*Response[CollectionOrderResult], error) {
	return Call[CollectionOrderResult](ctx, client, http.MethodGet, "/v2/collection-order/"+identifier, nil)
}

// QueryAccounts 列出账户 | 回签名 actAs 成员名下的余额户与保证金户, 无入参
func (client *Client) QueryAccounts(ctx context.Context) (*Response[[]Account], error) {
	return Call[[]Account](ctx, client, http.MethodGet, "/v2/account", nil)
}

// do 签名并发送 | 返回原始响应字节与 HTTP 状态码
func (client *Client) do(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	// 序列化请求体 | 签名用的字节必须与实际发送字节逐字节一致, 所以先序列化再签名
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("access: 序列化请求体失败: %w", err)
		}
	}

	// 签名
	generationTime := time.Now().UnixMilli()
	// 服务端按 request.baseUrl 重建规范串, query 不参与签名
	signPath, _, _ := strings.Cut(path, "?")
	signature, err := sign(signatureContent(signatureSpec{
		Method:         method,
		Path:           signPath,
		GenerationTime: generationTime,
		ActAs:          client.actAs,
		Payload:        string(body),
	}), client.signingKey)
	if err != nil {
		return nil, 0, err
	}

	// 发请求
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("access: 构造请求失败: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", authorizationHeader(client.usingKey, client.actAs, generationTime, signature))
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("access: 请求失败: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("access: 读取响应失败: %w", err)
	}
	return data, response.StatusCode, nil
}

// snippet 截断响应体片段 | 够排查即可, 不把整页错误 HTML 塞进 error
func snippet(body []byte) string {
	const limit = 512
	if len(body) > limit {
		return string(body[:limit]) + "…"
	}
	return string(body)
}
