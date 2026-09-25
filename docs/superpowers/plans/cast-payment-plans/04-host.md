# Payment plans: live offers, host admission and recovery

Suggested issue ID: `payplan-04-host`
Priority: 2
Kind: `payment-plan`
Depends-On: payplan-03-execution

Implement slice 04 of
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`, especially sections
2, 4, 6-7. This slice enables publication only after the previous slices can
execute and replay a selected plan.

## Work

1. Attach the deterministic PaymentActions extension to real pending priority
   decisions. Keep legacy Options and indices unchanged. Direct rules consumers,
   host consumers and replays must reconstruct the same extension without a
   special game configuration or a mutating read API.
2. Carry the extension and PaymentFallback through seat view, Pending, HTTP and
   stream decision paths. Audit deep copies in projection and human-seat parking.
   Preserve private-decision redaction for opponents/public spectators.
3. Extend parked-seat validation so a malformed/stale/forged planned request is
   rejected before acknowledgment and cannot crash the match on later Submit.
   Keep all engine mutations on the match goroutine and obey lock ordering.
4. Preserve the full witness in persisted intents, feedback snapshots and their
   loader/repro path. Prove restart, undo, ViewAt/ReplayTo and resumed target asks
   need no live-only cache or original browser toggle.
5. Expose the schema through existing Go/TypeScript generation. Preserve the
   current wire version for additive compatible fields unless actual repository
   compatibility checks demonstrate a necessary version change; document one if
   needed. No new endpoint or TableConfig auto-payment flag is required.

## Acceptance

Close host portions of PP-15 through PP-17, plus live-offer PP-01/PP-02/PP-10.
Use a real table and parked seat through Pending/SubmitIntent/HTTP, not only
direct Engine.Submit. Assert wrong table/seat credentials remain forbidden,
unknown/stale payment selectors return errors without advancing, and invalid
user input leaves the table live and still answerable.

Capture a planned targeted cast, restart/undo/replay it, and use feedback loading
to reproduce the same head. New data must survive caller-copy mutation tests.
An opponent's view/events/errors must not disclose a hand-card payment offer.
Legacy hosts/seats ignore the extension and retain existing intent/golden behavior.

Run affected host/httpapi/view/replay/repro tests and manual TestHeads with the
real corpus. Report PP mappings and exact persistence paths audited. Publication
is complete only when both direct and hosted consumers can use the selector.

Use scripts/agent-worktree.sh and normal gates/landing. If a server is needed,
choose an allocated 8090-8099 port and unique /tmp persistence. Never deploy the
demo or touch a peer's working files. No approximation-register growth.
