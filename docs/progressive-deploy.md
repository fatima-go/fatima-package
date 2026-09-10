# Progressive deployment v2

All related work uses `feature/progressive-deploy`.

구현 사용법과 실제 검증 결과: [신규 배포 기능 사용 및 검증](progressive-deploy-usage.md).

## Compatibility boundary

Existing HTTP routes, JSON contracts and business handlers retain their behavior.
Only listener initialization/shutdown is extended to multiplex HTTP/1.1 and native
gRPC on the existing configured port (normally Jupiter 9190, Juno 9180). A new
read-only capability path advertises v2. HTTP/2 alone is not a feature marker.
Network/authentication failures are not evidence of a legacy server. No fallback
may replay a deployment after a new mutation has been submitted.

Legacy clients continue to use HTTP. New deployment clients choose legacy before
submission when Jupiter or any selected Juno lacks the required API. Artifact
selection/resume have no legacy equivalent. Legacy operations are not restricted
by v2 locks; v2 guarantees apply to v2 plans, with drift checked at execution.

## Ownership

Jupiter owns immutable uploaded FARs and durable rollout plans. A plan fixes the
artifact digest, package IDs and order. It deploys one package, waits for an
explicit continue action, and then deploys the remainder serially, stopping on
failure or an uncertain result. Juno owns each actual deployment operation and
its durable progress/result. TUI attachment is independent of execution.

Original multi-platform FARs remain unchanged. Build author, uploader and deployer
are distinct metadata. Artifacts expire exactly 24 hours after upload completion;
expiry is never extended by reads or rollout references. Expired bytes are removed,
metadata/results survive. No new dispatch after expiry; a fully staged Juno can
finish its local operation. New rollout/continue requires sufficient time budget.

Both stores use atomic writes and interprocess locks. Multiple Jupiter processes
must point to the same shared POSIX storage with working flock/atomic rename. Each
rollout has a renewable execution lease; stable operation IDs allow recovery
without blindly re-running commands. Juno restart interrupts in-flight operations
for explicit inspection, never reports them successful or replays automatically.

## Local delivery and verification

The development builder consumes sibling working trees (including uncommitted
changes) and local core/proto dependencies, creates the normal platform tarballs,
and records source identities. No push or artifact HTTP server is required.
The local installer uses the authorized FATIMA_HOME, can stop/restart OPM, and
preserves configurations for update tests. Native Mac is the primary test target;
Linux-specific monitoring is recorded separately until executed on Linux.

Verification includes old/new client/server combinations, same-port coexistence,
TUI upload/list/detail/rollout/watch, three-Juno serial execution, interruption,
restart, duplicate request, artifact corruption/expiry, and installation from the
actual generated tarball. No external Slack notifications during tests.
