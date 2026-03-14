# Design

This seed defines a pragmatic customer support MVP with three connected
workflows: ticket handling, live chat, and self-service content.

## Product Shape

The application should have:

- a public help surface for searching articles and starting a chat or ticket
- an internal support workspace for agents
- a lightweight admin surface for managing knowledge base content

The first release should assume one organization and one support team.

## Actors and Access

- Public visitor: can browse articles, start a ticket, or start a chat.
- Ticket requester: does not need a full account in v1, but must provide a
  name and email address when opening a ticket. The product issues a signed
  ticket link and a short reference code.
- Agent: authenticated staff member who works tickets and chat sessions.
- Support lead or admin: authenticated staff member who manages assignments and
  knowledge base publishing.

The signed ticket link is the primary customer-access mechanism in v1. If a
requester loses the link, they can recover access by providing their email
address and ticket reference code through a lightweight lookup flow.

Basic transactional notifications are allowed for sending ticket links or
status updates. Treating email as a support channel, including inbox sync or
reply-by-email handling, remains out of scope for v1.

## Core Workflows

### Ticket Workflow

Agents should be able to:

- see tickets grouped by status
- assign tickets to themselves or another agent
- change priority and status
- send customer-visible replies
- leave internal notes that are not visible to customers

Customers should be able to:

- submit a help request from a public form with contact details
- receive a simple ticket reference and a signed access link
- open the secure ticket page later without creating an account
- reply in the web conversation thread through that secure ticket page

When an agent replies, the ticket should remain accessible through the same
signed link. If notifications are enabled, they should point the requester back
to the web thread rather than attempting reply-by-email.

### Live Chat Workflow

Customers should be able to open a website chat and send messages without
creating a full account.

Agents should be able to:

- see waiting and active chat sessions
- join a chat
- reply in real time or near-real time
- convert a chat transcript into a ticket when follow-up is needed

If no agent joins within a configured waiting window, or if chat is started
outside staffed hours, the product should fall back to ticket creation instead
of pretending the chat is staffed. That fallback should preserve the customer
message history already entered.

### Knowledge Base Workflow

Agents or admins should be able to:

- create articles
- save drafts
- publish or unpublish articles
- organize articles into categories

Customers should be able to:

- search articles
- browse by category
- open a public article page with clear related-help links

## State Model

### Ticket States

- `new`: newly created ticket that has not yet been picked up.
- `open`: actively being worked by an agent.
- `pending_customer`: agent is waiting for requester input.
- `resolved`: agent believes the issue is solved, but the requester may still
  reply and reopen it.
- `closed`: finished ticket with no further customer replies expected in v1.

Expected behavior:

- customer replies on `new`, `open`, `pending_customer`, or `resolved` tickets
  move the ticket back to `open`
- agents can move a ticket to `pending_customer`, `resolved`, or `closed`
- `closed` is the terminal state for the MVP

### Chat States

- `waiting`: customer has started a chat and is waiting for an agent.
- `active`: an agent has joined and messages are flowing.
- `ended`: conversation ended without escalation.
- `escalated`: transcript was converted into a ticket.

## MVP Boundaries

Included in v1:

- tickets
- chat
- knowledge base
- one support team
- basic search and filtering
- secure customer ticket access without a full account
- chat fallback to ticket creation when no agent is available

Deferred beyond v1:

- email ingestion and reply-by-email support
- AI suggested replies
- macros and workflow automation
- SLA policies and business-hours rules
- multi-brand or multi-tenant support
- phone and SMS channels

## Realization Guidance

Realizations should prioritize a coherent service loop over channel breadth:
customers can ask for help, agents can respond, and requesters can securely
continue the conversation without ambiguity.

Technology choice belongs in the realization approach documents. Whatever stack
is used, the first build should make ticket lifecycle, chat fallback, and
knowledge base publishing easy to inspect and validate.
