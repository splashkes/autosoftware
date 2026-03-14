# Change Sets

Draft placeholder for accepted mutation envelopes and atomic append units.

Working model:

- an accepted realization emits one or more change sets
- the registry appends change sets, not seeds
- replay and materialization operate on accepted change sets and rows
