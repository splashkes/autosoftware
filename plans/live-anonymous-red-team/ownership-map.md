# Finding-To-Code Ownership Map

These owner keys are referenced by the live scenario manifest and should be
included in each wave report.

| Owner key | Likely subsystem | Primary repo paths |
| --- | --- | --- |
| `boot-surface` | Boot shell, homepage, public shell assets | `kernel/cmd/webd/main.go`, `kernel/cmd/webd/bootloader.go` |
| `public-api-surface` | Public contracts and registry APIs | `kernel/internal/http/json/contracts.go`, `kernel/internal/http/json/registry.go`, `kernel/cmd/apid/main.go` |
| `csrf-browser-writes` | Same-origin and browser write enforcement | `kernel/internal/http/server/security.go`, `kernel/internal/http/server/request_context.go` |
| `feedback-loop-public-post` | Public incident ingest path and feedback collection | `kernel/internal/http/json/feedback_loop.go`, `kernel/internal/feedback_loop/reporter.go`, `kernel/cmd/webd/main.go` |
| `boot-execution-public` | Anonymous boot execution surface and preview routing | `kernel/internal/http/json/execution.go`, `kernel/cmd/webd/bootloader.go`, `kernel/cmd/webd/main.go` |
| `mounted-routing-rewrite` | Mounted realization proxying, cookie rewriting, and route prefix logic | `kernel/cmd/webd/main.go` |
| `registry-permalinks` | `/reg/{hash}` and `/r/{hash}` resolution paths | `kernel/cmd/webd/main.go`, `kernel/internal/runtime/registry_hash_index.go` |
| `rate-limits` | Anonymous rate limiting and protection envelopes | `kernel/internal/http/server/rate_limit.go`, `kernel/cmd/webd/main.go`, `kernel/cmd/apid/main.go` |
| `growth-surface` | Growth and partial rendering paths on the public shell | `kernel/internal/http/json/growth.go`, `kernel/cmd/webd/main.go`, `kernel/internal/realizations/growth.go` |
| `remote-registry-dependency` | Remote registry dependency and degraded-host handling | `kernel/internal/materializer/service.go`, `kernel/cmd/webd/main.go` |
