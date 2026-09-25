# Payment plans: integrated acceptance and release evidence

Suggested issue ID: `payplan-06-acceptance`
Priority: 2
Kind: `payment-plan`
Depends-On: payplan-05-client

Close the feature defined by
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`. Read the entire spec
and prior ticket reports. This ticket owns PP-20/PP-21 and any missing integration
evidence across the PP-01..PP-21 matrix. Do not declare success from legacy games
that never select a plan.

## Work

1. Add a focused test Seat using only View/Decision that selects offered plans
   through normal host submission. Run deterministic two-player and multiplayer
   fixture games with actual planned casts; all must finish naturally and replay.
   Include a targeted cast suspended across its target decision and a manual
   cast after a planned one in the same turn.
2. Demonstrate a legacy Seat ignoring PaymentActions has identical legacy
   options/intents/results. Compare planned execution with manual execution of
   its identical payment choices at equivalent rules boundaries; distinguish
   state equivalence from expected differences in decision-event bookkeeping.
3. Complete the restart/undo/feedback/clone/privacy coverage and PP-to-test matrix.
   Fix missing behavior within this spec rather than weakening acceptance or
   expanding the known-approximations register.
4. Run focused new tests, current manual goldens, repo-deck ratchets, conformance,
   make sim, generated-type verification and required repository gates. Verify
   .cards exists and no corpus-dependent test was silently skipped.
5. Measure fixed-board planning overhead, node counts, complete-plan hit rate,
   unsupported/limit reasons, external decisions and naturally completed games.
   Record exact commands, seeds, source/corpus pins and workload. No unqualified
   policy-strength or cross-engine speedup claim.
6. Write `docs/superpowers/reports/2026-09-24-cast-payment-plans.md` with all
   evidence, supported/excluded shapes and follow-up scope. Source date may be
   updated if implementation lands later; keep the spec link accurate.

## Acceptance

Every PP row names a passing automated test or the explicit command covering its
gate; identify each new test actually run. The report includes at least one
planned cast through external seat/HTTP admission, an empty-pool funded offer,
mixed manual/planned behavior, exact selected-source execution, replay/recovery
and adversarial rejection evidence. The browser can change preference mid-turn
without any game setting mutation.

No manual golden head is updated. No full-card support claim is inferred from
the presence of a plan. Unsupported automation keeps manual play available.
If evidence is incomplete, report the precise criterion and fix required;
do not mark the feature complete merely because unit packages print ok.

Use scripts/agent-worktree.sh, explicit staging and normal gates/landing. Keep
fixtures free of Forge scripts and do not bind/redeploy the live demo.
