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
const FLOWERSHOW_LOCAL_INTAKE_SUB = 'sub_playwright_intake';
const FLOWERSHOW_LOCAL_CLUB_ADMIN_SUB = 'sub_playwright_club_admin';

async function ensureLocalAdminRole(page: Page) {
  return ensureLocalRole(page, {
    cognito_sub: FLOWERSHOW_LOCAL_ADMIN_SUB,
    role: 'admin',
  });
}

async function ensureLocalRole(page: Page, data: Record<string, unknown>) {
  const response = await page.request.post(
    '/v1/commands/0007-Flowershow/roles.assign',
    {
      headers: {
        Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        'Content-Type': 'application/json',
      },
      data,
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
        subject_id: FLOWERSHOW_LOCAL_ADMIN_SUB,
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
        subject_id: FLOWERSHOW_LOCAL_VIEWER_SUB,
        cognito_sub: FLOWERSHOW_LOCAL_VIEWER_SUB,
        email: 'playwright-viewer@example.com',
        name: 'Playwright Viewer',
      },
    },
  });
  expect(response.ok()).toBeTruthy();
}

export async function loginLocalIntakeOperator(page: Page, showID = 'show_spring2025') {
  await ensureLocalRole(page, {
    cognito_sub: FLOWERSHOW_LOCAL_INTAKE_SUB,
    show_id: showID,
    role: 'show_intake_operator',
  });
  const response = await page.request.post('/__test/session', {
    headers: {
      Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
      'Content-Type': 'application/json',
    },
    data: {
      user: {
        subject_id: FLOWERSHOW_LOCAL_INTAKE_SUB,
        cognito_sub: FLOWERSHOW_LOCAL_INTAKE_SUB,
        email: 'playwright-intake@example.com',
        name: 'Playwright Intake',
      },
    },
  });
  expect(response.ok()).toBeTruthy();
}

export async function loginLocalClubAdmin(page: Page, organizationID = 'org_demo1') {
  await ensureLocalRole(page, {
    cognito_sub: FLOWERSHOW_LOCAL_CLUB_ADMIN_SUB,
    organization_id: organizationID,
    role: 'organization_admin',
  });
  const response = await page.request.post('/__test/session', {
    headers: {
      Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
      'Content-Type': 'application/json',
    },
    data: {
      user: {
        subject_id: FLOWERSHOW_LOCAL_CLUB_ADMIN_SUB,
        cognito_sub: FLOWERSHOW_LOCAL_CLUB_ADMIN_SUB,
        email: 'playwright-club-admin@example.com',
        name: 'Playwright Club Admin',
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
  await page.getByRole('tab', { name: 'Agent + access' }).click();
  await expect(page.locator('[data-agent-current-path]')).toHaveText(currentPath);
}

export function uniqueName(prefix: string) {
  return `${prefix} ${Date.now()}`;
}

export function ensureAuthStateDir() {
  fs.mkdirSync(path.dirname(FLOWERSHOW_AUTH_STATE_PATH), { recursive: true });
}
