import { expect, Page } from '@playwright/test';
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

const FLOWERSHOW_LOCAL_ADMIN_SUB = 'sub_playwright_admin';
const FLOWERSHOW_LOCAL_VIEWER_SUB = 'sub_playwright_viewer';

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
  const response = await page.request.post('/__test/session', {
    headers: {
      Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
      'Content-Type': 'application/json',
    },
    data: {
      user: {
        cognito_sub: FLOWERSHOW_LOCAL_ADMIN_SUB,
        email: 'playwright-admin@example.com',
        name: 'Playwright Admin',
      },
    },
  });
  expect(response.ok()).toBeTruthy();
  await page.goto('/admin');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('h1')).toContainText('Admin Dashboard');
}

export async function loginLocalViewer(page: Page) {
  const response = await page.request.post('/__test/session', {
    headers: {
      Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
      'Content-Type': 'application/json',
    },
    data: {
      user: {
        cognito_sub: FLOWERSHOW_LOCAL_VIEWER_SUB,
        email: 'playwright-viewer@example.com',
        name: 'Playwright Viewer',
      },
    },
  });
  expect(response.ok()).toBeTruthy();
}

export async function expectSignedInLanding(page: Page) {
  await expect(page).toHaveURL(/\/(account|admin)(?:$|\?)/);
  const heading = page.locator('h1');
  await expect(heading).toBeVisible();
  const text = (await heading.textContent()) || '';
  expect(['Your Profile', 'Admin Dashboard']).toContain(text.trim());
}

export async function openAgentAccess(page: Page) {
  const widget = page.locator('.agent-access-widget[data-agent-widget]');
  await expect(widget).toBeVisible();
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
