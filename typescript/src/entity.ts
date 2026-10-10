/** 币种 | 与后端 Currency 枚举对齐 */
export type Currency = 'INR' | 'USD';

/** 金额 | value 为最小币种单位（分）, 下单时缺失一侧由服务端按命中汇率换算补齐（找不到汇率将报错） */
export type Amount = {
  currency: Currency;
  value?: number;
};

/** 收款信息 | 与后端 ReceiptInformation 对齐：UPI 只给 VPA，银行给账号/户名/IFSC/开户行，链上给链名/地址 */
export type ReceiptInformation =
  | { type: 'UPI'; identifier: string }
  | {
      type: 'BANK';
      accountNumber: string;
      payeeName: string;
      indianFinancialSystemCode: string;
      bankName: string;
    }
  | { type: 'BLOCKCHAIN'; chain: 'TRON' | 'ETHEREUM'; address: string };

/** 代付下单入参 | 付款/收款两侧币种必传, 金额至少一侧完整；收款信息必传（钱打给谁不能含糊, 且参与幂等比较） */
export type PaymentOrder = {
  order: {
    uniqueIdentifier: string;
    paymentAmount: Amount;
    receiptAmount: Amount;
    externalIdentifier: string;
    product: { type: string };
    callbackURL?: string;
  };
  receiptInformation: ReceiptInformation;
};

/** 代收下单入参 | 付款/收款两侧币种必传, 金额至少一侧完整；代收无收款信息（钱收进来, 打给谁由付款方决定） */
export type CollectionOrder = {
  order: {
    uniqueIdentifier: string;
    paymentAmount: Amount;
    receiptAmount: Amount;
    externalIdentifier: string;
    product: { type: string };
    callbackURL?: string;
  };
  /**
   * 商户回跳地址 | 付款人完成支付后应回去的页面, 可空;
   * 服务端原样下发渠道, 既不校验它属于哪个域名, 也不拼到 `redirectURL` 上
   */
  returnURL?: string;
};

export type RerouteMoneyResponse<T> = {
  code: string;
  message?: string;
  data: T;
};

/** 单据种类 | 由服务端按产品形态推导 */
export type OrderType = 'PAYMENT' | 'COLLECTION';

/** 订单状态 */
export type OrderStatus =
  | 'CREATED'
  | 'PROCESSING'
  | 'SUCCEEDED'
  | 'FAILED';

/** 金额 | 服务端回填后两侧都完整, value 为最小币种单位（分） */
export type Money = {
  currency: Currency;
  value: number;
};

/** 单据信息 | 只声明商户侧要消费的字段, 其余响应字段（渠道、费率、归属关系等）按未声明处理 */
export type OrderResult = {
  uniqueIdentifier: string;
  externalIdentifier: string;
  /**
   * 渠道单号 | 下游渠道给出的订单编号, 服务端拿到渠道结论时落库, 落定后不再变化;
   * 下单被渠道明确拒绝或渠道没给号时为空
   */
  internalIdentifier?: string;
  type: OrderType;
  status: OrderStatus;
  paymentAmount: Money;
  receiptAmount: Money;
  feeAmount?: Money;
  product?: { type: string };
  callbackURL?: string;
  createdAt: string;
  updatedAt: string;
};

/** 代付单查询结果 */
export type PaymentOrderResult = {
  uniqueIdentifier: string;
  order: OrderResult;
  /** 收款方式 | 服务端从收款信息推导 */
  type?: ReceiptInformation['type'];
  receiptInformation?: ReceiptInformation;
};

/** 代收单查询结果 */
export type CollectionOrderResult = {
  uniqueIdentifier: string;
  order: OrderResult;
  /** 付款跳转地址 | 渠道给出的收银台地址, 商户据此把付款人导过去; 渠道没给则一直为空 */
  redirectURL?: string;
  /** 商户回跳地址 | 下单时登记的原样回显; 付款人付完后由渠道导回这个页面 */
  returnURL?: string;
};

/** 账户 | 请求方成员名下的余额户与保证金户, 余额为最小币种单位（分） */
export type Account = {
  uniqueIdentifier: string;
  /** 账户归属 | 接入方只会拿到自己名下的户 */
  belongsTo: 'MERCHANT' | 'AGENT';
  /** 账户类型 | BALANCE = 可提现余额户, MARGIN = 保证金户 */
  type: 'BALANCE' | 'MARGIN';
  currency: Currency;
  availableBalance: number;
  frozenBalance: number;
};
