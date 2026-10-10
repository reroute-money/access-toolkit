import { AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import { createSign } from 'crypto';
import { RerouteMoneyException } from './exception';

export class RerouteMoneyUtility {
  public static signRequest(option: {
    // -
    request: InternalAxiosRequestConfig;
    requestPayload: string;
    requestTime: number;
    // -
    privateKeyUniqueIdentifier: string;
    privateKey: string;
    actAs: string;
  }): void {
    // 构建签名内容
    // > -
    let signatureContent = `${option.request.method!.toUpperCase()} ${
      option.request.url
    }\r\n\r\nGeneration-Time: ${option.requestTime}\r\nAct-As: ${option.actAs}`;
    // > -
    if (option.requestPayload) {
      signatureContent += `\r\n\r\n${option.requestPayload}`;
    }

    // -
    const signature = new URLSearchParams();

    // -
    // > -
    signature.append('usingKey', option.privateKeyUniqueIdentifier);
    // > -
    signature.append('actAs', option.actAs);
    // > -
    signature.append('generationTime', String(option.requestTime));
    // > -
    signature.append(
      'requestSignature',
      this.sign(signatureContent, option.privateKey).toString('base64'),
    );

    // -
    option.request.headers.setAuthorization(
      `Signature ${signature.toString()}`,
    );
  }

  public static sign(sourceData: string, privateKey: string) {
    // -
    const signer = createSign('sha512');

    // -
    // > -
    signer.update(sourceData);
    // > -
    signer.end();

    // -
    return signer.sign(privateKey);
  }

  public static throwIfNotOk(response?: AxiosResponse) {
    if (response?.data) {
      // -
      const { code, message } = response.data;
      // -
      if (code !== 'OK') {
        // -
        throw new RerouteMoneyException(code, message);
      }
    }
  }
}
