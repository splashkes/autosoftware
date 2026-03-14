# Acceptance

Every realization of this seed must satisfy the following:

1. An agent can create, view, assign, reply to, resolve, and close support
   tickets.
2. A customer can start a ticket from a public help surface and receives both a
   ticket reference and a secure access path.
3. A customer can securely view and reply to their own ticket without creating
   a full account, using either the signed link or the documented recovery
   flow.
4. The realization documents and implements at least the following ticket
   states: `new`, `open`, `pending_customer`, `resolved`, and `closed`.
5. A customer can start a live chat session and an agent can reply from an
   internal support workspace.
6. If no agent is available, the chat flow falls back to ticket creation
   without discarding the customer's existing messages.
7. A chat transcript can be escalated into a ticket without losing message
   history or customer identity.
8. An admin can create, edit, draft, publish, and unpublish knowledge base
   articles.
9. A public visitor can search or browse the knowledge base and open article
   detail pages.
10. The realization distinguishes basic transactional notifications from
    deferred mailbox-sync, omnichannel, or AI features it does not implement.
