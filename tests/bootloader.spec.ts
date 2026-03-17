import { test, expect } from '@playwright/test';

test('bootloader homepage shows featured systems and readiness distinction', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByText('Featured Systems')).toBeVisible();
  await expect(page.locator('.footprint-label').first()).toBeVisible();
  await expect(page.getByText(/Running Now|Runnable But Idle/).first()).toBeVisible();
  await expect(page.locator('.featured-card').first()).toBeVisible();
});
