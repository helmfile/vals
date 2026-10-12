/*
 * SPDX-License-Identifier: Apache-2.0
 */

import { IamAuthenticator } from '@ibm-cloud/secrets-manager/auth/index.js';
// SecretsManagerV2 is a CJS `export =` module; import as `pkg` so we can destructure
// the namespace constants, and use the default export as the class/namespace type.
import SecretsManagerV2 from '@ibm-cloud/secrets-manager/secrets-manager/v2.js';
import { matchRef, SecretRef } from './matcher.js';
import { logger } from './logger.js';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const { GetSecretByNameTypeConstants } = SecretsManagerV2 as any;

// Matches bare ref+ibmsecrets://... refs (no wrapper required)
const REF_PATTERN = /(ref\+ibmsecrets:\/\/[^\s"'\\]+)/g;

function getValueByPath(obj: object, path: string): string {
    logger.info(obj);
    logger.info(path);
    const result = path
        .split('/')
        .filter(Boolean)
        .reduce((acc: object, key: string) => {
            if (acc === undefined || acc === null) return undefined;

            const index = Number(key);
            if (!isNaN(index) && Array.isArray(acc)) {
                return acc[index];
            }

            return (acc as never)[key];
        }, obj);

    if (!result) {
        throw new Error(`ERROR ${path} not applicable`);
    }
    return result.toString();
}

const CACHE: Map<string, SecretsManagerV2.Secret> = new Map();

const resolveRef = async (secretsManager: SecretsManagerV2, secretref: SecretRef): Promise<string> => {
    const secretType = secretref.secretParams.secretType;
    const fragment = secretref.fragmentPath;
    logger.info('Secret ref is ' + secretref.toString());
    let payload: string = '';

    let result = CACHE.get(secretref.secretId());
    if (!result) {
        logger.info('Cache miss');
        const response = await secretsManager.getSecretByNameType(secretref.secretParams);
        CACHE.set(secretref.secretId(), response.result);
        if (response.status != 200) {
            logger.error('Status response is ' + response.status + ' ' + response.statusText);
            throw new Error(response.statusText);
        }
        result = response.result;
    }

    logger.info(result);
    if (secretType === GetSecretByNameTypeConstants.SecretType.ARBITRARY) {
        payload = (result as { payload: string })['payload'];
    } else if (secretType === GetSecretByNameTypeConstants.SecretType.SERVICE_CREDENTIALS) {
        const obj = (result as { source_service: object; credentials: object })['credentials'];
        payload = getValueByPath(obj, fragment);
    } else if (secretType == GetSecretByNameTypeConstants.SecretType.KV) {
        const obj = (result as { data: object })['data'];
        payload = getValueByPath(obj, fragment);
    }

    return payload;
};

export async function resolveRefs(input: string, cloudApiKey: string, serviceEndpoint: string): Promise<string> {
    const authenticator = new IamAuthenticator({ apikey: cloudApiKey });
    const secretsManager = new SecretsManagerV2({ authenticator, serviceUrl: serviceEndpoint });

    // Collect all unique full-match tokens → resolve in parallel
    const matches = [...input.matchAll(REF_PATTERN)];
    const resolved = new Map<string, string>();

    await Promise.all(
        matches
            .filter((m) => !resolved.has(m[1]))
            .map(async (m) => {
                const secretRef = matchRef(m[1]);
                if (secretRef) {
                    resolved.set(m[1], await resolveRef(secretsManager, secretRef));
                }
            }),
    );

    // Reset lastIndex before replace (regex is stateful with /g)
    REF_PATTERN.lastIndex = 0;
    return input.replace(REF_PATTERN, (_match, ref) => resolved.get(ref) ?? ref);
}
