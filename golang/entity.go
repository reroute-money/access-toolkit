package access

import "time"

// Currency 币种 | 与后端 Currency 枚举对齐
type Currency string

// 币种取值
const (
	CurrencyINR  Currency = "INR"
	CurrencyUSD  Currency = "USD"
	CurrencyUSDT Currency = "USDT"
)

// Amount 金额 | Value 为最小币种单位（INR/USD 为分, USDT 为 10^-6）
//
// 下单时缺失一侧（Value 为 nil）由服务端按命中汇率换算补齐, 找不到汇率会报错。
type Amount struct {
	// Currency 币种 | 必传
	Currency Currency `json:"currency"`
	// Value 金额 | 缺省时由服务端补齐
	Value *int64 `json:"value,omitempty"`
}

// ReceiptType 收款方式
type ReceiptType string

// 收款方式取值
const (
	ReceiptTypeUPI        ReceiptType = "UPI"
	ReceiptTypeBANK       ReceiptType = "BANK"
	ReceiptTypeBLOCKCHAIN ReceiptType = "BLOCKCHAIN"
)

// Chain 链名 | 仅 BLOCKCHAIN 收款方式使用
type Chain string

// 链名取值
const (
	ChainTron     Chain = "TRON"
	ChainEthereum Chain = "ETHEREUM"
)

// ReceiptInformation 收款信息 | 后端是 UPI / BANK / BLOCKCHAIN 三选一的联合类型,
// Go 侧摊平成一张 omitempty 全字段表, 序列化字节与联合类型一致。
//
// 优先用 NewUPIReceipt / NewBankReceipt / NewBlockchainReceipt 构造, 避免填出不合法组合。
type ReceiptInformation struct {
	// Type 收款方式 | 必传
	Type ReceiptType `json:"type"`
	// Identifier UPI VPA | type=UPI 时必填
	Identifier string `json:"identifier,omitempty"`
	// AccountNumber 银行账号 | type=BANK 时必填
	AccountNumber string `json:"accountNumber,omitempty"`
	// PayeeName 户名 | type=BANK 时必填
	PayeeName string `json:"payeeName,omitempty"`
	// IndianFinancialSystemCode IFSC | type=BANK 时必填
	IndianFinancialSystemCode string `json:"indianFinancialSystemCode,omitempty"`
	// BankName 开户行 | type=BANK 时必填
	BankName string `json:"bankName,omitempty"`
	// Chain 链名 | type=BLOCKCHAIN 时必填
	Chain Chain `json:"chain,omitempty"`
	// Address 链上地址 | type=BLOCKCHAIN 时必填
	Address string `json:"address,omitempty"`
}

// NewUPIReceipt UPI 收款 | 只给 VPA
func NewUPIReceipt(identifier string) ReceiptInformation {
	return ReceiptInformation{Type: ReceiptTypeUPI, Identifier: identifier}
}

// NewBankReceipt 银行收款 | 账号 / 户名 / IFSC / 开户行
func NewBankReceipt(accountNumber, payeeName, indianFinancialSystemCode, bankName string) ReceiptInformation {
	return ReceiptInformation{
		Type:                      ReceiptTypeBANK,
		AccountNumber:             accountNumber,
		PayeeName:                 payeeName,
		IndianFinancialSystemCode: indianFinancialSystemCode,
		BankName:                  bankName,
	}
}

// NewBlockchainReceipt 链上收款 | 链名 + 地址
func NewBlockchainReceipt(chain Chain, address string) ReceiptInformation {
	return ReceiptInformation{Type: ReceiptTypeBLOCKCHAIN, Chain: chain, Address: address}
}

// Product 产品 | 下单只给类型, 具体产品由服务端按 (单据种类, 付款币种, 收款币种) 定位
type Product struct {
	// Type 产品类型 | 如 ONLINE_PAY / ONLINE_RECEIVE
	Type string `json:"type"`
}

