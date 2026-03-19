import { test, expect } from '@playwright/test';
import {
  ensureAuthStateDir,
  FLOWERSHOW_ADMIN_EMAIL,
  FLOWERSHOW_AUTH_STATE_PATH,
  FLOWERSHOW_REMOTE_E2E,
  expectSignedInLanding,
} from './flowershow.helpers';

test.describe('Flowershow Remote Admin OTP Setup', () => {
  test.skip(!FLOWERSHOW_REMOTE_E2E, 'Set FLOWERSHOW_REMOTE_E2E=1 to run remote OTP setup.');

  test('capture authenticated admin storage state after on-site email-code login', async ({
    page,
  }) => {
    test.setTimeout(0);

    await page.goto('/admin');
    await expect(page).toHaveURL(/\/admin\/login(?:$|\?)/);
    await expect(page.getByRole('heading', { name: 'Sign In' })).toBeVisible();

    await page.fill('#login_email', FLOWERSHOW_ADMIN_EMAIL);
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText(/choose sign-in method/i)).toBeVisible();
    await page.getByRole('button', { name: 'Email Me A Login Code' }).click();

    await expect(page.getByText(/check your email for the sign-in code/i)).toBeVisible();
    await expect(page.getByLabel(/Login Code/i)).toBeVisible();

    console.log(`Enter the OTP for ${FLOWERSHOW_ADMIN_EMAIL}, then resume the paused Playwright session.`);
    await page.pause();

    await expectSignedInLanding(page);

    ensureAuthStateDir();
    await page.context().storageState({ path: FLOWERSHOW_AUTH_STATE_PATH });
  });
});
