import * as currencies from 'dinero.js/currencies';

/**
 * 币种定义表 | 与后端 `_library` 的 `CURRENCY_DEFINITION` 同一份来源：dinero.js 的 ISO 4217 全量表，
 * 外加不在表内的链上出款币种 USDT（10^6）。后端增减币种随 dinero.js 版本走，这里零改动跟上。
 */
export const CURRENCY_DEFINITION = {
  ...currencies,
  USDT: { code: 'USDT', base: 10, exponent: 6 },
};

/**
 * 币种 | 从 `CURRENCY_DEFINITION` 全量派生（167 个），与后端 `Currency` 枚举同源。
 * `DineroCurrency` 是命名空间里的类型再导出，运行期不存在，派生时按类型剔除。
 */
export type Currency = Exclude<
  keyof typeof CURRENCY_DEFINITION,
  'DineroCurrency'
>;

/** 币种全集 | 运行期取值并按三字码排序，商户可直接用来渲染币种下拉 */
export const CURRENCIES = Object.keys(CURRENCY_DEFINITION)
  .filter((code) => code !== 'DineroCurrency')
  .sort() as Currency[];

/**
 * 币种小数位 | 即最小币种单位与主单位的比例（INR/USD 为 2 即分, VND/JPY 为 0 即无小数位, USDT 为 6）
 *
 * `Amount.value` 传的就是这个精度下的整数值, 下单前用它换算避免把 100 盾当 1.00 盾发出去。
 */
export const getCurrencyPrecision = (currency: Currency): number =>
  CURRENCY_DEFINITION[currency].exponent;
