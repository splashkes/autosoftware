import { test, expect } from '@playwright/test';
import { expectAgentPath, loginLocalAdmin } from './flowershow.helpers';

test.describe('Flowershow Public', () => {
  test('home page loads with seeded shows and agent widget', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toContainText('Flowershow');
    await expect(page.locator('text=Spring Rose Show 2025')).toBeVisible();
    await expect(page.locator('text=Fall Garden Festival 2025')).toBeVisible();
    await expectAgentPath(page, '/');
  });

  test('mobile nav collapses behind a menu button', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/');
    const toggle = page.getByRole('button', { name: 'Open navigation' });
    await expect(toggle).toBeVisible();
    await expect(page.getByRole('link', { name: 'Clubs', exact: true })).toBeHidden();
    await toggle.click();
    await expect(page.getByRole('link', { name: 'Clubs', exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Browse', exact: true })).toBeVisible();
  });

  test('show detail page displays schedule and entries', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');
    await expect(page.locator('text=Horticulture Specimens')).toBeVisible();
    await expect(page.locator('text=Floral Design')).toBeVisible();
    await expect(page.locator('text=Peace')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Metro Rose Society' }).first()).toHaveAttribute(
      'href',
      '/clubs/org_demo1',
    );
    await expect(page.getByRole('link', { name: 'Open Club' })).toHaveAttribute('href', '/clubs/org_demo1');
    await expectAgentPath(page, '/shows/spring-rose-show-2025');
  });

  test('club detail page shows members, credits, and show history', async ({ page }) => {
    await page.goto('/clubs/org_demo1');
    await expect(page.locator('h1')).toContainText('Metro Rose Society');
    await expect(page.getByRole('heading', { name: 'Upcoming Shows' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Past Shows' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Members & Exhibitors' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Show Credits' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Spring Rose Show 2025' }).first()).toBeVisible();
    await expectAgentPath(page, '/clubs/org_demo1');
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

  test('entry detail media opens in a lightbox and class page shows thumbnails', async ({ page }) => {
    await loginLocalAdmin(page);
    const tinyPng = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9pQ3knwAAAAASUVORK5CYII=',
      'base64',
    );
    const upload = await page.request.post('/admin/entries/entry_01/media', {
      multipart: {
        media: {
          name: 'entry-thumb.png',
          mimeType: 'image/png',
          buffer: tinyPng,
        },
      },
    });
    expect(upload.ok()).toBeTruthy();

    await page.goto('/entries/entry_01');
    await expect(page.getByRole('heading', { name: 'Media' })).toHaveCount(0);
    await page.locator('[data-media-open]').first().click();
    await expect(page.locator('[data-media-lightbox]')).toBeVisible();
    await expect(page.locator('[data-media-lightbox-stage] img')).toBeVisible();
    await expect(page.locator('text=entry-thumb.png')).toHaveCount(0);
    await page.getByRole('button', { name: 'Close image viewer' }).click();

    await page.goto('/shows/spring-rose-show-2025/classes/class_01');
    await expect(page).toHaveURL(/\/shows\/spring-rose-show-2025\/classes\/class_01$/);
    await expect(page.locator('.entry-thumb-grid')).toBeVisible();
    await expect(page.locator('img.entry-row-thumb').first()).toBeVisible();
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
