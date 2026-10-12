package access

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testNotifyBody 平台推送的业务单据报文片段 | 形状取自 OrderCallbackService.buildMerchantNotifyBody
// 的实际序列化结果（代收推 collection_orders 那一行, 订单包在 order 里, 不带 {code,data} 信封,
// fees/feeGroups 已被剔除, channel 只剩 name）
const testNotifyBody = `{"identifier":7,"uniqueIdentifier":"6f0f6c1a-2c94-4c1c-9c2f-2a1d3e4b5c6d","redirectURL":"https://checkout.example.com/pay/6f0f6c1a","returnURL":"https://shop.example.com/orders/DJP240101ABCDEFGH/paid","order":{"identifier":1201,"uniqueIdentifier":"9a8b7c6d-5e4f-4a1b-8c9d-0e1f2a3b4c5d","externalIdentifier":"DJP240101ABCDEFGH","internalIdentifier":"CH-2026-0001","status":"SUCCEEDED","type":"COLLECTION","amount":{"currency":"INR","value":100000},"paymentAmount":{"currency":"INR","value":100000},"receiptAmount":{"currency":"USDT","value":11700000},"product":{"type":"ONLINE_RECEIVE"},"callbackURL":"https://shop.example.com/api/v1/payments/webhook/reroute","channel":{"name":"Reroute Money"},"createdAt":"2026-10-09T00:00:00.000Z","updatedAt":"2026-10-09T00:01:00.000Z"}}`

const (
	testNotifyPath           = "/api/v1/payments/webhook/reroute"
	testNotifyActAs          = "-"
	testNotifyUsingKey       = "5b1c0e2d-7a3f-4c8b-9d0e-1f2a3b4c5d6e"
	testNotifyGeneration     = int64(1700000000000)
	testNotifyGenerationText = "1700000000000"
)

// generateTestKeyPair 造一对测试密钥 | 私钥签推送、公钥验推送, SPKI PEM 与后端 user-key resolver 产出一致
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("导出公钥失败: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return privateKey, string(publicPEM)
}

// signAsPlatform 按平台出站推送口径签名 | 与 IdentityVerificationUtility.buildHeaderWithSignature 同规则
func signAsPlatform(t *testing.T, privateKey *rsa.PrivateKey, content string) string {
	t.Helper()

	digest := sha512.Sum512([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA512, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

// notifyAuthorization 拼推送侧的 Authorization 头 | 参数顺序照 Node URLSearchParams 的插入顺序
// （usingKey、generationTime、requestSignature、actAs）, 与客户端出站签名顺序不同, 服务端按名取值
func notifyAuthorization(t *testing.T, requestSignature string, generationTime int64, actAs string) string {
	t.Helper()

	return "Signature " + strings.Join([]string{
		"usingKey=" + url.QueryEscape(testNotifyUsingKey),
		"generationTime=" + strconv.FormatInt(generationTime, 10),
		"requestSignature=" + url.QueryEscape(requestSignature),
		"actAs=" + url.QueryEscape(actAs),
	}, "&")
}

func TestVerifyMerchantNotifyAcceptsPlatformSignature(t *testing.T) {
	privateKey, publicPEM := generateTestKeyPair(t)

	// 规范串逐字锚定 | 平台改一个字节商户就验不过, 测试先固定住口径
	content := "POST /api/v1/payments/webhook/reroute\r\n\r\nGeneration-Time: " + testNotifyGenerationText + "\r\nAct-As: -\r\n\r\n" + testNotifyBody
	want := signatureContent(signatureSpec{
		Method:         "POST",
		Path:           testNotifyPath,
		GenerationTime: testNotifyGeneration,
		ActAs:          testNotifyActAs,
		Payload:        testNotifyBody,
	})
	if content != want {
		t.Fatalf("规范串与平台出站口径不一致:\n got=%q\nwant=%q", content, want)
	}

	requestSignature := signAsPlatform(t, privateKey, content)
	authorization := notifyAuthorization(t, requestSignature, testNotifyGeneration, testNotifyActAs)

	err := VerifyMerchantNotify(publicPEM, NotifyVerification{
		CallbackPath:  testNotifyPath,
		Authorization: authorization,
		Body:          []byte(testNotifyBody),
		Now:           time.UnixMilli(testNotifyGeneration),
	})
	if err != nil {
		t.Fatalf("合法推送验签失败: %v", err)
	}
}

func TestVerifyMerchantNotifyRejects(t *testing.T) {
	privateKey, publicPEM := generateTestKeyPair(t)
	content := signatureContent(signatureSpec{
		Method:         "POST",
		Path:           testNotifyPath,
		GenerationTime: testNotifyGeneration,
		ActAs:          testNotifyActAs,
		Payload:        testNotifyBody,
	})
	validAuthorization := notifyAuthorization(t, signAsPlatform(t, privateKey, content), testNotifyGeneration, testNotifyActAs)
	otherKey, _ := generateTestKeyPair(t)
	otherAuthorization := notifyAuthorization(t, signAsPlatform(t, otherKey, content), testNotifyGeneration, testNotifyActAs)

	for _, testCase := range []struct {
		name         string
		verification NotifyVerification
		wantErr      error
	}{
		{
			name: "报文被改一个字节",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: validAuthorization,
				Body:          []byte(strings.Replace(testNotifyBody, "SUCCEEDED", "FAILED", 1)),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrSignatureInvalid,
		},
		{
			name: "跳转路径与实际登记不一致",
			verification: NotifyVerification{
				CallbackPath:  "/api/v1/payments/callback",
				Authorization: validAuthorization,
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrSignatureInvalid,
		},
		{
			name: "公钥不配对",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: otherAuthorization,
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrSignatureInvalid,
		},
		{
			name: "时间戳超出窗口",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: validAuthorization,
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration).Add(DefaultNotifyMaxSkew + time.Minute),
			},
			wantErr: ErrSignatureExpired,
		},
		{
			name: "缺少签名前缀",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: strings.TrimPrefix(validAuthorization, "Signature "),
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrAuthorizationInvalid,
		},
		{
			name: "缺少 requestSignature",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: "Signature usingKey=" + testNotifyUsingKey + "&actAs=-&generationTime=" + testNotifyGenerationText,
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrAuthorizationInvalid,
		},
		{
			name: "generationTime 非数字",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: "Signature usingKey=" + testNotifyUsingKey + "&actAs=-&generationTime=abc&requestSignature=c2ln",
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrAuthorizationInvalid,
		},
		{
			name: "requestSignature 非 base64",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: notifyAuthorization(t, "!!!not-base64!!!", testNotifyGeneration, testNotifyActAs),
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrAuthorizationInvalid,
		},
		{
			name: "公钥不是 PEM",
			verification: NotifyVerification{
				CallbackPath:  testNotifyPath,
				Authorization: validAuthorization,
				Body:          []byte(testNotifyBody),
				Now:           time.UnixMilli(testNotifyGeneration),
			},
			wantErr: ErrPublicKeyInvalid,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			publicPEM := publicPEM
			if errors.Is(testCase.wantErr, ErrPublicKeyInvalid) {
				publicPEM = "not a pem"
			}
			err := VerifyMerchantNotify(publicPEM, testCase.verification)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("期望 %v, 实际 %v", testCase.wantErr, err)
			}
		})
	}
}

