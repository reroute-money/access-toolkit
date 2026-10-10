package access

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// 测试身份 | 与 typescript/tests/index.test.ts 用的是同一把 servicePrivateKey
const (
	testUsingKey         = "32d61b1d-5ec6-42f3-aff4-65d5086ec630"
	testActAs            = "9462318f-b3dc-4559-8c4a-c0cda9fd5e7c"
	testGeneration int64 = 1700000000000
	testPayload          = `{"order":{"paymentAmount":{"currency":"INR","value":100},"receiptAmount":{"currency":"INR"},"externalIdentifier":"32730576-e8c8-415f-a648-9b1568f1e992","product":{"type":"ONLINE_PAY"}},"receiptInformation":{"type":"UPI","identifier":"payee@okhdfcbank"}}`
)

const testServicePrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEwAIBADANBgkqhkiG9w0BAQEFAASCBKowggSmAgEAAoIBAQDZpBg0bb6IUfok
jPVr1yY53j4JH+MrAS1nZKu+ihoEKihtkJMCOUu/keJ1kmW9XlkIaFSc7jKRqeMD
0S8ktkJ/EKEFH8SV918MlNio+oFuC3CnFHJlahdziOObY1ihm+KiVkrLDBR7NjhW
eq5g67SZDkMYdnkYKje2dlRh9NL30V1VEadO1Ld0O9GvqX5CsZFVfIl7Ica+tzL9
xNAmXJtYvTBeEq3eT1q0yq6qcqbPKDooeTaV7FmSZ4G/ZMyBhE3Xhuv5IQBxEJ7V
fkItnhOVVOERH/48umlA081GHkyHBTTiM6GI/w0f/KmczxwLpvB2Qq1masWX9gU5
/cH6H+ZzAgMBAAECggEBALTmEPxbD5htUxZYV5GoReypjjS1XXOcSen7okkArvkx
NWNEXPznNM/paSrCYXBSYHx9g37g+gVbCQdeMuo31/OXlc8VhagE8esaOS/S8AvQ
1HT7qM4krZNpf43juIKzeF/oi8hTpR54hcFTvWdv4y+ozBokNyjfxmkR17eH8Gbs
BJMJCLnKj1HI4Z52jIP9Ln/fTh49HIkIgd7RPI7BTraGJv8Ai2xitLnabI2OVWBn
bNJGr9YL/GSoYwPsAUwlTLXXoBRsheqLW73+JT972pv0EdJ4nMY1lAeb5fbmOCDW
g7VAvhYg9Ll/FwFHN731eAxGPdmfbLBxvo0j8fwygLkCgYEA+3P/d60YmiQnSbob
YGPZoAs/zq9nd676+VGXQ0SzQkAQXDM8I3LXN6L/ZyHBI0ztOSTPWM1vUcvhdH4j
9jo5So0vD7BcA7VNop7jtBBHTeq1yluSF6xH9xK+1HwD+w3t+38S6gjFqGV09+/s
8BN0WpqHlXjnKKImzfFC0SyvMZUCgYEA3ZOTreabE3Jgc2ptmIDeFliMYM0TUeKe
fwg2w7FTtwENDNa7XaIXg2AS/jh9sRm8gSF/rrMJIc7gNcEfujxD14vMHUA/Dp/Z
fq0z5+bYzeRDZEJRk2MlbG1F03As8UqNcH7bngoHl76/Fx0rXhgojnP3sHBmlRbJ
/XOgKW+5RecCgYEA5JvWdDi0OFeMPJRuFVlSXxY7Ia8I85/rX0aVrjvEegQCm8Cc
+WGMV3HioCHpFS34Ms7LkGHdxlEFHcMc8NyIf8YSNkJRtbFc7u9j964hFa7BLkPI
c6tXFkM9qbNCbzp/1fn2+T2Wbn4nNM817CLT0xfREs0D5EZf3zZsV/43HW0CgYEA
gBY4j5W4XbSHtxB2DGuqdqZ3d9n06igj0XFxHw0aRLcx0RMsusLh7ssSD62eU0t6
Yfd4dQgAJnANwmVCrwj5etfQhbRG/3A+bXE4iT00ZUkoAAQqXhY5YwWEEOdOw6mC
O97JnAFWRE067+Sn1jwG7dpx+6fuVQ8YcxBtLKbuJk8CgYEAuajcliYRE5MXP+Si
rTLZykLxGaV6Iu2lWyirIcdx/SLX1iJDzjKPJos/QheWicYx3qAivpB7Jj6f7vGp
IHaYunfe0n2aUDWBzvCpM7o230ixzXkeIMkSIMykKsPCkM5OpfGvQbFQQHJcWvGO
uxUc8bRcT39FXoiVDVThsNP9PhQ=
-----END RSA PRIVATE KEY-----`

// goldenRequestSignature Node `createSign('sha512')` 对同一规范串的签名结果 | RSA PKCS#1 v1.5 确定性签名, 可直接比字节
const goldenRequestSignature = "gDQlNdk41s9zWHE3tD/rz6nsFhqiVSwHQnJEFWVRQKh9vdzvkSzOMHljG1RMOAufAA0hFBXEOluBvZKkupZjt57hrOmGxPA9JkkBhbTMNQp1lvXlPsnpQCRtcLItjmcrNlkMsEg3kJgexIr2qu21j31r2mHADSINK6qz652SE7Oq8fp1/5cLvRPN721vepprkbi/ECqVjQDOhGk98cFjlTIpqQqSK1qHtB+nYVmtnR+xQn5y4yhicKtRytXBRWCu08xm3dtvcY/J+gvWUmtCVfuC9IHYpAzAKA/ImnC1kwWvb1NKBLeMptSxdWuadVuq0mYgci1lCSK/jYA33KVlhw=="

// goldenAuthorizationHeader Node `URLSearchParams` 序列化出来的 Authorization 头
const goldenAuthorizationHeader = "Signature usingKey=32d61b1d-5ec6-42f3-aff4-65d5086ec630&actAs=9462318f-b3dc-4559-8c4a-c0cda9fd5e7c&generationTime=1700000000000&requestSignature=gDQlNdk41s9zWHE3tD%2Frz6nsFhqiVSwHQnJEFWVRQKh9vdzvkSzOMHljG1RMOAufAA0hFBXEOluBvZKkupZjt57hrOmGxPA9JkkBhbTMNQp1lvXlPsnpQCRtcLItjmcrNlkMsEg3kJgexIr2qu21j31r2mHADSINK6qz652SE7Oq8fp1%2F5cLvRPN721vepprkbi%2FECqVjQDOhGk98cFjlTIpqQqSK1qHtB%2BnYVmtnR%2BxQn5y4yhicKtRytXBRWCu08xm3dtvcY%2FJ%2BgvWUmtCVfuC9IHYpAzAKA%2FImnC1kwWvb1NKBLeMptSxdWuadVuq0mYgci1lCSK%2FjYA33KVlhw%3D%3D"

func TestSignatureContent(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		spec     signatureSpec
		expected string
	}{
		{
			name: "带请求体",
			spec: signatureSpec{
				Method:         "PUT",
				Path:           "/v2/payment-order",
				GenerationTime: testGeneration,
				ActAs:          testActAs,
				Payload:        testPayload,
			},
			expected: "PUT /v2/payment-order\r\n\r\nGeneration-Time: 1700000000000\r\nAct-As: " + testActAs + "\r\n\r\n" + testPayload,
		},
		{
			name: "无请求体不拼尾段",
			spec: signatureSpec{
				Method:         "get",
				Path:           "/v2/collection-order",
				GenerationTime: testGeneration,
				ActAs:          testActAs,
			},
			expected: "GET /v2/collection-order\r\n\r\nGeneration-Time: 1700000000000\r\nAct-As: " + testActAs,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := signatureContent(testCase.spec); got != testCase.expected {
				t.Fatalf("规范串不一致:\n got  %q\n want %q", got, testCase.expected)
			}
		})
	}
}

func TestSignMatchesNode(t *testing.T) {
	privateKey, err := parsePrivateKey(testServicePrivateKey)
	if err != nil {
		t.Fatalf("解析私钥失败: %v", err)
	}

	content := signatureContent(signatureSpec{
		Method:         "PUT",
		Path:           "/v2/payment-order",
		GenerationTime: testGeneration,
		ActAs:          testActAs,
		Payload:        testPayload,
	})
	signature, err := sign(content, privateKey)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if signature != goldenRequestSignature {
		t.Fatalf("签名结果与 Node 不一致:\n got  %s\n want %s", signature, goldenRequestSignature)
	}
}

func TestAuthorizationHeaderMatchesNode(t *testing.T) {
	header := authorizationHeader(testUsingKey, testActAs, testGeneration, goldenRequestSignature)
	if header != goldenAuthorizationHeader {
		t.Fatalf("Authorization 头与 Node 不一致:\n got  %s\n want %s", header, goldenAuthorizationHeader)
	}
}

func TestParsePrivateKey(t *testing.T) {
	pkcs1, err := parsePrivateKey(testServicePrivateKey)
	if err != nil {
		t.Fatalf("PKCS#1 解析失败: %v", err)
	}

	// PKCS#8 | 同一把密钥换个封装格式也要能解析
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(pkcs1)
	if err != nil {
		t.Fatalf("转 PKCS#8 失败: %v", err)
	}
	pkcs8PEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))
	pkcs8, err := parsePrivateKey(pkcs8PEM)
	if err != nil {
		t.Fatalf("PKCS#8 解析失败: %v", err)
	}
	if !pkcs1.Equal(pkcs8) {
		t.Fatal("PKCS#1 与 PKCS#8 解析出的密钥不是同一把")
	}

	// 非法输入
	for _, invalid := range []string{"", "not a pem", "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----"} {
		if _, err := parsePrivateKey(invalid); err == nil {
			t.Fatalf("非法私钥 %q 应当解析失败", invalid)
		}
	}
}

func TestNewClientValidatesConfig(t *testing.T) {
	base := Config{
		UserKey: UserKey{
			UniqueIdentifier:  testUsingKey,
			ServicePrivateKey: testServicePrivateKey,
		},
		TenantMember: TenantMember{UniqueIdentifier: testActAs},
	}

	// 正常构造 | 默认地址去掉末尾斜杠
	client, err := NewClient(base)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %s, want %s", client.baseURL, DefaultBaseURL)
	}
	trailingSlash := base
	trailingSlash.BaseURL = "http://127.0.0.1:4001/"
	client, err = NewClient(trailingSlash)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if client.baseURL != "http://127.0.0.1:4001" {
		t.Fatalf("baseURL = %s, 末尾斜杠应被去掉", client.baseURL)
	}

	// 必填项缺失 / 私钥坏 | 都要在建客户端时就失败
	for _, broken := range []func(Config) Config{
		func(config Config) Config { config.UserKey.UniqueIdentifier = ""; return config },
		func(config Config) Config { config.TenantMember.UniqueIdentifier = ""; return config },
		func(config Config) Config { config.UserKey.ServicePrivateKey = "garbage"; return config },
	} {
		if _, err := NewClient(broken(base)); err == nil {
			t.Fatal("非法配置应当构造失败")
		}
	}
}
