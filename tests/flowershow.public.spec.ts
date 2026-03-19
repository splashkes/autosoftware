import { test, expect } from '@playwright/test';
import { expectAgentPath } from './flowershow.helpers';

test.describe('Flowershow Public', () => {
  test('home page loads with seeded shows and agent widget', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toContainText('Flowershow');
    await expect(page.locator('text=Spring Rose Show 2025')).toBeVisible();
    await expect(page.locator('text=Fall Garden Festival 2025')).toBeVisible();
    await expectAgentPath(page, '/');
  });

  test('show detail page displays schedule and entries', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');
    await expect(page.locator('text=Horticulture Specimens')).toBeVisible();
    await expect(page.locator('text=Floral Design')).toBeVisible();
    await expect(page.locator('text=Peace')).toBeVisible();
    await expectAgentPath(page, '/shows/spring-rose-show-2025');
  });

  test('class browse shows schedule hierarchy', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025/classes');
    await expect(page.locator('h1')).toContainText('Classes');
    await expect(page.locator('text=Hybrid Tea Roses')).toBeVisible();
    await expect(page.locator('text=One Hybrid Tea Bloom')).toBeVisible();
  });

  test('entry detail shows initials only (privacy)', async ({ page }) => {
    await page.goto('/entries/entry_01');
    await expect(page.locator('h1')).toContainText('Peace');
    await expect(page.locator('text=MC')).toBeVisible();
    const bodyText = await page.locator('body').textContent();
    expect(bodyText).not.toContain('Margaret Chen');
  });

  test('taxonomy browse and taxon detail show related entries', async ({ page }) => {
    await page.goto('/taxonomy');
    await expect(page.locator('h1')).toContainText('Taxonomy');
    await expect(page.getByRole('link', { name: /^Rose\b/ }).first()).toBeVisible();
    await expect(
      page.getByRole('link', { name: /^Hybrid Tea\b/ }).first(),
    ).toBeVisible();

    await page.goto('/taxonomy/taxon_ht');
    await expect(page.locator('h1')).toContainText('Hybrid Tea');
    await expect(page.locator('text=Peace')).toBeVisible();
  });

  test('leaderboard displays rankings', async ({ page }) => {
    await page.goto('/leaderboard?org=org_demo1&season=2025');
    await expect(page.locator('h1')).toContainText('Leaderboard');
    await expect(page.locator('table tbody tr').first()).toBeVisible();
  });
});
