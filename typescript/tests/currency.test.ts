import { describe, expect, test } from '@rstest/core';
import * as DineroCurrencies from 'dinero.js/currencies';
import { CURRENCIES, getCurrencyPrecision } from '../src';

describe('Currency', () => {
  test('币种全集与后端同源 | dinero.js 的 ISO 4217 全量表加链上 USDT', () => {
    expect(CURRENCIES).toEqual(
      [...Object.keys(DineroCurrencies), 'USDT'].sort(),
    );
  });

  test('精度取币种自身的小数位 | value 按它换算', () => {
    expect(getCurrencyPrecision('INR')).toBe(2);
    expect(getCurrencyPrecision('VND')).toBe(0);
    expect(getCurrencyPrecision('KWD')).toBe(3);
    expect(getCurrencyPrecision('USDT')).toBe(6);
  });
});
