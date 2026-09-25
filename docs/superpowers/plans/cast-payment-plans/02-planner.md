# Payment plans: bounded source solver and dormant cast offers

Suggested issue ID: `payplan-02-planner`
Priority: 2
Kind: `payment-plan`
Depends-On: payplan-01-contract

Implement slice 02 of
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`, after its dependency
lands. Read sections 2-5 and the entire PP acceptance table. Build an exact
executable witness for the supported subset, with one deterministic recommendation
per cast. Keep publication dormant until payplan-04.

## Work

1. Implement semantic eligibility for the specified ordinary hand casts and
   simple tap sources. Reuse authoritative timing, targets, cost modifiers,
   activation eligibility, sickness/haste and payment-window restrictions.
2. Audit windowManaUnits/unlessManaReachable/resolveManaWith for reuse. Capture a
   complete successful source assignment, rather than returning their boolean
   answer or using a summed AvailableMana bound as proof.
3. Represent exclusive ability/color alternatives per source, fixed multiple
   output and current pool. Integrate the existing pool payment solver, including
   its provenance accounting or an explicit unsupported result for unmodelled
   metadata. Never flatten restricted/snow/typed/persistent units into ordinary
   mana.
4. Implement bounded deterministic search and the exact V1 ranking from section
   5. Backtrack for dual-source assignments. Expose ready/unsupported/insufficient/
   search-limit outcomes and diagnostic node counts without events or clock reads.
5. Build PaymentActions using shared cast candidate discovery with exact-plan
   admission. Preserve Options verbatim; set BaseOptionIndex only for the exact
   matching base cast. Support funded casts absent from Options. Freeze versioned
   identity and witness generation using slice 01's codec.
6. Add an independent witness legality check for the executor. No state mutation,
   RNG advancement, observer callbacks or public publication happens in this slice.

## Acceptance

Implement PP-01 through PP-10 at the planner/offer-builder layer. Required concrete
cases include Island/Swamp/Mountain versus Badlands ranking, one dual falsely
appearing to pay two pips, an assignment needing backtracking, true C versus
generic, pool-only payment, multi-output surplus, simple rock/dork, fixed taxes,
and all specified eligibility exclusions. A global mana modifier must not turn
a nominal bare tap into a falsely guaranteed production.

Assert repeated queries leave the state, event/head/intent counts, pending Seq
and RNG unchanged. Exercise deterministic budget exhaustion with and without a
complete plan already found. No partial plan can escape. Existing Options,
legacy bot behavior and TestHeads must remain unchanged.

Report a supported/excluded-shape table and fixed-board node counts/offer cost;
do not claim complete Magic payment or strategic optimality. Tests use authored
IR or the linked corpus, never tracked Forge script text.

Use scripts/agent-worktree.sh, explicit staging and normal gates/landing. A
missing .cards corpus blocks meaningful corpus verification. Do not grow the
AGENTS.md approximation register.
