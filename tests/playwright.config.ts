import { defineConfig } from '@playwright/test';
import path from 'node:path';

const serviceAppDir = path.resolve(
  __dirname,
  '../seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app',
);

const flowershowAppDir = path.resolve(
  __dirname,
  '../seeds/0007-Flowershow/realizations/a-firstbloom/artifacts/flowershow-app',
);

const csBaseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:38090';
const fsBaseURL = process.env.FLOWERSHOW_BASE_URL || 'http://127.0.0.1:38097';

export default defineConfig({
  testDir: '.',
  timeout: 30000,
  fullyParallel: false,
  workers: 1,
  use: {
    headless: true,
  },
  projects: [
    {
      name: 'customer-service',
      testMatch: 'customer-service.spec.ts',
      use: {
        baseURL: csBaseURL,
      },
    },
    {
      name: 'flowershow',
      testMatch: 'flowershow.spec.ts',
      use: {
        baseURL: fsBaseURL,
      },
    },
  ],
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER
    ? undefined
    : [
        {
          command: 'go run .',
          cwd: serviceAppDir,
          reuseExistingServer: false,
          timeout: 120000,
          url: csBaseURL,
          env: {
            ...process.env,
            AS_ADDR: new URL(csBaseURL).host,
            CHAT_TIMEOUT: process.env.CHAT_TIMEOUT || '3s',
          },
        },
        {
          command: 'go run .',
          cwd: flowershowAppDir,
          reuseExistingServer: false,
          timeout: 120000,
          url: fsBaseURL,
          env: {
            ...process.env,
            AS_ADDR: new URL(fsBaseURL).host,
            AS_ADMIN_PASSWORD: 'admin',
          },
        },
      ],
});
