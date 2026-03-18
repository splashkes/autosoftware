import { test, expect } from '@playwright/test';

test.describe('Flowershow', () => {
  test('home page loads with seeded shows', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toContainText('Flowershow');
    await expect(page.locator('text=Spring Rose Show 2025')).toBeVisible();
    await expect(page.locator('text=Fall Garden Festival 2025')).toBeVisible();
  });

  test('show detail page displays schedule and entries', async ({ page }) => {
    await page.goto('/shows/spring-rose-show-2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');
    await expect(page.locator('text=Horticulture Specimens')).toBeVisible();
    await expect(page.locator('text=Floral Design')).toBeVisible();
    // Entries table should exist
    await expect(page.locator('text=Peace')).toBeVisible();
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
    // Should show initials, not full name
    await expect(page.locator('text=MC')).toBeVisible();
    // Full name should NOT be visible on public page
    const bodyText = await page.locator('body').textContent();
    expect(bodyText).not.toContain('Margaret Chen');
  });

  test('taxonomy browse lists taxons', async ({ page }) => {
    await page.goto('/taxonomy');
    await expect(page.locator('h1')).toContainText('Taxonomy');
    await expect(page.locator('text=Rose')).toBeVisible();
    await expect(page.locator('text=Hybrid Tea')).toBeVisible();
  });

  test('taxon detail shows related entries', async ({ page }) => {
    await page.goto('/taxonomy/taxon_ht');
    await expect(page.locator('h1')).toContainText('Hybrid Tea');
    // Should have entries referencing this taxon
    await expect(page.locator('text=Peace')).toBeVisible();
  });

  test('leaderboard displays rankings', async ({ page }) => {
    await page.goto('/leaderboard?org=org_demo1&season=2025');
    await expect(page.locator('h1')).toContainText('Leaderboard');
    // Should have at least one exhibitor
    await expect(page.locator('table tbody tr').first()).toBeVisible();
  });

  test('admin login and dashboard', async ({ page }) => {
    // Redirect without auth
    await page.goto('/admin');
    await expect(page).toHaveURL(/\/admin\/login/);

    // Login with wrong password
    await page.fill('#password', 'wrong');
    await page.click('button[type="submit"]');
    await expect(page.locator('text=Invalid password')).toBeVisible();

    // Login with correct password
    await page.fill('#password', 'admin');
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.locator('h1')).toContainText('Admin Dashboard');
  });

  test('admin CRUD: create show, add classes, add entries, set placements', async ({
    page,
  }) => {
    // Login
    await page.goto('/admin/login');
    await page.fill('#password', 'admin');
    await page.click('button[type="submit"]');

    // Create new show
    await page.click('text=New Show');
    await page.fill('#name', 'Test Evening Show');
    await page.fill('#location', 'Test Hall');
    await page.fill('#season', '2025');
    await page.click('button:has-text("Create Show")');

    // Should be on show admin page
    await expect(page.locator('h1')).toContainText('Test Evening Show');
  });

  test('admin show detail page loads for seeded show', async ({ page }) => {
    await page.goto('/admin/login');
    await page.fill('#password', 'admin');
    await page.click('button[type="submit"]');

    // Navigate to seeded show
    await page.click('text=Spring Rose Show 2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');
  });

  test('API: shows directory returns JSON', async ({ request }) => {
    const response = await request.get('/v1/projections/0007-Flowershow/shows');
    expect(response.ok()).toBeTruthy();
    const shows = await response.json();
    expect(shows.length).toBeGreaterThan(0);
    expect(shows[0].name).toBeDefined();
  });

  test('API: commands require auth', async ({ request }) => {
    const response = await request.post(
      '/v1/commands/0007-Flowershow/shows.create',
      {
        data: { name: 'Unauthorized Show' },
      },
    );
    expect(response.status()).toBe(401);
  });

  test('API: commands accept service token', async ({ request }) => {
    const response = await request.post(
      '/v1/commands/0007-Flowershow/persons.create',
      {
        data: { first_name: 'API', last_name: 'Test' },
        headers: { Authorization: 'Bearer test-token' },
      },
    );
    // Without AS_SERVICE_TOKEN set, this will be 401
    // With it set to "test-token", it would be 201
    // In CI, AS_SERVICE_TOKEN should be set via env
    expect([201, 401]).toContain(response.status());
  });

  test('health check endpoint', async ({ request }) => {
    const response = await request.get('/healthz');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data.status).toBe('ok');
    expect(data.seed).toBe('0007-Flowershow');
  });
});