// OrderRequest 订单公共入参 | 付款/收款两侧币种必传, 金额至少一侧完整
type OrderRequest struct {
	// UniqueIdentifier 平台单号 | 留空由服务端生成
	UniqueIdentifier string `json:"uniqueIdentifier,omitempty"`
	// PaymentAmount 付款金额 | 付款人实际支付的金额
	PaymentAmount Amount `json:"paymentAmount"`
	// ReceiptAmount 收款金额 | 收款侧最终到账金额
	ReceiptAmount Amount `json:"receiptAmount"`
	// ExternalIdentifier 外部单号 | 成员级唯一, 参与幂等比较
	ExternalIdentifier string `json:"externalIdentifier"`
	// Product 产品
	Product Product `json:"product"`
	// CallbackURL 回调地址 | 可空
	CallbackURL string `json:"callbackURL,omitempty"`
}

// PaymentOrder 代付下单入参 | 收款信息必传（钱打给谁不能含糊, 且参与幂等比较）
type PaymentOrder struct {
	// Order 订单公共入参
	Order OrderRequest `json:"order"`
	// ReceiptInformation 收款信息
	ReceiptInformation ReceiptInformation `json:"receiptInformation"`
}

// CollectionOrder 代收下单入参 | 代收无收款信息（钱收进来, 打给谁由付款方决定）
type CollectionOrder struct {
	// Order 订单公共入参
	Order OrderRequest `json:"order"`
	// ReturnURL 商户回跳地址 | 付款人完成支付后应回去的页面, 可空;
	// 平台原样下发渠道, 既不校验它属于哪个域名, 也不拼到 redirectURL 上
	ReturnURL string `json:"returnURL,omitempty"`
}

// Money 金额 | 服务端回填后的金额, 两侧都完整
type Money struct {
	// Currency 币种
	Currency Currency `json:"currency"`
	// Value 最小币种单位（分）
	Value int64 `json:"value"`
}

// OrderStatus 订单状态
type OrderStatus string

// 订单状态取值
const (
	OrderStatusCreated    OrderStatus = "CREATED"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusSucceeded  OrderStatus = "SUCCEEDED"
	OrderStatusFailed     OrderStatus = "FAILED"
)

// OrderType 单据种类
type OrderType string

// 单据种类取值
const (
	OrderTypePayment    OrderType = "PAYMENT"
	OrderTypeCollection OrderType = "COLLECTION"
)

