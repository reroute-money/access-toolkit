import { beforeAll, describe, expect, test } from '@rstest/core';
import { RerouteMoneyClient, RerouteMoneyException } from '../src';
import { CollectionOrder } from '../src';

describe(RerouteMoneyClient.name, () => {
  let client: RerouteMoneyClient;

  beforeAll(() => {
    client = new RerouteMoneyClient({
      _httpClient: {
        baseURL: 'http://127.0.0.1:4001',
      },
      tenantMember: {
        uniqueIdentifier: '9462318f-b3dc-4559-8c4a-c0cda9fd5e7c',
      },
      userKey: {
        uniqueIdentifier: '32d61b1d-5ec6-42f3-aff4-65d5086ec630',
        servicePrivateKey: `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`,
        platformPublicKey: `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnYaYoZFwHfRkEBNYnDBO
aH0yOfxJtH2UWvcVLr8MGWd7nJrYs09TSEnX2FozmNSgKwWwfMNA8ZNuoJLmb+iO
g2XfKlB2AhOGHI8G2ARJyJwR8naaZeE7h8RDfz5XLydWRzggtHUYFka5XyNof2qg
yhhSeZUSSaC7XA6TtAtRXS2X3yRdGEuxSNxtBGnnUVH8r4RMSneX3sfukCGBk987
UfOc9DuCNV2kqo8pUc3PAJs08ENx+CvY/XP7CrxCbPEu5U0wKsOLRJH85AvUv7vg
JQCj6MD+yCD/+uvbwUPbbiVFsyEEp5Y51FWTwhsePFVQwAOuYAacsPheUypM/Jg3
uwIDAQAB
-----END PUBLIC KEY-----`,
      },
    });
  });

  test('makePaymentOrder', async () => {
    // -
    const result = await client.makePaymentOrder({
      order: {
        // 付款侧完整, 收款侧只传币种（由服务端按命中汇率换算补齐）
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier: crypto.randomUUID(),
        product: {
          type: 'ONLINE_PAY',
        },
      },
      receiptInformation: {
        type: 'UPI',
        identifier: 'payee@okhdfcbank',
      },
    });
    // -
    expect(result).toBeDefined();
  });

  test('makePaymentOrderIdempotently', async () => {
    // -
    const result = await client.makePaymentOrder({
      order: {
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier: '32730576-e8c8-415f-a648-9b1568f1e992',
        product: {
          type: 'ONLINE_PAY',
        },
      },
      receiptInformation: {
        type: 'BANK',
        accountNumber: '12345678900',
        payeeName: 'Ashok Kumar',
        bankCode: 'HDFC0001234',
        bankName: 'HDFC Bank',
      },
    });
    // -
    expect(result).toBeDefined();
  });

  test('makePaymentOrderNonIdempotently', async () => {
    await expect(() =>
      client.makePaymentOrder({
        order: {
          paymentAmount: {
            currency: 'INR',
          },
          receiptAmount: {
            currency: 'INR',
            value: Math.floor(Math.random() * 100),
          },
          externalIdentifier: 'd7c6f033-9955-48ae-afc3-c98bf798aa90',
          product: {
            type: 'ONLINE_PAY',
          },
        },
        receiptInformation: {
          type: 'UPI',
          identifier: 'payee@okhdfcbank',
        },
      }),
    ).rejects.toThrow(RerouteMoneyException);
  });

  test('makeCollectionOrder', async () => {
    // -
    const result = await client.makeCollectionOrder({
      order: {
        // 付款侧完整, 收款侧只传币种（由服务端按命中汇率换算补齐）
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier: crypto.randomUUID(),
        product: {
          type: 'ONLINE_RECEIVE',
        },
      },
    });
    // -
    expect(result).toBeDefined();
  });

  test('makeCollectionOrderIdempotently', async () => {
    // -
    const data = {
      order: {
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier: crypto.randomUUID(),
        product: {
          type: 'ONLINE_RECEIVE',
        },
      },
    } as CollectionOrder;
    // 同一外部单号重复下单 → 复用库内单据, 返回同一张单
    expect(
      (await client.makeCollectionOrder(data)).data.order.uniqueIdentifier,
    ).toBe(
      (await client.makeCollectionOrder(data)).data.order.uniqueIdentifier,
    );
  });

  test('makeCollectionOrderNonIdempotently', async () => {
    // 生成单号
    const externalIdentifier = crypto.randomUUID();
    // 先按付款侧完整落一张单
    await client.makeCollectionOrder({
      order: {
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier,
        product: {
          type: 'ONLINE_RECEIVE',
        },
      },
    });
    // -
    await expect(() =>
      client.makeCollectionOrder({
        order: {
          paymentAmount: {
            currency: 'INR',
            value: 200,
          },
          receiptAmount: {
            currency: 'INR',
          },
          externalIdentifier,
          product: {
            type: 'ONLINE_RECEIVE',
          },
        },
      }),
    ).rejects.toThrow(RerouteMoneyException);
  });

  test('queryPaymentOrder', async () => {
    // 先落一张单, 再按平台单号查回
    const made = await client.makePaymentOrder({
      order: {
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier: crypto.randomUUID(),
        product: {
          type: 'ONLINE_PAY',
        },
      },
      receiptInformation: {
        type: 'UPI',
        identifier: 'payee@okhdfcbank',
      },
    });
    // -
    const result = await client.queryPaymentOrder(
      made.data.order.uniqueIdentifier,
    );
    // -
    expect(result.data.order.uniqueIdentifier).toBe(
      made.data.order.uniqueIdentifier,
    );
  });

  test('queryCollectionOrderByExternalIdentifier', async () => {
    // 外部单号与内部 uuid 走同一个端点
    const externalIdentifier = crypto.randomUUID();
    // -
    const made = await client.makeCollectionOrder({
      order: {
        paymentAmount: {
          currency: 'INR',
          value: 100,
        },
        receiptAmount: {
          currency: 'INR',
        },
        externalIdentifier,
        product: {
          type: 'ONLINE_RECEIVE',
        },
      },
    });
    // -
    const result = await client.queryCollectionOrder(externalIdentifier);
    // -
    expect(result.data.order.uniqueIdentifier).toBe(
      made.data.order.uniqueIdentifier,
    );
  });

  test('queryAccounts', async () => {
    // -
    const result = await client.queryAccounts();
    // -
    expect(Array.isArray(result.data)).toBe(true);
  });
});
