import { test, expect } from '@playwright/test';
import {
  FLOWERSHOW_REMOTE_E2E,
  openAgentAccess,
} from './flowershow.helpers';

test.describe('Flowershow Remote Admin', () => {
  test.skip(
    !FLOWERSHOW_REMOTE_E2E,
    'Set FLOWERSHOW_REMOTE_E2E=1 to run remote admin coverage.',
  );

  test('authenticated admin dashboard and widget load remotely', async ({ page }) => {
    await page.goto('/admin');
    await expect(page.locator('h1')).toContainText('Admin Dashboard');
    await openAgentAccess(page);
    await expect(
      page.getByRole('link', {
        name: 'GET /v1/contracts/0007-Flowershow/a-firstbloom',
      }),
    ).toBeVisible();
  });

  test('seeded admin show workspace is reachable remotely', async ({ page }) => {
    await page.goto('/admin/shows/show_spring2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');
    await expect(page.locator('#admin-info-panel')).toContainText('Judges');
    await expect(page.locator('.agent-access-shell')).toBeVisible();
  });
});
