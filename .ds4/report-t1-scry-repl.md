# Task report — scryrepl: `R:Event$ Scry` replacements

Ticket: `agent-20260923T135903Z-2444d628`
Commit: `ae4fed1e5c0ea0935dd18c688ea704cb303bd191`
Branch/worktree: `.worktrees/agent-20260923T135903Z-2444d628`

## What changed and why (per file)

The scry instruction is now proposed to the replacement machinery at the
**instruction boundary**, before any library card is inspected (CR 614.4). A
body can rewrite the proposed count in place (Kenessos: Num+1) or replace the
whole instruction (Eligeth: draw that many instead). The completed
`events.Scry` record — the bottom-card marker `trig:Scry` reads — is still
emitted afterwards, carrying the number of cards actually put on the bottom,
and is deliberately kept **out** of the replacement pass (so Kenessos cannot
be applied to its own result a second time). This preserves the existing
`ToBottom$` / The Temporal Anchor behaviour the reference commit `b8691254`
had to flag as a deviation.

- `effects/registry.go` — added `Scry(p, source, count) (int32, bool)` to the
  `Host` interface: the effects tier's only route to replacement matching
  (effects never imports rules). Returns the surviving count and
  `proceed=false` when the scry was replaced whole.
- `effects/context_test.go` — `fakeHost.Scry` returns `(count, true)` (no
  engine registry to consult), the same discipline as `ExploreReplaced`.
- `effects/cardflow.go` (`effLookAndArrange`) — for the `Scry` verb only, call
  `h.Scry` after the base count and `extraOf` are known and **before** the
  KArrange options are built; on `proceed=false`, skip this library entirely
  (`continue` → nothing looked at, no arrange).
