package access

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testPrivateKey 测试私钥 | 与 typescript 版测试用的是同一把
func testPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := parsePrivateKey(testServicePrivateKey)
	if err != nil {
		t.Fatalf("解析测试私钥失败: %v", err)
	}
	return privateKey
}

// serveWith 起一个假接入点并返回指向它的客户端
func serveWith(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{
		BaseURL: server.URL,
		UserKey: UserKey{
			UniqueIdentifier:  testUsingKey,
			ServicePrivateKey: testServicePrivateKey,
		},
		TenantMember: TenantMember{UniqueIdentifier: testActAs},
	})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	return client
}

// verifyLikeBackend 按后端 IdentityVerificationMiddleware 的口径重建规范串并验签 | 顺带把签名参数还给用例检查
func verifyLikeBackend(t *testing.T, request *http.Request, rawBody []byte) url.Values {
	t.Helper()

	// 拆 Authorization 头
	authorization := request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Signature ") {
		t.Fatalf("Authorization 头格式不对: %q", authorization)
	}
	parameters, err := url.ParseQuery(strings.TrimPrefix(authorization, "Signature "))
	if err != nil {
		t.Fatalf("Authorization 头解析失败: %v", err)
	}
	for _, required := range []string{"usingKey", "actAs", "generationTime", "requestSignature"} {
		if parameters.Get(required) == "" {
			t.Fatalf("Authorization 头缺少 %s: %q", required, authorization)
		}
	}

	// 时效 | 客户端发的时间戳不该跟本地差出 5 分钟
	generationTime, err := strconv.ParseInt(parameters.Get("generationTime"), 10, 64)
	if err != nil {
		t.Fatalf("generationTime 不是整数: %q", parameters.Get("generationTime"))
	}
	if delta := absInt64(time.Now().UnixMilli() - generationTime); delta > 5*60*1000 {
		t.Fatalf("签名时间戳超出时效: %d", generationTime)
	}

	// 重建规范串 | 方法、路径、原始字节三者都取自收到的请求
	signature := signatureContent(signatureSpec{
		Method:         request.Method,
		Path:           request.URL.Path,
		GenerationTime: generationTime,
		ActAs:          parameters.Get("actAs"),
		Payload:        string(rawBody),
	})
	digest, err := base64.StdEncoding.DecodeString(parameters.Get("requestSignature"))
	if err != nil {
		t.Fatalf("requestSignature 不是合法 base64: %v", err)
	}
	sum := sha512.Sum512([]byte(signature))
	if err := rsa.VerifyPKCS1v15(&testPrivateKey(t).PublicKey, crypto.SHA512, sum[:], digest); err != nil {
		t.Fatalf("验签失败: %v\n规范串: %q", err, signature)
	}
	return parameters
}

