# Brief

Build a customer service app that helps a small support team handle tickets,
reply in live chat, and publish a searchable knowledge base.

Desired initial outcome:

- one web app with public and agent-facing surfaces
- support tickets with status, assignment, priority, and reply history
- customers can continue a ticket from a secure web link without creating a
  full account
- live chat that can escalate into a ticket
- knowledge base articles that agents can author and customers can read
- clear MVP boundaries so implementation can start without product drift

Primary users:

- support agent working an inbox
- support lead triaging and assigning work
- customer asking for help or reading self-service content

Scope:

- a shared ticket inbox with filtering by status, assignee, and priority
- a ticket detail view with conversation history, internal notes, and explicit
  customer-facing status
- a public support form that collects contact details and issues a signed
  ticket access link plus a simple reference code
- a customer ticket page and reply flow using the secure link or email-plus-
  code recovery
- a basic website chat widget and matching agent chat console, with fallback
  to ticket creation when no agent is available
- knowledge base article CRUD for admins and public article pages for readers
- one small-team deployment with no multi-tenant complexity

Constraints:

- prefer a single deployable web application
- keep integrations minimal in v1
- defer AI, mailbox sync, SMS, and complex workflow automation
- allow basic transactional notifications if needed for ticket access, but not
  email-as-a-support-channel
- keep the first release understandable enough to implement inside one seed
