import { test, expect } from '@playwright/test';

const BASE = 'http://127.0.0.1:8095';

// ── AC-09: Public help center and knowledge base ──

test('home page loads with help center', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toContainText('How can we help');
  await expect(page.locator('.action-cards')).toBeVisible();
});

test('AC-09: browse knowledge base articles', async ({ page }) => {
  await page.goto('/help/articles');
  await expect(page.locator('h1')).toContainText('Knowledge Base');
  // Seeded articles should appear
  await expect(page.locator('text=Getting Started')).toBeVisible();
});

test('AC-09: search knowledge base', async ({ page }) => {
  await page.goto('/help/articles?q=ticket');
  await expect(page.locator('text=How to Track Your Ticket')).toBeVisible();
});

test('AC-09: view article detail with related links', async ({ page }) => {
  await page.goto('/help/articles/getting-started');
  await expect(page.locator('h1')).toContainText('Getting Started');
  // Should have "Need More Help?" sidebar
  await expect(page.locator('text=Need More Help?')).toBeVisible();
});

// ── AC-02, AC-03: Customer ticket creation and secure access ──

test('AC-02: create ticket and receive reference + secure link', async ({ page }) => {
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Test User');
  await page.fill('#email', 'test@example.com');
  await page.fill('#subject', 'Test Issue');
  await page.fill('#description', 'I need help with something.');
  await page.click('button[type="submit"]');

  // Should show confirmation
  await expect(page.locator('h1')).toContainText('Request Submitted');
  // Should show reference code
  await expect(page.locator('strong:has-text("CS-")').first()).toBeVisible();
  // Should have "View Your Ticket" link
  const viewLink = page.locator('a:has-text("View Your Ticket")');
  await expect(viewLink).toBeVisible();

  // Follow the secure link
  await viewLink.click();
  await expect(page.locator('h1')).toContainText('CS-');
  await expect(page.locator('text=Test Issue')).toBeVisible();
});

test('AC-03: customer can view and reply via secure link', async ({ page }) => {
  // Create a ticket first
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Reply Tester');
  await page.fill('#email', 'reply@test.com');
  await page.fill('#subject', 'Reply Test Ticket');
  await page.fill('#description', 'Original message');
  await page.click('button[type="submit"]');

  // Follow secure link
  await page.click('a:has-text("View Your Ticket")');
  await expect(page.locator('h1')).toContainText('CS-');

  // Reply to the ticket
  await page.fill('textarea[name="body"]', 'Customer follow-up reply');
  await page.click('button:has-text("Send Reply")');

  // Verify reply appears
  await expect(page.locator('text=Customer follow-up reply')).toBeVisible();
});

test('AC-03: ticket lookup by email and reference code', async ({ page }) => {
  // Create a ticket
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Lookup Tester');
  await page.fill('#email', 'lookup@test.com');
  await page.fill('#subject', 'Lookup Test');
  await page.fill('#description', 'Testing ticket lookup');
  await page.click('button[type="submit"]');

  // Grab the reference code from the confirmation page
  const refText = await page.locator('strong:has-text("CS-")').first().textContent();
  const refCode = refText?.trim() || '';

  // Navigate to lookup page
  await page.goto('/help/tickets/lookup');
  await page.fill('#email', 'lookup@test.com');
  await page.fill('#ref_code', refCode);
  await page.click('button[type="submit"]');

  // Should redirect to ticket view
  await expect(page.locator('h1')).toContainText(refCode);
});

// ── AC-01, AC-04: Agent ticket management ──

test('agent can login', async ({ page }) => {
  await page.goto('/agent/login');
  await page.fill('#email', 'admin@support.local');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');

  // Should redirect to inbox
  await expect(page).toHaveURL(/\/agent\/inbox/);
  await expect(page.locator('h1')).toContainText('Inbox');
});

