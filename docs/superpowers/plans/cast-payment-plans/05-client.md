# Payment plans: per-action client preference and controls

Suggested issue ID: `payplan-05-client`
Priority: 2
Kind: `payment-plan`
Depends-On: payplan-04-host

Implement slice 05 of
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`. Section 8 is the product
contract. A player can change whether casts use suggestions during their turn;
the engine observes only the selected per-action witness.

## Work

1. Add the seated-view Auto-pay mana preference, default off, scoped to one seat
   and match. Keep it across decisions in that view and reset for another match
   or seat. Toggling sends no intent and never casts/passes/taps by itself.
2. Group PaymentActions with their BaseOptionIndex when present and display
   planned-only casts appropriately when not. Keep one action group per cast.
   Show a source/production summary before submission; do not expose internal
   hashes or planner diagnostics as routine product controls.
3. With the preference on, a normal cast click uses the first plan. Provide
   Cast with suggested mana while off, and Pay manually for an ordinary cast
   while on. When a base cast is absent, explain/use the existing manual tapping
   controls rather than inventing a legacy index. A fixture with several plans
   must allow exact plan selection by ID.
4. Serialize the exact offered PaymentSelection with empty Choices and no Rest.
   Manual submissions remain unchanged. The toggle cannot alter an already
   submitted selection. Do not keep a plan after decision Seq changes.
5. Integrate stale-adoption, double-click/request-in-flight guards, hold-priority
   and auto-pass. An enabled playable planned-only cast prevents an empty-window
   auto-pass. Preference-off manual behavior remains as before.
6. Display the actual manual decision and a concise explanation when the engine
   supplies PaymentFallback. Spectators must have neither offers nor controls.

## Acceptance

Cover PP-18/PP-19 plus client portions of PP-02/PP-17. Test on/off/on during one
turn, zero network calls from toggling alone, the exact submitted witness,
explicit one-cast/manual overrides, grouped options, future multiple-plan list
ordering, stale replies, in-flight toggling and no duplicate submissions.
Test auto-pass suppression for a planned-only cast while enabled. Exercise the
real supported funded-cast flow through the host where the existing web harness
permits; fixture-only UI tests do not establish engine payment correctness.

Run the actual package.json test/type/lint commands for changed code and the
generated-type check. Report the controls used and PP mappings. If doing a visual
check, use a task port and unique /tmp data, never demo ports or deploy-demo.

Use scripts/agent-worktree.sh with --web when appropriate. Shared node_modules
must not be overwritten. Stage explicit paths, use normal landing, and do not
add approximation rows or change bot defaults.
