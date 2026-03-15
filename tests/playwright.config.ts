import { defineConfig } from '@playwright/test';
import path from 'node:path';

const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:38090';
const serviceAppDir = path.resolve(
  __dirname,
  '../seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app',
);

export default defineConfig({
  testDir: '.',
  timeout: 30000,
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL,
    headless: true,
  },
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER
    ? undefined
    : {
        command: 'go run .',
        cwd: serviceAppDir,
        reuseExistingServer: false,
        timeout: 120000,
        url: baseURL,
        env: {
          ...process.env,
          AS_ADDR: new URL(baseURL).host,
          CHAT_TIMEOUT: process.env.CHAT_TIMEOUT || '3s',
        },
      },
});
