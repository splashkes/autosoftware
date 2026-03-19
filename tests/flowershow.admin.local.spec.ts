import { test, expect } from '@playwright/test';
import {
  expectAgentPath,
  loginLocalAdmin,
  loginLocalIntakeOperator,
  loginLocalViewer,
  uniqueName,
} from './flowershow.helpers';

test.describe('Flowershow Admin Local', () => {
  test('admin login and dashboard work locally', async ({ page }) => {
    await page.goto('/admin');
    await expect(page).toHaveURL(/\/admin\/login/);
    await expect(page.locator('body')).not.toContainText('Bootstrap Override');

    await loginLocalAdmin(page);
  });

  test('admin reaches the shared account token manager and can issue a scoped agent token', async ({
    page,
    request,
  }) => {
    await loginLocalAdmin(page);
    await page.goto('/admin');
    await page.getByRole('link', { name: 'Agent / API Access Tokens' }).click();

    await expect(page).toHaveURL(/\/account#agent-tokens$/);
    await expect(page.locator('body')).toContainText('Agent / API access tokens');

    await page.fill('#token_label', uniqueName('Playwright Operator Token'));
    await page.fill('#expires_in_days', '7');
    await page
      .locator('label.account-token-profile:has-text("Show Operator") input[type="radio"]')
      .check();
    await page.getByRole('button', { name: 'Generate Agent Token' }).click();

    await expect(page.getByRole('heading', { name: 'Copy This Token Now' })).toBeVisible();
    await expect(page.locator('body')).not.toContainText('Issue A New Agent / API Access Token');
    await expect(page.locator('body')).not.toContainText('Issued Agent / API Access Tokens');
    await expect(page.getByRole('button', { name: 'Copy Token' })).toBeVisible();
    const tokenField = page.locator('[data-issued-agent-token]');
    await expect(tokenField).toBeVisible();
    const token = await tokenField.inputValue();
    expect(token).toMatch(/^fsa_/);

    const account = await request.get(
      '/v1/projections/0007-Flowershow/account',
      {
        headers: { Authorization: `Bearer ${token}` },
      },
    );
    expect(account.ok()).toBeTruthy();

    const rolesAssign = await request.post(
      '/v1/commands/0007-Flowershow/roles.assign',
      {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        data: {
          cognito_sub: 'sub_playwright_forbidden',
          role: 'admin',
        },
      },
    );
    expect(rolesAssign.status()).toBe(403);
    expect(await rolesAssign.json()).toMatchObject({
      error: {
        code: 'permission_denied',
        auth_mode: 'agent_token',
      },
    });
  });

  test('signed-in non-admin lands on account instead of looping through admin', async ({
    page,
  }) => {
    await loginLocalViewer(page);
    await page.goto('/admin');

    await expect(page).toHaveURL(/\/account\?notice=admin_required$/);
    await expect(page.locator('h1')).toContainText('Your Profile');
    await expect(page.locator('body')).toContainText(
      'does not currently have admin access',
    );
  });

  test('admin can assign a judge on a seeded show', async ({ page }) => {
    await loginLocalAdmin(page);
    await page.goto('/admin/shows/show_spring2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');

    const judgeSelect = page.locator('form[action$="/judges"] select[name="person_id"]');
    await judgeSelect.selectOption({ index: 1 });
    await page
      .locator('form[action$="/judges"] button:has-text("Assign Judge")')
      .click();

    await expect(page.locator('#admin-info-panel')).toContainText('assigned');
  });

  test('admin can create schedule hierarchy, add an entry, and suppress it from public view', async ({
    page,
  }) => {
    await loginLocalAdmin(page);

    const showName = uniqueName('Playwright Evening Show');
    await page.goto('/admin/shows/new');
    await page.fill('#name', showName);
    await page.fill('#location', 'Playwright Hall');
    await page.fill('#season', '2026');
    await page.click('button:has-text("Create Show")');

    await expect(page.locator('h1')).toContainText(showName);
    const adminShowPath = new URL(page.url()).pathname;
    const publicPath = await page
      .getByRole('link', { name: 'View Public' })
      .getAttribute('href');
    expect(publicPath).toBeTruthy();

    await page.getByRole('button', { name: 'Schedule' }).click();
    const scheduleForm = page.locator('form[action$="/schedule"]');
    await scheduleForm.locator('input[name="notes"]').fill(
      'Local test schedule with HTMX parity coverage.',
    );
    await scheduleForm.getByRole('button').click();
    await expect(page.locator('#admin-schedule-panel')).toContainText(
      'Update Schedule Governance',
    );

    await page.locator('summary:has-text("+ Add Division")').click();
    const divisionForm = page.locator('form[action$="/divisions"]');
    await divisionForm.locator('input[name="code"]').fill('I');
    await divisionForm.locator('input[name="title"]').fill('Playwright Division');
    await divisionForm.locator('select[name="domain"]').selectOption('horticulture');
    await divisionForm.locator('button:has-text("Add Division")').click();
    await expect(page.locator('#admin-schedule-panel')).toContainText(
      'Playwright Division',
    );

    await page.locator('summary:has-text("+ Add Section")').click();
    const sectionForm = page.locator('form[action$="/sections"]');
    await sectionForm.locator('input[name="code"]').fill('A');
    await sectionForm.locator('input[name="title"]').fill('Playwright Section');
    await sectionForm.locator('button:has-text("Add Section")').click();
    await expect(page.locator('#admin-schedule-panel')).toContainText(
      'Playwright Section',
    );

    await page.locator('summary:has-text("+ Add Class")').click();
    const classForm = page.locator('form[action$="/classes"]');
    await classForm.locator('input[name="class_number"]').fill('12');
    await classForm.locator('input[name="title"]').fill('Playwright Bloom Class');
    await classForm.locator('select[name="domain"]').selectOption('horticulture');
    await classForm.locator('input[name="description"]').fill(
      'Browser-created class for parity coverage.',
    );
    await classForm.locator('button:has-text("Add Class")').click();
    await expect(page.locator('#admin-schedule-panel')).toContainText(
      'Playwright Bloom Class',
    );

    await page.getByRole('button', { name: 'Entries' }).click();
    const entryForm = page.locator('form[action$="/entries"]');
    await entryForm
      .locator('select[name="class_id"]')
      .selectOption('12: Playwright Bloom Class');
    await entryForm.locator('select[name="person_id"]').selectOption({ index: 1 });
    await entryForm.locator('input[name="name"]').fill('Playwright Peace');
    await entryForm.locator('input[name="notes"]').fill('Created in Playwright.');
    await entryForm.locator('button:has-text("Add Entry")').click();
    await expect(page.locator('#admin-entries-panel')).toContainText(
      'Playwright Peace',
    );

    await page.goto(publicPath!);
    await expect(page.locator('body')).toContainText('Playwright Peace');
    await expectAgentPath(page, publicPath!);

    await page.goto(adminShowPath);
    await page.getByRole('button', { name: 'Entries' }).click();
    const entryRow = page.locator('tr', { hasText: 'Playwright Peace' });
    await entryRow.locator('button:has-text("Suppress")').click();
    await expect(page.locator('#admin-entries-panel')).toContainText('suppressed');

    await page.goto(publicPath!);
    await expect(page.locator('body')).not.toContainText('Playwright Peace');
  });

  test('show intake operator can work inside one show without global admin access', async ({
    page,
  }) => {
    await loginLocalIntakeOperator(page);

    await page.goto('/admin/shows/show_spring2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');

    await page.getByRole('button', { name: 'Entries' }).click();
    const filter = page.locator('[data-person-filter-input]').first();
    await expect(filter).toBeVisible();
    await filter.fill('susan');

    const visibleOptions = await page
      .locator('#entry-create-person-select option:not([hidden])')
      .allTextContents();
    expect(visibleOptions.join(' | ')).toContain('Susan Park');
    expect(visibleOptions.join(' | ')).not.toContain('Margaret Chen');

    await page.goto('/admin/roles');
    await expect(page).toHaveURL(/\/account\?notice=admin_required$/);
    await expect(page.locator('h1')).toContainText('Your Profile');
  });
});