// OrderResult 服务端回填后的单据 | 只声明商户侧要消费的字段,
// 其余响应字段（渠道、费率、商户/代理关系等）解析时直接忽略
type OrderResult struct {
	// UniqueIdentifier 平台单号
	UniqueIdentifier string `json:"uniqueIdentifier"`
	// ExternalIdentifier 外部单号
	ExternalIdentifier string `json:"externalIdentifier"`
	// InternalIdentifier 渠道单号 | 下游渠道给出的订单编号, 由平台在拿到渠道结论时落库;
	// 下单被渠道明确拒绝或渠道没给号时为空, 落定后不再变化
	InternalIdentifier string `json:"internalIdentifier,omitempty"`
	// Type 单据种类 | 由服务端按产品形态推导
	Type OrderType `json:"type"`
	// Status 订单状态
	Status OrderStatus `json:"status"`
	// PaymentAmount 付款金额
	PaymentAmount *Money `json:"paymentAmount,omitempty"`
	// ReceiptAmount 收款金额
	ReceiptAmount *Money `json:"receiptAmount,omitempty"`
	// FeeAmount 手续费 | 按付款侧计
	FeeAmount *Money `json:"feeAmount,omitempty"`
	// Product 命中的产品
	Product *Product `json:"product,omitempty"`
	// CallbackURL 本次交易实际使用的回调地址
	CallbackURL string `json:"callbackURL,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// PaymentOrderResult 代付下单结果
type PaymentOrderResult struct {
	// UniqueIdentifier 付款单号
	UniqueIdentifier string `json:"uniqueIdentifier"`
	// Order 订单信息
	Order OrderResult `json:"order"`
	// Type 收款方式 | 服务端从收款信息推导, 便于按方式筛选
	Type ReceiptType `json:"type,omitempty"`
	// ReceiptInformation 收款信息回显
	ReceiptInformation *ReceiptInformation `json:"receiptInformation,omitempty"`
}

// CollectionOrderResult 代收下单结果
type CollectionOrderResult struct {
	// UniqueIdentifier 收款单号
	UniqueIdentifier string `json:"uniqueIdentifier"`
	// Order 订单信息
	Order OrderResult `json:"order"`
	// RedirectURL 付款跳转地址 | 渠道给出的收银台地址, 商户据此把付款人导过去; 渠道没给则一直为空
	RedirectURL string `json:"redirectURL,omitempty"`
	// ReturnURL 商户回跳地址 | 下单时登记的原样回显; 付款人付完后由渠道导回这个页面
	ReturnURL string `json:"returnURL,omitempty"`
}

// AccountBelongsTo 账户归属
type AccountBelongsTo string

// 账户归属取值
const (
	AccountBelongsToMerchant AccountBelongsTo = "MERCHANT"
	AccountBelongsToAgent    AccountBelongsTo = "AGENT"
)

// AccountType 账户类型
type AccountType string

// 账户类型取值
const (
	AccountTypeBalance AccountType = "BALANCE"
	AccountTypeMargin  AccountType = "MARGIN"
)

// Account 账户 | 查询按签名里的 actAs 定位成员, 只会回请求方名下的户; 余额为最小币种单位（分）
type Account struct {
	// UniqueIdentifier 账户编号
	UniqueIdentifier string `json:"uniqueIdentifier"`
	// BelongsTo 账户归属
	BelongsTo AccountBelongsTo `json:"belongsTo"`
	// Type 账户类型 | BALANCE = 可提现余额户, MARGIN = 保证金户
	Type AccountType `json:"type"`
	// Currency 币种
	Currency Currency `json:"currency"`
	// AvailableBalance 可用余额
	AvailableBalance int64 `json:"availableBalance"`
	// FrozenBalance 冻结余额
	FrozenBalance int64 `json:"frozenBalance"`
}

// MerchantNotify 平台订单推送报文 | 我方→商户方向, 与 OrderCallbackService.pushMerchantNotify 序列化的
// Order 实体对齐：代收/代付共用一套报文, 由 Type 区分单据种类。
//
// 只声明商户要消费的字段, 其余（tenantMember、汇率、账户等）解析时忽略; 平台侧已剔除 fees / feeGroups,
// 并把 channel 裁成只剩 name。报文不带 {code,data} 信封。
type MerchantNotify struct {
	// UniqueIdentifier 平台单号
	UniqueIdentifier string `json:"uniqueIdentifier"`
	// ExternalIdentifier 商户侧外部单号
	ExternalIdentifier string `json:"externalIdentifier"`
	// Type 单据种类
	Type OrderType `json:"type"`
	// Status 订单状态 | 平台只在 SUCCEEDED / FAILED 终态推送
	Status OrderStatus `json:"status"`
	// BelongsTo 单据归属
	BelongsTo string `json:"belongsTo,omitempty"`
	// Amount 订单金额
	Amount *Money `json:"amount,omitempty"`
	// PaymentAmount 付款金额
	PaymentAmount *Money `json:"paymentAmount,omitempty"`
	// ReceiptAmount 收款金额
	ReceiptAmount *Money `json:"receiptAmount,omitempty"`
	// Product 命中的产品
	Product *Product `json:"product,omitempty"`
	// CallbackURL 本次交易实际使用的回调地址
	CallbackURL string `json:"callbackURL,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// Response 响应信封
type Response[T any] struct {
	// Code 业务码 | 成功为 CodeOK
	Code string `json:"code"`
	// Message 错误信息 | 成功时可空
	Message string `json:"message,omitempty"`
	// Data 业务数据
	Data T `json:"data"`
}

// Err 信封业务码检查 | 对应 TS 的 throwIfNotOk: HTTP 再好看的是 code, 非 OK 一律当失败
func (response *Response[T]) Err() error {
	if response.Code != CodeOK {
		return &Error{Code: response.Code, Message: response.Message}
	}
	return nil
}
