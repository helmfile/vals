/*
 * SPDX-License-Identifier: Apache-2.0
 */

import pino from 'pino';
import env from 'env-var';

const logfile = env.get('APP_DEBUG').default('false').asBool();
const stderrLevel = env.get('APP_STDERR_LEVEL').default('error').asString();

let transports;

const logfilePath = env.get('APP_LOG_FILE').default('app.log').asString();

if (logfile) {
    transports = pino.transport({
        targets: [
            {
                target: 'pino/file',
                level: 'trace',
                options: { destination: logfilePath, append: false },
            },
            {
                target: 'pino-pretty',
                level: 'error',
                options: { destination: 2, level: stderrLevel }, // use 2 for stderr
            },
        ],
    });
} else {
    transports = pino.transport({
        targets: [
            {
                target: 'pino-pretty',
                level: 'error',
                options: { destination: 2, level: stderrLevel }, // use 2 for stderr
            },
        ],
    });
}

export const logger = pino.pino(
    {
        serializers: {
            err: pino.stdSerializers.err,
        },
    },
    transports,
);
