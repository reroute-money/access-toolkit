package access

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultNotifyMaxSkew 推送时间戳允许偏移 | 与后端对入站签名施加的 5 分钟窗口一致
const DefaultNotifyMaxSkew = 5 * time.Minute

// 验签失败原因 | 让商户区分「报文是伪造的」「报文过期了」与「自己配错了」
var (
	// ErrAuthorizationInvalid Authorization 头缺失或不是合法的签名参数串
	ErrAuthorizationInvalid = errors.New("access: Authorization 头不是合法的签名参数")
	// ErrSignatureExpired generationTime 超出允许窗口
	ErrSignatureExpired = errors.New("access: 平台推送签名已过期")
	// ErrSignatureInvalid 签名与报文不匹配（伪造、被改, 或公钥配错）
	ErrSignatureInvalid = errors.New("access: 平台推送签名不匹配")
	// ErrPublicKeyInvalid platformPublicKey 解析不了
	ErrPublicKeyInvalid = errors.New("access: platformPublicKey 不可用")
	// ErrNotifyBodyInvalid 推送报文不是合法 JSON
	ErrNotifyBodyInvalid = errors.New("access: 平台推送报文不是合法 JSON")
)

// NotifyVerification 验签订单推送所需的要素
type NotifyVerification struct {
	// CallbackPath 下单时登记的 callbackURL 的 pathname
	//
	// 平台拼规范串用的是 `new URL(callbackURL).pathname`（见 OrderCallbackService.pushMerchantNotify）,
	// 与请求实际到达商户时的路径无关 —— 商户挂在反代子路径下时两者会错开, 所以必须以配置值为准,
	// 不能取请求自己的 URL.Path。
	CallbackPath string
	// Authorization 请求头 Authorization 原文 | 形如 `Signature usingKey=..&actAs=..&generationTime=..&requestSignature=..`
	Authorization string
	// Body 收到的原始请求体字节 | 必须与平台发送字节逐字一致, 先验签再解报文
	Body []byte
	// Now 判定时间戳新鲜度的基准时间 | 零值取 time.Now()
	Now time.Time
	// MaxSkew 允许的时间戳偏移 | 零值取 DefaultNotifyMaxSkew
	MaxSkew time.Duration
}

// VerifyMerchantNotify 用 platformPublicKey 验订单推送签名 | RSA-SHA512 + PKCS#1 v1.5 + base64
//
// 规范串复用包内 signatureContent, 与后端 IdentityVerificationUtility.buildSignatureContent 同源,
// 改一个字节即验不过。签名参数全部取报文自带的值重建, 不写死出站那侧的 actAs 取值。
func VerifyMerchantNotify(publicKeyPEM string, verification NotifyVerification) error {
	// 校验重建规范串所需的输入
	if strings.TrimSpace(verification.CallbackPath) == "" {
		return fmt.Errorf("access: 缺少 NotifyVerification.CallbackPath, 无法重建规范串")
	}

	// 取签名参数
	parameters, err := parseAuthorization(verification.Authorization)
	if err != nil {
		return err
	}

	// 时间戳窗口 | 过期的合法签名也不接受, 免得重放窗口无限长
	now := verification.Now
	if now.IsZero() {
		now = time.Now()
	}
	maxSkew := verification.MaxSkew
	if maxSkew <= 0 {
		maxSkew = DefaultNotifyMaxSkew
	}
	skew := now.Sub(time.UnixMilli(parameters.generationTime)).Abs()
	if skew > maxSkew {
		return fmt.Errorf("%w: 偏移 %v, 允许 %v", ErrSignatureExpired, skew, maxSkew)
	}

	// 解公钥
	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return err
	}

	// 重建规范串 | 请求体原样参与, 空体不拼该段（与发送侧同一套规则）
	content := signatureContent(signatureSpec{
		Method:         http.MethodPost,
		Path:           verification.CallbackPath,
		GenerationTime: parameters.generationTime,
		ActAs:          parameters.actAs,
		Payload:        string(verification.Body),
	})

	// 验签
	signature, err := base64.StdEncoding.DecodeString(parameters.requestSignature)
	if err != nil {
		return fmt.Errorf("%w: requestSignature 不是合法 base64: %v", ErrAuthorizationInvalid, err)
	}
	digest := sha512.Sum512([]byte(content))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA512, digest[:], signature); err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}
	return nil
}

// ParseMerchantNotify 解订单推送报文 | 报文是 Order 实体本身, 不带 {code,data} 信封
func ParseMerchantNotify(body []byte) (*MerchantNotify, error) {
	var notify MerchantNotify
	if err := json.Unmarshal(body, &notify); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotifyBodyInvalid, err)
	}
	return &notify, nil
}

// notifySignature 从 Authorization 头取出的签名参数
type notifySignature struct {
	// usingKey 平台侧签名所用用户密钥编号
	usingKey string
	// actAs 签名串里的扮演角色 | 出站推送当前固定为 `-`
	actAs string
	// generationTime 签名时间戳（毫秒）
	generationTime int64
	// requestSignature base64 后的签名
	requestSignature string
}

// parseAuthorization 解析签名头 | 参数按名取值, 与顺序无关
func parseAuthorization(authorization string) (notifySignature, error) {
	const prefix = "Signature "

	raw := strings.TrimSpace(authorization)
	if !strings.HasPrefix(raw, prefix) {
		return notifySignature{}, fmt.Errorf("%w: 缺少 %q 前缀", ErrAuthorizationInvalid, prefix)
	}
	values, err := url.ParseQuery(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return notifySignature{}, fmt.Errorf("%w: %v", ErrAuthorizationInvalid, err)
	}

	generationTime, err := strconv.ParseInt(strings.TrimSpace(values.Get("generationTime")), 10, 64)
	if err != nil {
		return notifySignature{}, fmt.Errorf("%w: generationTime 非法: %v", ErrAuthorizationInvalid, err)
	}
	// 平台出站推送用主体用户的 platformPrivateKey 签, actAs 暂时固定填 `-`;
	// 商户验签按报文实际带的值重建规范串, 不假设它一定为空或为 `-`
	notify := notifySignature{
		usingKey:         strings.TrimSpace(values.Get("usingKey")),
		actAs:            values.Get("actAs"),
		generationTime:   generationTime,
		requestSignature: strings.TrimSpace(values.Get("requestSignature")),
	}
	if notify.usingKey == "" || notify.requestSignature == "" {
		return notifySignature{}, fmt.Errorf("%w: 缺少 usingKey 或 requestSignature", ErrAuthorizationInvalid)
	}
	return notify, nil
}

// parsePublicKey 解析 platformPublicKey | 支持 SPKI（`PUBLIC KEY`）与 PKCS#1（`RSA PUBLIC KEY`）PEM
func parsePublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("%w: 不是合法的 PEM", ErrPublicKeyInvalid)
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return asRSAPublicKey(key)
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: 公钥解析失败（仅支持 RSA 的 SPKI / PKCS#1 PEM）: %v", ErrPublicKeyInvalid, err)
	}
	return key, nil
}

// asRSAPublicKey 判定解析出的公钥确实是 RSA
func asRSAPublicKey(key interface{}) (*rsa.PublicKey, error) {
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: 公钥类型应为 RSA, 实际 %T", ErrPublicKeyInvalid, key)
	}
	return rsaKey, nil
}
