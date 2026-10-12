#!/usr/bin/env node

/*
 * SPDX-License-Identifier: Apache-2.0
 */

import sourceMapSupport from 'source-map-support';
import fs from 'node:fs';
import { resolveRefs } from './main.js';
import env from 'env-var';
import { logger } from './logger.js';

import yargs from 'yargs';
import { hideBin } from 'yargs/helpers';

sourceMapSupport.install();

interface CliArgs {
    'cloud-api-key': string;
    'service-endpoint': string;
}

const y: CliArgs = yargs()
    .scriptName('ibmsecrets-vals')
    .usage('$0 [args]')
    .option('cloud-api-key', {
        describe: 'IBM Cloud API key (CLOUD_API_KEY)',
        default: '',
        type: 'string',
    })
    .option('service-endpoint', {
        describe: 'IBM Secrets Service Endpoint url (IBMSECRETS_SERVICE_ENDPOINT)',
        default: '',
        type: 'string',
    })
    .help()
    .parse(hideBin(process.argv)) as CliArgs;

try {
    const cloudApiKey = env.get('CLOUD_API_KEY').default(y['cloud-api-key']).required().asString();
    const serviceEndpoint = env
        .get('IBMSECRETS_SERVICE_ENDPOINT')
        .default(y['service-endpoint'])
        .required()
        .asUrlString();

    const template = fs.readFileSync(0, 'utf-8');

    const result = await resolveRefs(template, cloudApiKey, serviceEndpoint);
    console.log(result);

    process.exit(0);
} catch (e) {
    if (e instanceof env.EnvVarError) {
        logger.error('Environment configuration mistake:: ' + e);
    } else {
        logger.error({ err: e }, 'Error:: ');
    }
    process.exit(-1);
}
