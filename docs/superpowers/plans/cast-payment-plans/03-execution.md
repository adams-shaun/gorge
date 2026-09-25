# Payment plans: cast continuation, execution and replay

Suggested issue ID: `payplan-03-execution`
Priority: 2
Kind: `payment-plan`
Depends-On: payplan-02-planner

Implement slice 03 of
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`, especially sections
4, 6 and 7. Use the merged V1 witness and pure planner. Live publication remains
off until the host integration in payplan-04; focused tests may attach real
planner offers to the actual pending decision through a test-only helper.

## Work

1. Extend Submit to admit the exclusive payment selector with current actor/Seq,
   offer membership and full rules-side witness legality checks BEFORE recording
   the intent or clearing pending. Handle planned-only casts without inventing a
   legacy option index or accepting an unvalidated client cast descriptor.
2. Enter beginCast through the authoritative normal-cast path. Store an immutable
   witness on pendingCast, copy it in Clone, and retain it across target asks.
3. At the ordinary payment stage, revalidate final cost and all remaining sources;
   execute the exact selected abilities/color choices through existing mana
   machinery. Pay through the authoritative spend/provenance path. Do not emit
   nominal mana directly or write Game fields, and do not create synthetic
   public tap-choice intents.
4. Implement section 6's fallback/interruption semantics. No silent substitution
   or partial nominal success. Preserve completed legal effects and use the
   existing cast reversal where the proposed cast itself becomes illegal.
5. Record the full witness in Intent and canonical action/plan ID suffix in
   DecisionMade.Text for planned submissions only. Pin the suffix bytes and
   preserve every legacy event encoding and event ordinal.
6. Exercise ordinary Replay/ReplayTo and clone/resume without the original live
   cache or any UI setting. Replay validates the selected V1 witness; it never
   asks for the current first recommendation.

## Acceptance

Cover engine-level PP-11 through PP-16. A real targeted spell must pose its
normal target ask before any planned tap, then finish the chosen plan without
redundant tap/color asks. Assert exact source set, production, spend, surplus,
activation provenance and normal priority/stack behavior.

Rejections must preserve state, pending decision, RNG, log/head and intent count.
Test a post-offer changed cost/source before activation and an activation that
unexpectedly suspends; neither case may substitute a source. Test witness
tampering and mutation of submitted/copy payloads. Compare planned execution
with manual execution of the SAME sources/choices at equivalent rule boundaries;
their decision bookkeeping differs, so do not require their event hashes to match.

Use the full corpus for TestHeads/manual regression and focused conformance
tests. Keep the new tests discoverable with Test.*PaymentPlan. Report each PP
ID, commands/results, chosen event suffix, and fallback behavior.

Use scripts/agent-worktree.sh, explicit staging and normal landing. Do not
publish offers before host admission and privacy are ready; do not grow AGENTS.md.
