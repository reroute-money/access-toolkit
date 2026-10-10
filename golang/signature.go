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
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// CodeOK 信封成功码 | 与后端 stateful-query 拦截器返回的 code 对齐
const CodeOK = "OK"

// signatureSpec 出站签名要素 | 与后端 IdentityVerificationUtility.buildSignatureContent 的入参一一对应
type signatureSpec struct {
	// HTTP 方法 | 大写后进规范串
	Method string
	// 请求路径 | 不含 host 与 query, 必须与服务端 request.baseUrl 逐字一致
	Path string
	// 签名时间戳 | 毫秒, 与 Authorization 内 generationTime 同值
	GenerationTime int64
	// 扮演角色（租户成员编号）
	ActAs string
	// 请求体原始串 | 有体才拼, 必须与实际发送字节逐字节一致
	Payload string
}

// signatureContent 拼接口签名规范串
//
// 形如 `{METHOD} {path}\r\n\r\nGeneration-Time: {t}\r\nAct-As: {a}[\r\n\r\n{payload}]`,
// 与后端 `IdentityVerificationUtility.buildSignatureContent` 同源；改一个字节服务端就验不过。
func signatureContent(spec signatureSpec) string {
	var builder strings.Builder
	builder.WriteString(strings.ToUpper(spec.Method))
	builder.WriteByte(' ')
	builder.WriteString(spec.Path)
	builder.WriteString("\r\n\r\nGeneration-Time: ")
	builder.WriteString(strconv.FormatInt(spec.GenerationTime, 10))
	builder.WriteString("\r\nAct-As: ")
	builder.WriteString(spec.ActAs)
	if spec.Payload != "" {
		builder.WriteString("\r\n\r\n")
		builder.WriteString(spec.Payload)
	}
	return builder.String()
}

// sign RSA-SHA512 + PKCS#1 v1.5, 输出 base64 | 与 Node `createSign('sha512')` 同结果（该方案确定性签名, 可直接比字节）
func sign(content string, privateKey *rsa.PrivateKey) (string, error) {
	digest := sha512.Sum512([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA512, digest[:])
	if err != nil {
		return "", fmt.Errorf("access: 签名失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// authorizationHeader 拼 Authorization 头 | `Signature usingKey=?&actAs=?&generationTime=?&requestSignature=?`
//
// 按顺序手工拼串而非 url.Values.Encode(): 后者会按 key 排序, 这里要保持与 Node `URLSearchParams` 一致的插入顺序
// （服务端按参数名取值, 顺序不影响验签, 但商户侧比对报文时少一处意外）。
func authorizationHeader(usingKey, actAs string, generationTime int64, requestSignature string) string {
	parameters := [][2]string{
		{"usingKey", usingKey},
		{"actAs", actAs},
		{"generationTime", strconv.FormatInt(generationTime, 10)},
		{"requestSignature", requestSignature},
	}
	var builder strings.Builder
	builder.WriteString("Signature ")
	for index, parameter := range parameters {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(url.QueryEscape(parameter[0]))
		builder.WriteByte('=')
		builder.WriteString(url.QueryEscape(parameter[1]))
	}
	return builder.String()
}

// parsePrivateKey 解析 servicePrivateKey | 支持 PKCS#1（`RSA PRIVATE KEY`）与 PKCS#8（`PRIVATE KEY`）PEM
func parsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("access: servicePrivateKey 不是合法的 PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if err := key.Validate(); err != nil {
			return nil, fmt.Errorf("access: servicePrivateKey 不可用: %w", err)
		}
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("access: servicePrivateKey 解析失败（仅支持 RSA 的 PKCS#1 / PKCS#8 PEM）")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("access: servicePrivateKey 类型应为 RSA, 实际 %T", parsed)
	}
	return key, nil
}
