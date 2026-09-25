# Payment plans: protocol, identity and ownership contract

Suggested issue ID: `payplan-01-contract`
Priority: 2
Kind: `payment-plan`

Implement slice 01 of
`docs/superpowers/specs/2026-09-24-cast-payment-plans.md`. Read the complete spec;
sections 2, 4 and 7 govern this slice. The outcome is additive data types and
validation that can represent a cast with an exact payment witness while keeping
every existing option index and manual intent unchanged. Do not publish live
payment actions yet.

## Work

1. Add Decision.PaymentActions, Intent.Payment, Decision.PaymentFallback and
   the data-only types from the spec. Define the full V1 payment witness,
   printed/intrinsic ability discriminants, six-symbol mana representation and
   quantity/list bounds. Define the genesis zone-incarnation sentinel.
2. Implement canonical map-free encoding and domain-separated full SHA-256 IDs.
   IDs bind version, Seq, player, cast and complete witness; exclude display text,
   preferred rank and BaseOptionIndex. Document and pin byte vectors.
3. Add exclusive-selector validation before the ordinary Choices bounds:
   payment requires priority, matching actor/Seq, empty Choices/Rest and exact
   membership/witness equality. Reject unknown versions and malformed/bounded
   payloads. Keep legacy validation byte/behavior compatible.
4. Add a reusable deep-copy helper for the new types and audit ownership sites
   in engine clones, view copies, host pending/parking and intent admission. Make
   dormant callers safe now; no new rules dependency in decision/view/protocol.
5. Regenerate TypeScript through cmd/gentypes. Tests must demonstrate omission
   of absent optional fields preserves existing JSON and no new decision.Kind
   or event ordinal is introduced.

## Acceptance

Cover the structural and copy/identity parts of PP-11, PP-15 and PP-16. At least:

* legacy valid/invalid intent cases retain results, including empty Choices;
* mixed selectors, wrong kind/player/Seq, unoffered IDs, modified witness,
  negative/overflowing quantities, oversized activations and duplicate sources
  reject in targeted tests;
* option/list reorder and label changes do not change canonical identities;
* mutating a copied plan/action cannot modify the original;
* TypeScript generation check and affected dependency-order checks pass;
* ordinary TestHeads passes with the real .cards corpus.

This slice does not claim engine-side legality or replay execution; those are
payplan-03. It must not make unsupported planned submissions reachable in live
games. Report exact type/codec decisions so later tickets need not invent them.

Use scripts/agent-worktree.sh, explicit staging and the normal landing workflow.
Never add Forge scripts or approximation rows. Completion report must name tests,
commands, mapped PP IDs and any remaining behavior owned by dependent tickets.
