import { test, expect } from '@playwright/test';
import {
  ensureAuthStateDir,
  FLOWERSHOW_ADMIN_EMAIL,
  FLOWERSHOW_AUTH_STATE_PATH,
  FLOWERSHOW_REMOTE_E2E,
} from './flowershow.helpers';

test.describe('Flowershow Remote Admin OTP Setup', () => {
  test.skip(!FLOWERSHOW_REMOTE_E2E, 'Set FLOWERSHOW_REMOTE_E2E=1 to run remote OTP setup.');

  test('capture authenticated admin storage state after OTP login', async ({ page }) => {
    test.setTimeout(0);

    await page.goto('/admin');
    if (page.url().includes('/admin/login')) {
      await expect(page.getByText(/Cognito is enabled/i)).toBeVisible();
      await page.getByRole('link', { name: /Continue With Cognito/i }).click();
    }

    // Complete the hosted UI and OTP flow manually for FLOWERSHOW_ADMIN_EMAIL,
    // then resume the test once the browser has returned to /admin.
    console.log(
      `Complete the OTP login for ${FLOWERSHOW_ADMIN_EMAIL}, then resume the paused Playwright session.`,
    );
    await page.pause();

    await expect(page).toHaveURL(/\/admin(?:$|\?)/);
    await expect(page.locator('h1')).toContainText('Admin Dashboard');

    ensureAuthStateDir();
    await page.context().storageState({ path: FLOWERSHOW_AUTH_STATE_PATH });
  });
});