- `rules/replacement.go` —
  - `replacementEvent`: map `events.Scry` → `"Scry"`.
  - `applyReplacementsDispatch`: route `events.Scry` to the new
    `continueScryReplacements` **before** the generic single-match/CR-616.1
    branch, so a proposal is never handed to the generic path.
  - `replacementMatchesRememberedUngated`: new `case "Scry"` gated on
    `ValidPlayer$` (Kenessos/Eligeth both carry `ValidPlayer$ You`).
  - New `(*Engine).Scry` (Host hook), `continueScryReplacements`
    (count rewrite / whole replacement), `scryReplacementCount` (the
    `ReplaceCount$Num` grammar, bound to the held instruction's count), and
    `emitScryRecord` (logs the completed record outside the replacement pass).
  - `init()` `RegisterNonAPI`: add `"repl:Scry"`.
- `rules/arrange.go` (`handleArrange`) — the Scry marker now goes through
  `e.emitScryRecord(...)` instead of `e.emit(...)`, so the finished action's
  record is not re-matched. The record's `Amount` is unchanged
  (`len(pileB)`, the number actually bottomed).
- `rules/coverage_test.go` — new `TestScryReplacementPrimitiveIsRegistered`
  pinning `repl:Scry` in `effects.Supported()`.
- `rules/scry_replacement_test.go` (new) — the focused tests.

## Gates — exact commands and real output

`gofmt -l` on every changed Go file (empty), then `gentypes -check` (empty):

```
== gofmt ==
== gentypes ==
```

`go test ./internal/archtest/`:

```
ok  	github.com/adams-shaun/gorge/internal/archtest	3.452s
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` (split
did NOT move — no re-pin needed):

```
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.424s
```

Targeted rules command (brief's expected form, with the verified registry test
name `TestScryReplacementPrimitiveIsRegistered`; I also added the marker-guard
test to the same run):

```
$ go test -run 'TestKenessosReplacesScryCountBeforeLooking|TestKenessosDoesNotReplaceTheCompletedScryRecord|TestEligethDrawsInsteadOfScrying|TestScryReplacementPrimitiveIsRegistered' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

Regression checks run once (all green): `TestScryPosesArrangeAndSuspends`,
`TestScryRestOrdersTheBottomPile`, `TestTemporalAnchorScryBottomTrigger`,
`TestParamCensusScanIsComplete`, `TestEveryRepoDeckParamsAreRead`,
`TestEveryRepoDeckIsFullySupported`, and in `effects/`
`TestPrimitivesAreRegistered|TestScry|TestSurveil|TestScryOptional`:

```
ok  	github.com/adams-shaun/gorge/rules	0.887s
ok  	github.com/adams-shaun/gorge/rules	0.762s     (ratchets)
ok  	github.com/adams-shaun/gorge/effects	0.007s
```

Corpus presence: `.cards` was already a valid symlink in this worktree
(`.cards -> /home/sadams/projects/gorge/.cards`), so the corpus-backed tests
actually ran (not the vacuous skip path).

## Fails without the fix

I copied `rules/replacement.go` to `.ds4/scratch/replacement.go.bak`, reverted
the four production hunks (the `events.Scry` dispatch case, the
`replacementEvent` case, the matcher `case "Scry"`, and the `repl:Scry`
registration), ran the targeted command, then restored the file and verified
`cmp` byte-identity (`restored byte-identical`). Real failing output:

```
--- FAIL: TestScryReplacementPrimitiveIsRegistered (0.00s)
    coverage_test.go:109: effects.Supported() is missing "repl:Scry"
--- FAIL: TestKenessosReplacesScryCountBeforeLooking (0.60s)
    scry_replacement_test.go:62: Kenessos Scry 3 arranged 3 options (max 3), want 4
--- FAIL: TestKenessosDoesNotReplaceTheCompletedScryRecord (0.00s)
    scry_replacement_test.go:83: precondition: Kenessos did not widen the window: Max = 3, want 4 (a base Scry 3 look)
--- FAIL: TestEligethDrawsInsteadOfScrying (0.00s)
    scry_replacement_test.go:116: non-priority decision &{... Kind:arrange ... Max:3 ...} while draining the stack
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.652s
FAIL
```

Every new test asserts a precondition (Kenessos/Eligeth's `R:Event$ Scry` is
live on the battlefield; the library holds enough cards; the base window
differs from the replaced window) and each fails when the production fix is
reverted. The marker test's reverted failure lands on its precondition
(without the fix the window is never widened), which is the intended loud
failure rather than a silent pass.

## Head/ratchet movement

None. `rules/heads_test.go` untouched. `TestEveryRepoDeckIsFullySupported` and
`TestEveryRepoDeckParamsAreRead` both pass: no repo deck carries
`R:Event$ Scry` (`grep -rln 'R:Event\$ Scry|Kenessos|Eligeth'
internal/testutil/decks/` → no files), so `knownUnsupported` /
`knownUnsupportedParams` are unaffected. `TestParamCensusScanIsComplete`
passes with the new reads. Botbench split unmoved.

## Deviations from the brief

- The brief's workspace facts state "Current HEAD is `c947f5c8`". Measured
  HEAD at start was `7b4ab59e4c0ed1c4f4c9c330797ec2a454e6e0fe`. I implemented
  against the actual HEAD. (The brief elsewhere says to re-measure brief
  claims; this one held only in that the referenced commit `b8691254` is not
  an ancestor.)
- The brief suggested the reference commit's `Host.Scry` approach. I kept it
  but changed its semantics: the reference emitted the `events.Scry` marker at
  the proposal (pre-look) count and removed the `handleArrange` marker, which
  breaks `ToBottom$` / The Temporal Anchor (the reference commit itself flags
  this). Mine keeps the marker at `handleArrange` with the bottom count and
  bypasses the replacement pass for it, so existing marker semantics are
  preserved. This is strictly better for the brief's "preserve the correct
  final Scry marker/trigger behavior".
- The brief said "Do not change or add a Known-approximations row." I neither
  added nor changed one; the remaining CR 616.1 ordering deviation is in the
  commit message, this report, and a new ticket.

## Issues

1. **CR 616.1 ordering among competing `R:Event$ Scry` replacements is not
   implemented.** `continueScryReplacements` applies collected matches in
   deterministic scan order (the same stand-in the CreateToken and Explore
   classes document). With both Kenessos and Eligeth active, a count rewrite
   reached first is folded into the `Draw` replacement's count; a `Draw`
   reached first consumes the instruction and the rewrite never runs. CR 616.1
   makes the order the affected player's choice. Measured prevalence: 2 files
   carry `R:Event$ Scry` (`/usr/bin/grep -rlE 'R:Event\$ Scry'
   .cards/cardsfolder | wc -l` = 2); neither is in the repo decks. Filed as
   `.ds4/new-tickets/scry-replacement-cr616-order.md`.

2. **CR-lane visibility.** A focused CR-lane test citing **CR 616.1** would
   stop this ordering gap being invisible to the ledger. Not written (out of
   scope for this ticket).

3. **Minor:** the whole-instruction replacement reuses `lifeReplacementDraw`
   (named for the GainLife→Draw lane) because it is the only suspension-aware
   "draw N" helper (loops `effects.DrawFor`, parks on a Dredge ask). Functionally
   correct and shared; the name is simply narrower than the reuse. Not a defect.

No Known-approximations row was added, grown, or removed; `knownApproximationRows`
unchanged.
