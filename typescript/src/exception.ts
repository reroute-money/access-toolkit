export class RerouteMoneyException extends Error {
  /**
   * 错误名
   */
  public override name = 'RerouteMoneyException';
  /**
   * 构造函数
   * @param code 错误码
   * @param message 错误信息
   */
  public constructor(
    private readonly code: string,
    message: string,
  ) {
    super(message);
  }
}
