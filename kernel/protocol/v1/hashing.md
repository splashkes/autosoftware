# Hashing

Registry permalinks use SHA-256 content hashes.

Current contract:

- `content_hash` is the lowercase hex SHA-256 digest for the canonical JSON
  representation of the resource exposed by the registry API
- `permalink_url` is derived as `/reg/<content_hash>`
- `canonical_url` names the stable semantic browse location for the latest
  active form of the resource
- `permalink_url` names an immutable browse location for exactly one hashed
  representation of that resource
- `/reg` is a reserved registry namespace
- `/@<content_hash><canonical_url>` remains the hash-validated browse
  route that `permalink_url` resolves into

Implications:

- two byte-identical canonical resource payloads produce the same
  `content_hash`
- a changed canonical payload must produce a different `content_hash` and a
  different `permalink_url`
- preview execution routes such as `/__runs/<execution-id>/...` are excluded
  from the hashing contract and are never canonical inputs

Validation requirements:

- registry API payloads that advertise `canonical_url`, `permalink_url`, and
  `content_hash` must keep them mutually consistent
- `/reg/<content_hash>` must resolve through the runtime hash index rather than
  by scanning registry graph content
- human-facing permalink routes must reject mismatched hashes rather than
  silently falling back to the canonical route
