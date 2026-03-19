import { test, expect } from '@playwright/test';
import {
  ensureAuthStateDir,
  FLOWERSHOW_ADMIN_EMAIL,
  FLOWERSHOW_AUTH_STATE_PATH,
  FLOWERSHOW_REMOTE_E2E,
} from './flowershow.helpers';

test.describe('Flowershow Remote Admin OTP Setup', () => {
  test.skip(!FLOWERSHOW_REMOTE_E2E, 'Set FLOWERSHOW_REMOTE_E2E=1 to run remote OTP setup.');

  test('capture authenticated admin storage state after on-site email-code login', async ({
    page,
  }) => {
    test.setTimeout(0);

    await page.goto('/admin');
    await expect(page).toHaveURL(/\/admin\/login(?:$|\?)/);
    await expect(page.getByRole('heading', { name: 'Admin Login' })).toBeVisible();

    await page.fill('#login_email', FLOWERSHOW_ADMIN_EMAIL);
    await page.getByRole('button', { name: 'Get Email Login Code' }).click();

    await expect(page.getByText(/check your email for the sign-in code/i)).toBeVisible();
    await expect(page.getByLabel(/Login Code/i)).toBeVisible();

    console.log(`Enter the OTP for ${FLOWERSHOW_ADMIN_EMAIL}, then resume the paused Playwright session.`);
    await page.pause();

    await expect(page).toHaveURL(/\/admin(?:$|\?)/);
    await expect(page.locator('h1')).toContainText('Admin Dashboard');

    ensureAuthStateDir();
    await page.context().storageState({ path: FLOWERSHOW_AUTH_STATE_PATH });
  });
});
