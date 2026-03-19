import { test, expect } from '@playwright/test';
import { expectAgentPath, loginLocalAdmin } from './flowershow.helpers';

test.describe('Flowershow Agent Widget', () => {
  test('widget exposes contract links on public pages', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('footer + section.agent-access-shell')).toBeVisible();
    await expect(
      page.getByText('This software is designed to be accessed by people and agents'),
    ).toBeVisible();
    await expect(
      page.getByRole('tab', { name: '(re)design this software' }),
    ).toBeVisible();
    await expect(
      page.getByRole('link', { name: 'GET /v1/contracts', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('link', {
        name: 'GET /v1/contracts/0007-Flowershow/a-firstbloom',
      }),
    ).toBeVisible();

    await page.getByRole('tab', { name: '(re)design this software' }).click();
    await expect(page.getByRole('link', { name: 'Seed README' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Kernel Bootloader' })).toBeVisible();
  });

  test('widget shows current page path and contract detail', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025');
    await expectAgentPath(page, '/shows/spring-rose-show-2025');
    await expect(page.getByRole('link', { name: 'Show projection' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Show workspace projection' })).toBeVisible();

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
      'account-issued agent token with the required permissions',
    );
    await expect(page.getByRole('link', { name: 'Show workspace projection' })).toBeVisible();
    await page.getByRole('tab', { name: '(re)design this software' }).click();
    await expect(page.locator('.agent-access-content')).toContainText(
      'start from the seed documents, the realization contract, and the shared sprout references',
    );
  });
});
