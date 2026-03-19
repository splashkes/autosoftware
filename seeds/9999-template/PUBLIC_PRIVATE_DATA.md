*Quick `PUBLIC_PRIVATE_DATA.md` map: top = graph inventory plus node and edge boundaries; middle = auth examples and special cases like metadata-only or attendance data; bottom = public API, runtime-only exclusions, and extension rules.*

# Public / Private Data

Copy and rewrite this file for the specific seed.
Start with the actual graph and boundary definitions for the seed, not with
authoring instructions.

The finished seed-local document should define:

- the canonical node inventory for the seed
- the important edge kinds between those nodes
- which data is `shared_metadata`
- which data is `public_payload`
- which data is `private_payload`
- which data is `runtime_only`

This document should also make clear:

- whether anonymously public data is available through authoritative APIs as
  well as the first-party UI
- which anonymous-client delivery protections still apply, such as global rate
  limiting or crawl-abuse controls
- whether the seed has metadata-only public registration, digest-only public
  registration, or fully public content
- which relations are public, private, mixed, or runtime-only
- which objects have immutable versions and actor provenance
- which few small things are intentionally excluded from the shared registry
- the concrete auth-split examples that matter for this seed
- which agent-facing runtime context exists only for authoring help and must
  stay outside canonical shared truth

Suggested section order:

- canonical nodes
- canonical edges
- node-by-node shared/public/private/runtime slices
- edge visibility rules
- auth-split examples
- public access rule
- runtime-only exclusions
- extension rule
- contract rule
