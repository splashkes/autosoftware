import { defineConfig } from '@playwright/test';
import path from 'node:path';

function defaultRunID(): string {
  return new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z');
}

const runID = process.env.LIVE_REDTEAM_RUN_ID || defaultRunID();
const attackerPort = process.env.LIVE_REDTEAM_ATTACKER_PORT || '40123';
const attackerOrigin = process.env.LIVE_REDTEAM_ATTACKER_ORIGIN || `http://127.0.0.1:${attackerPort}`;
const runRoot = process.env.LIVE_REDTEAM_OUTPUT_DIR
  ? path.resolve(process.env.LIVE_REDTEAM_OUTPUT_DIR)
  : path.resolve(__dirname, '../plans/live-anonymous-red-team/reports/runs', runID);

export default defineConfig({
  testDir: '.',
  testMatch: 'live-redteam.spec.ts',
  timeout: 45000,
  fullyParallel: false,
  workers: 1,
  outputDir: path.join(runRoot, 'browser'),
  reporter: [['list']],
  use: {
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure'
  },
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER
    ? undefined
    : {
        command: `python3 -m http.server ${attackerPort} --bind 127.0.0.1`,
        cwd: path.resolve(__dirname, 'attacker-origin'),
        reuseExistingServer: false,
        timeout: 15000,
        url: attackerOrigin
      }
});
