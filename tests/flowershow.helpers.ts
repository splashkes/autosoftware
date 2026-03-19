import { expect, Page } from '@playwright/test';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

export const FLOWERSHOW_SERVICE_TOKEN =
  process.env.FLOWERSHOW_SERVICE_TOKEN || 'test-token';
export const FLOWERSHOW_REMOTE_E2E =
  process.env.FLOWERSHOW_REMOTE_E2E === '1';
export const FLOWERSHOW_ADMIN_EMAIL =
  process.env.FLOWERSHOW_ADMIN_EMAIL || 'simon@plashkes.com';
export const FLOWERSHOW_AUTH_STATE_PATH = path.resolve(
  __dirname,
  '.auth/flowershow-admin.json',
);
const FLOWERSHOW_BASE_URL =
  process.env.FLOWERSHOW_BASE_URL || 'http://127.0.0.1:38097';

const FLOWERSHOW_SESSION_SECRET =
  process.env.AS_SESSION_SECRET || 'playwright-flowershow-session-secret';
const FLOWERSHOW_LOCAL_ADMIN_SUB = 'sub_playwright_admin';

function encodeSignedCookie(value: unknown) {
  const payload = Buffer.from(JSON.stringify(value));
  const signature = crypto
    .createHmac('sha256', FLOWERSHOW_SESSION_SECRET)
    .update(payload)
    .digest();
  return `${payload.toString('base64url')}.${signature.toString('base64url')}`;
}

async function ensureLocalAdminRole(page: Page) {
  const response = await page.request.post(
    '/v1/commands/0007-Flowershow/roles.assign',
    {
      headers: {
        Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        'Content-Type': 'application/json',
      },
      data: {
        cognito_sub: FLOWERSHOW_LOCAL_ADMIN_SUB,
        role: 'admin',
      },
    },
  );
  expect(response.ok()).toBeTruthy();
}

export async function loginLocalAdmin(page: Page) {
  await ensureLocalAdminRole(page);
  const sessionCookie = encodeSignedCookie({
    user: {
      cognito_sub: FLOWERSHOW_LOCAL_ADMIN_SUB,
      email: 'playwright-admin@example.com',
      name: 'Playwright Admin',
    },
    expires_at: Math.floor(Date.now() / 1000) + 33 * 24 * 60 * 60,
  });
  await page.context().addCookies([
    {
      name: 'as_flowershow_session',
      value: sessionCookie,
      url: FLOWERSHOW_BASE_URL,
      httpOnly: true,
      sameSite: 'Lax',
      expires: Math.floor(Date.now() / 1000) + 33 * 24 * 60 * 60,
    },
  ]);
  await page.goto('/admin');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('h1')).toContainText('Admin Dashboard');
}

export async function openAgentAccess(page: Page) {
  const widget = page.locator('details.agent-access-widget');
  await expect(widget).toBeVisible();
  const summary = widget.locator('summary.agent-access-summary');
  await summary.click();
  await expect(widget.locator('.agent-access-content')).toBeVisible();
}

export async function expectAgentPath(page: Page, currentPath: string) {
  await openAgentAccess(page);
  await expect(page.locator('[data-agent-current-path]')).toHaveText(currentPath);
}

export function uniqueName(prefix: string) {
  return `${prefix} ${Date.now()}`;
}

export function ensureAuthStateDir() {
  fs.mkdirSync(path.dirname(FLOWERSHOW_AUTH_STATE_PATH), { recursive: true });
}