func TestMakePaymentOrder(t *testing.T) {
	var verified url.Values
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", request.Method)
		}
		if request.URL.Path != "/v2/payment-order" {
			t.Errorf("path = %s, want /v2/payment-order", request.URL.Path)
		}
		if contentType := request.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Content-Type = %s", contentType)
		}
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		// 请求体字节必须与入参一致（收款侧只给币种时 value 段整个省略）
		if string(rawBody) != testPayload {
			t.Errorf("请求体不一致:\n got  %s\n want %s", rawBody, testPayload)
		}
		verified = verifyLikeBackend(t, request, rawBody)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"uniqueIdentifier":"po-1","order":{"uniqueIdentifier":"o-1","externalIdentifier":"32730576-e8c8-415f-a648-9b1568f1e992","type":"PAYMENT","status":"CREATED","paymentAmount":{"currency":"INR","value":100},"receiptAmount":{"currency":"INR","value":100},"createdAt":"2026-10-08T00:00:00.000Z","updatedAt":"2026-10-08T00:00:00.000Z"},"type":"UPI","receiptInformation":{"type":"UPI","identifier":"payee@okhdfcbank"}}}`))
	})

	value := int64(100)
	response, err := client.MakePaymentOrder(context.Background(), PaymentOrder{
		Order: OrderRequest{
			PaymentAmount:      Amount{Currency: CurrencyINR, Value: &value},
			ReceiptAmount:      Amount{Currency: CurrencyINR},
			ExternalIdentifier: "32730576-e8c8-415f-a648-9b1568f1e992",
			Product:            Product{Type: "ONLINE_PAY"},
		},
		ReceiptInformation: NewUPIReceipt("payee@okhdfcbank"),
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}

	// 签名参数
	if got := verified.Get("usingKey"); got != testUsingKey {
		t.Errorf("usingKey = %s, want %s", got, testUsingKey)
	}
	if got := verified.Get("actAs"); got != testActAs {
		t.Errorf("actAs = %s, want %s", got, testActAs)
	}

	// 响应解码
	if response.Data.Order.UniqueIdentifier != "o-1" {
		t.Errorf("order.uniqueIdentifier = %s", response.Data.Order.UniqueIdentifier)
	}
	if response.Data.Order.Status != OrderStatusCreated {
		t.Errorf("order.status = %s", response.Data.Order.Status)
	}
	if response.Data.ReceiptInformation == nil || response.Data.ReceiptInformation.Identifier != "payee@okhdfcbank" {
		t.Errorf("receiptInformation = %+v", response.Data.ReceiptInformation)
	}
	if response.Data.Order.PaymentAmount == nil || response.Data.Order.PaymentAmount.Value != 100 {
		t.Errorf("paymentAmount = %+v", response.Data.Order.PaymentAmount)
	}
	if response.Data.Order.CreatedAt.Year() != 2026 {
		t.Errorf("createdAt = %s", response.Data.Order.CreatedAt)
	}
}

func TestMakeCollectionOrder(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v2/collection-order" {
			t.Errorf("path = %s", request.URL.Path)
		}
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		// 代收不带收款信息
		if strings.Contains(string(rawBody), "receiptInformation") {
			t.Errorf("请求体不应含 receiptInformation: %s", rawBody)
		}
		// 付款侧缺金额、收款侧完整 | value 段只出现在给定的那一侧
		if want := `{"order":{"paymentAmount":{"currency":"INR"},"receiptAmount":{"currency":"INR","value":200},"externalIdentifier":"co-1","product":{"type":"ONLINE_RECEIVE"}}}`; string(rawBody) != want {
			t.Errorf("请求体不一致:\n got  %s\n want %s", rawBody, want)
		}
		verifyLikeBackend(t, request, rawBody)
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"uniqueIdentifier":"co-1","order":{"uniqueIdentifier":"o-2","status":"CREATED"}}}`))
	})

	value := int64(200)
	response, err := client.MakeCollectionOrder(context.Background(), CollectionOrder{
		Order: OrderRequest{
			PaymentAmount:      Amount{Currency: CurrencyINR},
			ReceiptAmount:      Amount{Currency: CurrencyINR, Value: &value},
			ExternalIdentifier: "co-1",
			Product:            Product{Type: "ONLINE_RECEIVE"},
		},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if response.Data.Order.UniqueIdentifier != "o-2" {
		t.Errorf("order.uniqueIdentifier = %s", response.Data.Order.UniqueIdentifier)
	}
}

func TestQueryPaymentOrder(t *testing.T) {
	const identifier = "96c89d4c-1111-4222-8333-444455556666"
	var signedPath string
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		if request.URL.Path != "/v2/payment-order/"+identifier {
			t.Errorf("path = %s", request.URL.Path)
		}
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		// 无请求体的 GET 也要签名 | 漏了 Authorization 头服务端直接拒
		if request.Header.Get("Authorization") == "" {
			t.Errorf("GET 未带签名头")
		}
		signedPath = request.URL.Path
		verifyLikeBackend(t, request, rawBody)
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"uniqueIdentifier":"po-1","order":{"uniqueIdentifier":"o-1","externalIdentifier":"32730576-e8c8-415f-a648-9b1568f1e992","status":"SUCCEEDED","paymentAmount":{"currency":"INR","value":100},"receiptAmount":{"currency":"INR","value":100},"createdAt":"2026-10-08T00:00:00.000Z","updatedAt":"2026-10-08T00:00:00.000Z"},"type":"UPI"}}`))
	})

	response, err := client.QueryPaymentOrder(context.Background(), identifier)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if signedPath != "/v2/payment-order/"+identifier {
		t.Errorf("签名路径 = %s", signedPath)
	}
	if response.Data.Order.Status != OrderStatusSucceeded {
		t.Errorf("order.status = %s", response.Data.Order.Status)
	}
}

func TestQueryCollectionOrder(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		// 外部单号同样拼进路径 | 平台单号与外部单号走同一个端点
		if request.URL.Path != "/v2/collection-order/co-1" {
			t.Errorf("path = %s", request.URL.Path)
		}
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		verifyLikeBackend(t, request, rawBody)
		_, _ = writer.Write([]byte(`{"code":"OK","data":{"uniqueIdentifier":"co-1","order":{"uniqueIdentifier":"o-2","status":"PROCESSING"}}}`))
	})

	response, err := client.QueryCollectionOrder(context.Background(), "co-1")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if response.Data.Order.Status != OrderStatusProcessing {
		t.Errorf("order.status = %s", response.Data.Order.Status)
	}
}

func TestQueryAccounts(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v2/account" {
			t.Errorf("path = %s", request.URL.Path)
		}
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		verifyLikeBackend(t, request, rawBody)
		_, _ = writer.Write([]byte(`{"code":"OK","data":[{"uniqueIdentifier":"acc-1","belongsTo":"MERCHANT","type":"BALANCE","currency":"INR","availableBalance":530,"frozenBalance":459},{"uniqueIdentifier":"acc-2","belongsTo":"MERCHANT","type":"MARGIN","currency":"INR","availableBalance":100,"frozenBalance":0}]}`))
	})

	response, err := client.QueryAccounts(context.Background())
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("账户数 = %d, want 2", len(response.Data))
	}
	balance := response.Data[0]
	if balance.Type != AccountTypeBalance || balance.AvailableBalance != 530 || balance.FrozenBalance != 459 {
		t.Errorf("余额户 = %+v", balance)
	}
	if response.Data[1].Type != AccountTypeMargin {
		t.Errorf("保证金户 = %+v", response.Data[1])
	}
}

func TestCallSignsPathWithoutQuery(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, request *http.Request) {
		rawBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读请求体失败: %v", err)
		}
		verifyLikeBackend(t, request, rawBody)
		_, _ = writer.Write([]byte(`{"code":"OK","data":null}`))
	})

	_, err := Call[json.RawMessage](context.Background(), client, http.MethodGet, "/v2/order?externalIdentifier=o-1", nil)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
}

func TestCallReturnsBusinessError(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, _ *http.Request) {
		// 幂等冲突按后端口径仍是 200 + 业务码
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"code":"ORDER_IDEMPOTENCY_CONFLICT","message":"同一外部单号已存在且金额不一致"}`))
	})

	value := int64(200)
	_, err := client.MakeCollectionOrder(context.Background(), CollectionOrder{
		Order: OrderRequest{
			PaymentAmount:      Amount{Currency: CurrencyINR, Value: &value},
			ReceiptAmount:      Amount{Currency: CurrencyINR},
			ExternalIdentifier: "co-1",
			Product:            Product{Type: "ONLINE_RECEIVE"},
		},
	})
	var businessError *Error
	if !errors.As(err, &businessError) {
		t.Fatalf("err = %v, want *access.Error", err)
	}
	if businessError.Code != "ORDER_IDEMPOTENCY_CONFLICT" {
		t.Errorf("code = %s", businessError.Code)
	}
	if !strings.Contains(businessError.Error(), "ORDER_IDEMPOTENCY_CONFLICT") {
		t.Errorf("Error() = %s", businessError.Error())
	}
}

func TestCallReturnsTransportError(t *testing.T) {
	client := serveWith(t, func(writer http.ResponseWriter, _ *http.Request) {
		// 网关侧的非 JSON 响应
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("<html>bad gateway</html>"))
	})

	_, err := client.MakePaymentOrder(context.Background(), PaymentOrder{
		Order: OrderRequest{
			PaymentAmount:      Amount{Currency: CurrencyINR, Value: ptr(int64(100))},
			ReceiptAmount:      Amount{Currency: CurrencyINR},
			ExternalIdentifier: "po-1",
			Product:            Product{Type: "ONLINE_PAY"},
		},
		ReceiptInformation: NewUPIReceipt("payee@okhdfcbank"),
	})
	var statusError *HTTPStatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("err = %v, want *access.HTTPStatusError", err)
	}
	if statusError.StatusCode != http.StatusBadGateway {
		t.Errorf("statusCode = %d", statusError.StatusCode)
	}
}

// ptr 取地址 | 构造 Amount.Value
func ptr(value int64) *int64 {
	return &value
}

// absInt64 取绝对值
func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
