import path from 'node:path';
import { test, expect } from '@playwright/test';
import {
  expectAgentPath,
  loginLocalAdmin,
  loginLocalClubAdmin,
  loginLocalIntakeOperator,
  loginLocalViewer,
  uniqueName,
} from './flowershow.helpers';

const fixtureImage = path.join(process.cwd(), 'node_modules/playwright-core/lib/server/chromium/appIcon.png');

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
    await page.goto('/account?section=access');

    await expect(page).toHaveURL(/\/account\?section=access$/);
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

    await page.locator('summary:has-text("Assign judge")').click();
    const judgeSelect = page.locator('form[action$="/judges"] select[name="person_id"]');
    await judgeSelect.selectOption({ index: 1 });
    await page
      .locator('form[action$="/judges"]')
      .evaluate((form: HTMLFormElement) => form.requestSubmit());

    await expect(page.locator('#admin-setup-panel')).toContainText('assigned');
  });

  test('admin can create a judge profile from the judge directory', async ({ page }) => {
    await loginLocalAdmin(page);
    await page.goto('/admin?section=judges');

    const firstName = uniqueName('Priya');
    const emailSlug = firstName.toLowerCase().replace(/[^a-z0-9]+/g, '-');
    await page.fill('#judge_first_name', firstName);
    await page.fill('#judge_last_name', 'Tester');
    await page.fill('#judge_email', `${emailSlug}@example.com`);
    await page.fill('#judge_phone', '555-0105');
    await page.fill('#judge_specialties', 'Dahlias, specimen judging');
    await page.fill('#judge_qualifications', 'National panel judge');
    await page.fill('#judge_notes', 'Prefers morning assignments.');
    await page.getByRole('button', { name: 'Add Judge' }).click();

    await expect(page).toHaveURL(/\/admin\/judges\/person_/);
    await expect(page.locator('h1')).toContainText(`${firstName} Tester`);
    await expect(page.locator('body')).toContainText('National panel judge');
    await expect(page.locator('body')).toContainText('Flowers Chosen First');
  });

  test('show admin uses the shared sidebar shell and switches sidebar links with the active tab', async ({
    page,
  }) => {
    await loginLocalAdmin(page);
    await page.goto('/admin/shows/show_spring2025');

    const sidebar = page.locator('[data-show-admin-shell] .account-sidebar');
    const activeNav = sidebar.locator('[data-show-admin-nav].is-active');
    await expect(sidebar).toBeVisible();
    await expect(activeNav).toContainText('Show Basics');
    await expect(activeNav).not.toContainText('Add Entry');

    await page.getByRole('button', { name: 'Intake' }).click();
    await expect(activeNav).toContainText('Class Intake');
    await expect(activeNav).toContainText('Hybrid Tea Roses');
    await expect(activeNav).not.toContainText('Show Basics');

    await page.getByRole('button', { name: 'Governance' }).click();
    await expect(activeNav).toContainText('Standards');
    await expect(activeNav).toContainText('Effective Rules');
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

    const scheduleForm = page.locator('form[action$="/schedule"]');
    await scheduleForm.locator('input[name="notes"]').fill(
      'Local test schedule with HTMX parity coverage.',
    );
    await scheduleForm.getByRole('button').click();
    await expect(page.locator('#admin-setup-panel')).toContainText(
      'Update Schedule Governance',
    );

    await page.locator('summary:has-text("Add division")').click();
    const divisionForm = page.locator('form[action$="/divisions"]');
    await divisionForm.locator('input[name="code"]').fill('I');
    await divisionForm.locator('input[name="title"]').fill('Playwright Division');
    await divisionForm.locator('select[name="domain"]').selectOption('horticulture');
    await divisionForm.locator('button:has-text("Add Division")').click();
    await expect(page.locator('#admin-setup-panel')).toContainText(
      'Playwright Division',
    );

    await page.locator('summary:has-text("Add section")').click();
    const sectionForm = page.locator('form[action$="/sections"]');
    await sectionForm.locator('input[name="code"]').fill('A');
    await sectionForm.locator('input[name="title"]').fill('Playwright Section');
    await sectionForm.locator('button:has-text("Add Section")').click();
    await expect(page.locator('#admin-setup-panel')).toContainText(
      'Playwright Section',
    );

    await page.locator('summary:has-text("Add class")').click();
    const classForm = page.locator('form[action$="/classes"]');
    await classForm.locator('input[name="class_number"]').fill('12');
    await classForm.locator('input[name="title"]').fill('Playwright Bloom Class');
    await classForm.locator('select[name="domain"]').selectOption('horticulture');
    await classForm.locator('input[name="description"]').fill(
      'Browser-created class for parity coverage.',
    );
    await classForm.locator('button:has-text("Add Class")').click();
    await expect(page.locator('#admin-setup-panel')).toContainText(
      'Playwright Bloom Class',
    );

    await page.getByRole('button', { name: 'Intake' }).click();
    await page
      .locator('[data-intake-modal-open][data-intake-mode="new"][data-intake-class-label="12: Playwright Bloom Class"]')
      .click();
    await page.locator('[data-intake-entrant-input]').fill('Margaret Chen · member · Metro Rose Society');
    await page.locator('form[data-intake-entry-form] input[name="name"]').fill('Playwright Peace');
    await page.locator('form[data-intake-entry-form] input[name="notes"]').fill('Created in Playwright.');
    await page
      .locator('form[data-intake-entry-form] [data-intake-capture-input="photo"]')
      .setInputFiles(fixtureImage);
    await expect(page.locator('form[data-intake-entry-form] [data-intake-upload-queue]')).toContainText('appIcon.jpg');
    await page.getByRole('button', { name: 'Save Entry' }).click();
    await expect(page.locator('#admin-intake-panel')).toContainText(
      'Playwright Peace',
    );

    await page.getByRole('button', { name: 'Board' }).click();
    await expect(page.locator('#admin-board-panel')).toContainText('Playwright Peace');

    await page.goto(publicPath!);
    await expect(page.locator('body')).toContainText('Playwright Peace');
    await expectAgentPath(page, publicPath!);

    await page.goto(adminShowPath);
    await page.getByRole('button', { name: 'Floor' }).click();
    const entryRow = page.locator('tr', { hasText: 'Playwright Peace' });
    await entryRow.locator('button:has-text("Suppress")').click();
    await expect(page.locator('#admin-floor-panel')).toContainText('suppressed');

    await page.goto(publicPath!);
    await expect(page.locator('body')).not.toContainText('Playwright Peace');
  });

  test('show intake operator can work inside one show without global admin access', async ({
    page,
  }) => {
    await loginLocalIntakeOperator(page);

    await page.goto('/admin/shows/show_spring2025');
    await expect(page.locator('h1')).toContainText('Spring Rose Show 2025');

    await page.getByRole('button', { name: 'Intake' }).click();
    await page.locator('[data-intake-modal-open][data-intake-mode="new"]').first().click();
    const entrantInput = page.locator('[data-intake-entrant-input]');
    await expect(entrantInput).toBeVisible();
    await entrantInput.fill('Susan Park · guest · Metro Rose Society');
    await entrantInput.blur();
    await expect(page.locator('[data-intake-person-id-input]')).not.toHaveValue('');

    await page.goto('/admin/roles');
    await expect(page).toHaveURL(/\/account\?notice=admin_required$/);
    await expect(page.locator('h1')).toContainText('Your Profile');
  });

  test('club admin can use the club workspace and create an invite without global admin access', async ({
    page,
  }) => {
    await loginLocalClubAdmin(page);

    await page.goto('/admin/clubs/org_demo1');
    await expect(page.locator('h1')).toContainText('Metro Rose Society');
    await page.locator('summary:has-text("Invite")').click();

    await page.fill('#invite_first_name', 'Taylor');
    await page.fill('#invite_last_name', 'Grant');
    await page.fill('#invite_email', `taylor+${Date.now()}@example.com`);
    await page.locator('summary:has-text("Special roles")').click();
    await page
      .locator('label.account-token-profile:has-text("Club Admin") input[type="checkbox"]')
      .check();
    await page.getByRole('button', { name: 'Send Invite' }).click();

    await expect(page).toHaveURL(/\/admin\/clubs\/org_demo1\?section=invites(?:&notice=.*)?#club-invites$/);
    await expect(page.locator('body')).toContainText('Invite sent to Taylor Grant');
    await expect(page.locator('body')).toContainText('Taylor Grant');

    await page.goto('/admin');
    await expect(page).toHaveURL(/\/account\?notice=admin_required$/);
  });
});
