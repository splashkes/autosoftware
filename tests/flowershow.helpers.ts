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

export async function loginLocalAdmin(page: Page) {
  await page.goto('/admin/login');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');
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