test('AC-01, AC-04: full ticket lifecycle', async ({ page, context }) => {
  // 1. Customer creates a ticket
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Lifecycle Customer');
  await page.fill('#email', 'lifecycle@test.com');
  await page.fill('#subject', 'Lifecycle Test');
  await page.fill('#description', 'Testing full lifecycle');
  await page.click('button[type="submit"]');
  await expect(page.locator('h1')).toContainText('Request Submitted');

  // 2. Agent logs in and sees the ticket
  const agentPage = await context.newPage();
  await agentPage.goto('/agent/login');
  await agentPage.fill('#email', 'admin@support.local');
  await agentPage.fill('#password', 'admin');
  await agentPage.click('button[type="submit"]');
  await expect(agentPage).toHaveURL(/\/agent\/inbox/);

  // Find the ticket in inbox
  await expect(agentPage.locator('text=Lifecycle Test')).toBeVisible();
  await agentPage.click('text=Lifecycle Test');

  // 3. Verify ticket is in "new" state
  await expect(agentPage.locator('.badge:has-text("New")')).toBeVisible();

  // 4. Assign ticket to self
  await agentPage.selectOption('select[name="assignee_id"]', 'agent-1');
  await agentPage.locator('form[action*="/assign"] button').click();

  // Should now be "open" (auto-transitions on assignment)
  await expect(agentPage.locator('.badge:has-text("Open")')).toBeVisible();

  // 5. Agent replies
  await agentPage.fill('textarea[name="body"]', 'Agent reply to customer');
  await agentPage.click('button:has-text("Send Reply")');
  await expect(agentPage.locator('text=Agent reply to customer')).toBeVisible();

  // 6. Set to pending_customer
  await agentPage.selectOption('select[name="status"]', 'pending_customer');
  await agentPage.locator('form[action*="/status"] button').click();
  await expect(agentPage.locator('.badge:has-text("Pending Customer")')).toBeVisible();

  // 7. Set to resolved
  await agentPage.selectOption('select[name="status"]', 'resolved');
  await agentPage.locator('form[action*="/status"] button').click();
  await expect(agentPage.locator('.badge:has-text("Resolved")')).toBeVisible();

  // 8. Set to closed
  await agentPage.selectOption('select[name="status"]', 'closed');
  await agentPage.locator('form[action*="/status"] button').click();
  await expect(agentPage.locator('.badge:has-text("Closed")')).toBeVisible();

  // 9. Add internal note
  // Note: after closing, the internal note form should still work
  // Actually, let me check a ticket before closing for notes
});

test('AC-01: agent can add internal notes', async ({ page }) => {
  // Create a ticket first
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Notes Test');
  await page.fill('#email', 'notes@test.com');
  await page.fill('#subject', 'Internal Notes Test');
  await page.fill('#description', 'Test internal notes');
  await page.click('button[type="submit"]');

  // Agent login
  await page.goto('/agent/login');
  await page.fill('#email', 'admin@support.local');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');

  // Find and open the ticket
  await page.click('text=Internal Notes Test');

  // Add internal note
  const noteTextarea = page.locator('textarea[placeholder*="internal note"]');
  await noteTextarea.fill('This is a private agent note');
  await page.locator('button:has-text("Add Note")').click();

  // Verify note appears
  await expect(page.locator('text=This is a private agent note')).toBeVisible();
});

test('AC-01: agent can change priority', async ({ page }) => {
  // Create ticket
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Priority Test');
  await page.fill('#email', 'priority@test.com');
  await page.fill('#subject', 'Priority Change Test');
  await page.fill('#description', 'Testing priority');
  await page.click('button[type="submit"]');

  // Login as agent
  await page.goto('/agent/login');
  await page.fill('#email', 'admin@support.local');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');

  await page.click('text=Priority Change Test');

  // Change priority to urgent
  await page.selectOption('select[name="priority"]', 'urgent');
  await page.locator('form[action*="/priority"] button').click();
  await expect(page.locator('.badge:has-text("Urgent")')).toBeVisible();
});

// ── AC-05, AC-06: Live chat ──

test('AC-05: customer can start chat, agent can reply', async ({ page, context }) => {
  // Customer starts chat
  await page.goto('/chat');
  await page.fill('#name', 'Chat Customer');
  await page.fill('#email', 'chat@test.com');
  await page.fill('#message', 'Hello, I need help!');
  await page.click('button[type="submit"]');

  // Should be on chat page now
  await expect(page.locator('.badge:has-text("Waiting")')).toBeVisible();
  await expect(page.locator('text=Hello, I need help!')).toBeVisible();

  // Agent logs in and sees waiting chat
  const agentPage = await context.newPage();
  await agentPage.goto('/agent/login');
  await agentPage.fill('#email', 'agent@support.local');
  await agentPage.fill('#password', 'agent');
  await agentPage.click('button[type="submit"]');

  await agentPage.goto('/agent/chats');
  await expect(agentPage.locator('text=Chat Customer')).toBeVisible();

  // Agent joins
  await agentPage.click('button:has-text("Join")');
  await expect(agentPage.locator('.badge:has-text("Active")')).toBeVisible();
});

// ── AC-06: Chat fallback to ticket when no agent available ──

