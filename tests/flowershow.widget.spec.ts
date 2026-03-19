import { test, expect } from '@playwright/test';
import { expectAgentPath, loginLocalAdmin, openAgentAccess } from './flowershow.helpers';

test.describe('Flowershow Agent Widget', () => {
  test('widget exposes contract links on public pages', async ({ page }) => {
    await page.goto('/');
    await openAgentAccess(page);
    await expect(page.getByText('This realization exposes a live contract')).toBeVisible();
    await expect(
      page.getByRole('link', { name: 'GET /v1/contracts', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('link', {
        name: 'GET /v1/contracts/0007-Flowershow/a-firstbloom',
      }),
    ).toBeVisible();
  });

  test('widget shows current page path and contract detail', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025');
    await expectAgentPath(page, '/shows/spring-rose-show-2025');

    await page
      .getByRole('link', {
        name: 'GET /v1/contracts/0007-Flowershow/a-firstbloom',
      })
      .click();
    await expect(page.locator('body')).toContainText('"seed_id":"0007-Flowershow"');
    await expect(page.locator('body')).toContainText('"ui_surfaces"');
  });

  test('widget is visible on admin pages too', async ({ page }) => {
    await loginLocalAdmin(page);
    await page.goto('/admin/shows/show_spring2025');
    await expectAgentPath(page, '/admin/shows/show_spring2025');
    await expect(page.locator('.agent-access-content')).toContainText(
      'Authorization: Bearer <service token>',
    );
  });
});
