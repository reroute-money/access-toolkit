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

// testNotifyBody 平台推送的 Order 实体报文片段 | 字段形状取自 OrderCallbackService.pushMerchantNotify
// 的实际序列化结果（Order 实体本身, 不带 {code,data} 信封, fees/feeGroups 已被剔除, channel 只剩 name）
const testNotifyBody = `{"identifier":7,"uniqueIdentifier":"6f0f6c1a-2c94-4c1c-9c2f-2a1d3e4b5c6d","externalIdentifier":"DJP240101ABCDEFGH","belongsTo":"MERCHANT","amount":{"currency":"INR","value":100000},"paymentAmount":{"currency":"INR","value":100000},"receiptAmount":{"currency":"USDT","value":11700000},"status":"SUCCEEDED","type":"COLLECTION","product":{"type":"ONLINE_RECEIVE"},"callbackURL":"https://shop.example.com/api/v1/payments/webhook/reroute","channel":{"name":"Reroute Money"},"createdAt":"2026-10-09T00:00:00.000Z","updatedAt":"2026-10-09T00:01:00.000Z"}`

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
	if notify.UniqueIdentifier != "6f0f6c1a-2c94-4c1c-9c2f-2a1d3e4b5c6d" {
		t.Errorf("uniqueIdentifier = %q", notify.UniqueIdentifier)
	}
	if notify.ExternalIdentifier != "DJP240101ABCDEFGH" {
		t.Errorf("externalIdentifier = %q", notify.ExternalIdentifier)
	}
	if notify.Status != OrderStatusSucceeded {
		t.Errorf("status = %q", notify.Status)
	}
	if notify.Type != OrderTypeCollection {
		t.Errorf("type = %q", notify.Type)
	}
	if notify.PaymentAmount == nil || notify.PaymentAmount.Currency != CurrencyINR || notify.PaymentAmount.Value != 100000 {
		t.Errorf("paymentAmount = %+v", notify.PaymentAmount)
	}
	if notify.ReceiptAmount == nil || notify.ReceiptAmount.Currency != CurrencyUSDT || notify.ReceiptAmount.Value != 11700000 {
		t.Errorf("receiptAmount = %+v", notify.ReceiptAmount)
	}
	if notify.CallbackURL != "https://shop.example.com/api/v1/payments/webhook/reroute" {
		t.Errorf("callbackURL = %q", notify.CallbackURL)
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
