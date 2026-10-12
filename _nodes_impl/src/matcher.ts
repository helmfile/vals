/*
 * SPDX-License-Identifier: Apache-2.0
 */

import SecretsManagerV2 from '@ibm-cloud/secrets-manager/secrets-manager/v2.js';
import { logger } from './logger.js';

const regex = /^(([^:/?#]+):\/\/)?([^?#]*)(\?([^#]*))?(#(.*))?/gm;

export class SecretRef {
    secretParams: SecretsManagerV2.GetSecretByNameTypeParams;
    fragmentPath: string;

    constructor(secretType: string, secretGroupName: string, name: string, fragmentPath: string) {
        this.secretParams = {
            secretType,
            secretGroupName,
            name,
        };

        this.fragmentPath = fragmentPath;
    }

    public secretId(): string {
        return `${this.secretParams.secretType}:${this.secretParams.secretGroupName}:${this.secretParams.name}`;
    }

    public toString(): string {
        return this.secretId() + '#' + this.fragmentPath;
    }
}

export const matchRef = (ref: string): SecretRef | undefined => {
    let m;

    let secretRef: SecretRef | undefined;
    while ((m = regex.exec(ref)) !== null) {
        // This is necessary to avoid infinite loops with zero-width matches
        if (m.index === regex.lastIndex) {
            regex.lastIndex++;
        }
        logger.info(m);
        const paramParts = m[3].split('/');

        secretRef = new SecretRef(paramParts[3], paramParts[1], paramParts[5], m[7] ? m[7] : '');
    }

    return secretRef;
};