func TestVerifyMerchantNotifyMissingCallbackPath(t *testing.T) {
	_, publicPEM := generateTestKeyPair(t)
	err := VerifyMerchantNotify(publicPEM, NotifyVerification{
		Authorization: "Signature usingKey=k&actAs=-&generationTime=1700000000000&requestSignature=c2ln",
	})
	if err == nil || !strings.Contains(err.Error(), "CallbackPath") {
		t.Fatalf("期望指出缺少 CallbackPath, 实际 %v", err)
	}
}

func TestVerifyMerchantNotifyNowDefaultsToCurrentTime(t *testing.T) {
	// Now 留零值时按当前时间判窗口, 商户不必显式传
	privateKey, publicPEM := generateTestKeyPair(t)
	generationTime := time.Now().Add(time.Minute).UnixMilli()
	content := signatureContent(signatureSpec{
		Method:         "POST",
		Path:           testNotifyPath,
		GenerationTime: generationTime,
		ActAs:          testNotifyActAs,
		Payload:        testNotifyBody,
	})
	authorization := notifyAuthorization(t, signAsPlatform(t, privateKey, content), generationTime, testNotifyActAs)

	if err := VerifyMerchantNotify(publicPEM, NotifyVerification{
		CallbackPath:  testNotifyPath,
		Authorization: authorization,
		Body:          []byte(testNotifyBody),
	}); err != nil {
		t.Fatalf("Now 零值应回落当前时间, 实际失败: %v", err)
	}
}

