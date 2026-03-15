# Validation

Automated Playwright tests cover all 10 acceptance criteria.
Test file: `tests/customer-service.spec.ts`

## Acceptance Criteria Results

ac-01: PASS — Agent can create, view, assign, reply to, resolve, and close tickets.
  Tests: "AC-01, AC-04: full ticket lifecycle", "AC-01: agent can add internal notes",
         "AC-01: agent can change priority"

ac-02: PASS — Customer creates ticket from public surface, receives reference and secure access.
  Test: "AC-02: create ticket and receive reference + secure link"

ac-03: PASS — Customer views and replies via signed link; recovery via email + ref code.
  Tests: "AC-03: customer can view and reply via secure link",
         "AC-03: ticket lookup by email and reference code"

ac-04: PASS — Ticket states implemented: new, open, pending_customer, resolved, closed.
  Test: "AC-01, AC-04: full ticket lifecycle"

ac-05: PASS — Customer starts live chat, agent replies from internal workspace.
  Test: "AC-05: customer can start chat, agent can reply"

ac-06: PASS — Chat falls back to ticket creation when no agent joins within timeout.
  Test: "AC-06: chat falls back to ticket when no agent joins"

ac-07: PASS — Chat transcript escalated into ticket preserving messages and identity.
  Test: "AC-07: chat can be escalated to ticket"

ac-08: PASS — Admin can create, edit, draft, publish, and unpublish KB articles.
  Test: "AC-08: admin can create, edit, publish, and unpublish articles"

ac-09: PASS — Public visitor can search/browse KB and view article detail.
  Tests: "AC-09: browse knowledge base articles", "AC-09: search knowledge base",
         "AC-09: view article detail with related links"

ac-10: PASS — Ticket creation provides web-based access path; no mailbox sync or AI.
  Test: "AC-10: ticket creation provides access path, not email channel"

## Running Tests

```bash
# Start the app (with short chat timeout for testing)
cd seeds/0003-customer-service-app/realizations/a-web-mvp/artifacts/service-app
CHAT_TIMEOUT=3s go run .

# Run tests
cd tests
npx playwright test
```
