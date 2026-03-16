# Notes

Current evaluation notes:

1. The existing root documents are useful concept inputs but were not yet in
   the standard seed structure.
2. The main unresolved implementation risk is event identity:
   recurrence, timezones, duplicates, and upstream edits all need durable hub
   semantics before adapter breadth expands.
3. The safest first adapter pair is Google Calendar plus ICS because it proves
   both authenticated and feed-style ingestion without forcing CalDAV
   complexity on day one.
4. The two realization variants should share the same core command and
   projection vocabulary even if their entry screens differ sharply.