func TestParseMerchantNotify(t *testing.T) {
	notify, err := ParseMerchantNotify([]byte(testNotifyBody))
	if err != nil {
		t.Fatalf("解报文失败: %v", err)
	}
	// 顶层是单据那一行 | 商户拿去查单、存作自己那笔交易引用的号
	if notify.UniqueIdentifier != "6f0f6c1a-2c94-4c1c-9c2f-2a1d3e4b5c6d" {
		t.Errorf("uniqueIdentifier = %q", notify.UniqueIdentifier)
	}
	if notify.RedirectURL != "https://checkout.example.com/pay/6f0f6c1a" {
		t.Errorf("redirectURL = %q", notify.RedirectURL)
	}
	if notify.ReturnURL != "https://shop.example.com/orders/DJP240101ABCDEFGH/paid" {
		t.Errorf("returnURL = %q", notify.ReturnURL)
	}
	// 交易事实在 order 那一层
	if notify.Order.ExternalIdentifier != "DJP240101ABCDEFGH" {
		t.Errorf("order.externalIdentifier = %q", notify.Order.ExternalIdentifier)
	}
	if notify.Order.InternalIdentifier != "CH-2026-0001" {
		t.Errorf("order.internalIdentifier = %q", notify.Order.InternalIdentifier)
	}
	if notify.Order.Status != OrderStatusSucceeded {
		t.Errorf("order.status = %q", notify.Order.Status)
	}
	if notify.Order.Type != OrderTypeCollection {
		t.Errorf("order.type = %q", notify.Order.Type)
	}
	if notify.Order.PaymentAmount == nil || notify.Order.PaymentAmount.Currency != CurrencyINR || notify.Order.PaymentAmount.Value != 100000 {
		t.Errorf("order.paymentAmount = %+v", notify.Order.PaymentAmount)
	}
	if notify.Order.ReceiptAmount == nil || notify.Order.ReceiptAmount.Currency != CurrencyUSDT || notify.Order.ReceiptAmount.Value != 11700000 {
		t.Errorf("order.receiptAmount = %+v", notify.Order.ReceiptAmount)
	}
	if notify.Order.CallbackURL != "https://shop.example.com/api/v1/payments/webhook/reroute" {
		t.Errorf("order.callbackURL = %q", notify.Order.CallbackURL)
	}
	// 两个号不是一回事 | order.uniqueIdentifier 是 orders 表的号, 查单接口不认它, 别拿它覆盖单据号
	if notify.Order.UniqueIdentifier == "" || notify.Order.UniqueIdentifier == notify.UniqueIdentifier {
		t.Errorf("order.uniqueIdentifier = %q, 顶层 = %q", notify.Order.UniqueIdentifier, notify.UniqueIdentifier)
	}
	// 代收单不带代付那两列
	if notify.Type != "" || notify.ReceiptInformation != nil {
		t.Errorf("代收单不该带收款方式或收款信息, 实际 type=%q receiptInformation=%+v", notify.Type, notify.ReceiptInformation)
	}
}

func TestParseMerchantNotifyPaymentDocument(t *testing.T) {
	body := `{"uniqueIdentifier":"2b7d4c9e-1a3f-4d2c-8e5b-6a9c0d1e2f34","type":"UPI","receiptInformation":{"type":"UPI","identifier":"payee@okhdfcbank"},"order":{"uniqueIdentifier":"9a8b7c6d-5e4f-4a1b-8c9d-0e1f2a3b4c5d","externalIdentifier":"DJP240101IJKLMNOP","status":"SUCCEEDED","type":"PAYMENT","paymentAmount":{"currency":"INR","value":50000}}}`

	notify, err := ParseMerchantNotify([]byte(body))
	if err != nil {
		t.Fatalf("解报文失败: %v", err)
	}
	if notify.UniqueIdentifier != "2b7d4c9e-1a3f-4d2c-8e5b-6a9c0d1e2f34" {
		t.Errorf("uniqueIdentifier = %q", notify.UniqueIdentifier)
	}
	// 代付单顶层的 type 是收款方式, 不是单据种类（后者在 order.type）
	if notify.Type != ReceiptTypeUPI {
		t.Errorf("type = %q, 应为收款方式 UPI", notify.Type)
	}
	if notify.Order.Type != OrderTypePayment {
		t.Errorf("order.type = %q", notify.Order.Type)
	}
	if notify.ReceiptInformation == nil || notify.ReceiptInformation.Identifier != "payee@okhdfcbank" {
		t.Errorf("receiptInformation = %+v", notify.ReceiptInformation)
	}
	if notify.RedirectURL != "" {
		t.Errorf("代付单不该带回跳地址, 实际 redirectURL = %q", notify.RedirectURL)
	}
}

func TestParseMerchantNotifyRejectsMalformedBody(t *testing.T) {
	if _, err := ParseMerchantNotify([]byte(`<html>bad gateway</html>`)); !errors.Is(err, ErrNotifyBodyInvalid) {
		t.Fatalf("期望 ErrNotifyBodyInvalid, 实际 %v", err)
	}
}

func TestParsePublicKeyAcceptsBothEncodings(t *testing.T) {
	privateKey, spkiPEM := generateTestKeyPair(t)

	parsed, err := parsePublicKey(spkiPEM)
	if err != nil {
		t.Fatalf("SPKI 公钥解析失败: %v", err)
	}
	if parsed.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Fatal("SPKI 解析出的公钥与私钥不匹配")
	}

	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	}))
	if _, err := parsePublicKey(pkcs1PEM); err != nil {
		t.Fatalf("PKCS#1 公钥解析失败: %v", err)
	}

	if _, err := parsePublicKey("-----BEGIN PUBLIC KEY-----\nZm9v\n-----END PUBLIC KEY-----\n"); !errors.Is(err, ErrPublicKeyInvalid) {
		t.Fatalf("期望 ErrPublicKeyInvalid, 实际 %v", err)
	}
}
