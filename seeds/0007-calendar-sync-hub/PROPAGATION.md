# Propagation Architecture

Production events can propagate outward.

Destination examples:

- Google calendars
- venue calendars
- public event feeds

## Push Logic

Propagation edges track delivery state.

States:

- pending
- pushed
- failed

Retry logic should exist for failed pushes.