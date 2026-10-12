import {
  AxiosError,
  AxiosInstance,
  AxiosRequestConfig,
  AxiosResponse,
  create,
  mergeConfig,
} from 'axios';
import { RerouteMoneyUtility } from './utility.ts';
import {
  Account,
  CollectionOrder,
  CollectionOrderResult,
  PaymentOrder,
  PaymentOrderResult,
  RerouteMoneyResponse,
} from './entity.ts';
import { DeepPartial } from 'es-toolkit/types';

export class RerouteMoneyClient {
  private readonly httpClient: AxiosInstance;

  public constructor(option: {
    _httpClient?: AxiosRequestConfig;
    userKey: {
      uniqueIdentifier: string;
      servicePrivateKey: string;
      platformPublicKey: string;
    };
    tenantMember: {
      uniqueIdentifier: string;
    };
  }) {
    this.httpClient = create(
      mergeConfig(
        {
          baseURL: 'https://access-point.reroute.money',
          headers: {
            'Content-Type': 'application/json',
          },
          transformRequest: [
            function (data) {
              // -
              const stringifiedData = JSON.stringify(data);
              // -
              RerouteMoneyUtility.signRequest({
                // -
                request: this,
                requestPayload: stringifiedData,
                requestTime: Date.now(),
                // -
                privateKeyUniqueIdentifier: option.userKey.uniqueIdentifier,
                actAs: option.tenantMember.uniqueIdentifier,
                privateKey: option.userKey.servicePrivateKey,
              });
              // -
              return stringifiedData;
            },
          ],
        },
        option._httpClient ?? {},
      ),
    );
  }

  public async call<T>(
    config: AxiosRequestConfig,
  ): Promise<RerouteMoneyResponse<T>> {
    return this.httpClient
      .request(config)
      .then((response) => {
        // -
        RerouteMoneyUtility.throwIfNotOk(response);
        // -
        return response.data;
      })
      .catch((error: unknown) => {
        // -
        if (error instanceof AxiosError) {
          RerouteMoneyUtility.throwIfNotOk(error.response);
        }
        // -
        throw error;
      });
  }

  public makePaymentOrder(data: DeepPartial<PaymentOrder>) {
    return this.call<PaymentOrderResult>({
      url: '/v2/payment-order',
      method: 'PUT',
      data,
    });
  }

  public makeCollectionOrder(data: DeepPartial<CollectionOrder>) {
    return this.call<CollectionOrderResult>({
      url: '/v2/collection-order',
      method: 'PUT',
      data,
    });
  }

  /**
   * 查询代付单 | `identifier` 为付款单号（响应顶层的 `uniqueIdentifier`, 即 `payment_orders` 行自己的 uuid）
   * 或商户外部单号（`order.externalIdentifier`）; 订单层的 `order.uniqueIdentifier` 不参与匹配, 拿它查必然查空
   */
  public queryPaymentOrder(identifier: string) {
    return this.call<PaymentOrderResult>({
      // 路径参数直接拼进 url | 签名取的就是这份 url, 服务端验签用的 request.baseUrl 同样是含参数的完整路径
      url: `/v2/payment-order/${identifier}`,
      method: 'GET',
    });
  }

  /**
   * 查询代收单 | `identifier` 为收款单号（响应顶层的 `uniqueIdentifier`, 即 `collection_orders` 行自己的 uuid）
   * 或商户外部单号（`order.externalIdentifier`）; 订单层的 `order.uniqueIdentifier` 不参与匹配
   */
  public queryCollectionOrder(identifier: string) {
    return this.call<CollectionOrderResult>({
      url: `/v2/collection-order/${identifier}`,
      method: 'GET',
    });
  }

  /** 列出账户 | 回签名 actAs 成员名下的户, 无需入参 */
  public queryAccounts() {
    return this.call<Account[]>({
      url: '/v2/account',
      method: 'GET',
    });
  }

  public getHTTPClient() {
    return this.httpClient;
  }
}