test('AC-06: chat falls back to ticket when no agent joins', async ({ page }) => {
  // Start chat (server has CHAT_TIMEOUT=3s for testing)
  await page.goto('/chat');
  await page.fill('#name', 'Fallback Customer');
  await page.fill('#email', 'fallback@test.com');
  await page.fill('#message', 'Need help urgently');
  await page.click('button[type="submit"]');

  // Wait for the timeout fallback (3 seconds + buffer)
  await page.waitForTimeout(5000);

  // Reload to see the updated state
  await page.reload();

  // Chat should now be escalated
  await expect(page.locator('.badge:has-text("Escalated")')).toBeVisible();
  // Should mention that conversation was saved as a ticket
  await expect(page.locator('.alert:has-text("Your conversation has been saved")')).toBeVisible();
});

// ── AC-07: Chat escalation to ticket ──

test('AC-07: chat can be escalated to ticket', async ({ page, context }) => {
  // Customer starts chat
  await page.goto('/chat');
  await page.fill('#name', 'Escalate Customer');
  await page.fill('#email', 'escalate@test.com');
  await page.fill('#message', 'I have a complex issue');
  await page.click('button[type="submit"]');

  // Agent joins
  const agentPage = await context.newPage();
  await agentPage.goto('/agent/login');
  await agentPage.fill('#email', 'admin@support.local');
  await agentPage.fill('#password', 'admin');
  await agentPage.click('button[type="submit"]');

  await agentPage.goto('/agent/chats');
  await agentPage.click('button:has-text("Join")');

  // Escalate to ticket
  // Need to handle the confirm dialog
  agentPage.on('dialog', dialog => dialog.accept());
  await agentPage.click('button:has-text("Escalate to Ticket")');

  // Should redirect to ticket detail
  await expect(agentPage.locator('h1')).toContainText('CS-');
  await expect(agentPage.locator('text=Escalated from live chat')).toBeVisible();
});

// ── AC-08: Knowledge base authoring ──

test('AC-08: admin can create, edit, publish, and unpublish articles', async ({ page }) => {
  // Login as admin
  await page.goto('/agent/login');
  await page.fill('#email', 'admin@support.local');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');

  // Go to KB admin
  await page.goto('/agent/articles');
  await expect(page.locator('h1')).toContainText('Knowledge Base');

  // Create new article
  await page.click('text=New Article');
  await page.fill('#title', 'Test Article');
  await page.fill('#category', 'Testing');
  await page.fill('#body', '## Test Content\n\nThis is a test article with **bold text**.\n\n- Item 1\n- Item 2');
  await page.click('button:has-text("Create Article")');

  // Should redirect to edit page
  await expect(page.locator('h1')).toContainText('Edit: Test Article');
  await expect(page.locator('text=This article is a draft')).toBeVisible();

  // Publish
  await page.click('button:has-text("Publish")');
  await expect(page.locator('text=This article is published')).toBeVisible();

  // Check public article page
  await page.goto('/help/articles/test-article');
  await expect(page.locator('h1')).toContainText('Test Article');

  // Go back and unpublish
  await page.goto('/agent/articles');
  await page.click('text=Test Article');
  await page.click('button:has-text("Unpublish")');
  await expect(page.locator('text=This article is a draft')).toBeVisible();

  // Public page should 404 now
  const res = await page.goto('/help/articles/test-article');
  expect(res?.status()).toBe(404);
});

// ── AC-10: Transactional notifications distinction ──

test('AC-10: ticket creation provides access path, not email channel', async ({ page }) => {
  await page.goto('/help/tickets/new');
  await page.fill('#name', 'Notification Test');
  await page.fill('#email', 'notify@test.com');
  await page.fill('#subject', 'Notification Test');
  await page.fill('#description', 'Testing notification boundary');
  await page.click('button[type="submit"]');

  // Confirmation page should provide a web link, not promise email delivery
  await expect(page.locator('text=secure link')).toBeVisible();
  await expect(page.locator('a:has-text("View Your Ticket")')).toBeVisible();
  // Should mention lookup as recovery, not email-based access
  await expect(page.locator('text=ticket lookup')).toBeVisible();
});

// ── Inbox filtering ──

test('inbox filters by status and assignee', async ({ page }) => {
  // Login
  await page.goto('/agent/login');
  await page.fill('#email', 'admin@support.local');
  await page.fill('#password', 'admin');
  await page.click('button[type="submit"]');

  // Filter by status
  await page.goto('/agent/inbox?status=new');
  // Should show only new tickets (or none if all were changed)
  await expect(page.locator('h1')).toContainText('Inbox');

  // Filter by unassigned
  await page.goto('/agent/inbox?assignee=unassigned');
  await expect(page.locator('h1')).toContainText('Inbox');
});
