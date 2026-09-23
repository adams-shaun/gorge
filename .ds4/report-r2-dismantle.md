# Round report — Dismantle target-counter LKI at the departure boundary (fix of findings-r2)

## Verdict on the prior finding

**[MAJOR] `effects/registry.go:2332` — entry-time snapshot applied as LKI for any later-departed target.** FIXED. The authoritative capture is now the zone-change boundary, not resolution start:

- `rules/engine.go` (`Engine.emit`): on the final, replacement-adjusted `MoveZone` with `From == ZBattlefield` (after the replacement pass, before `events.Apply`'s Move fold clears the counters), the engine calls the new `snapshotDepartingTargetCounters(ev.Obj)`.
- `rules/resolution.go` (`snapshotDepartingTargetCounters`): if the departing object is an object target of the published resolution chain (`Ctx.Targets`, else `Ctx.PickedTargets` — the exact groups `targetedGroup` and the `Targeted$CardCounters` read walk), its counters are cloned into `Ctx.TargetCountersLKI[oid]` at that instant, overwriting `effects.Resolve`'s entry capture. A target that departs with NO counters has its entry deleted (so the read fails closed to the live, already-cleared zero); a target that gains counters mid-chain and then departs gains an entry the entry capture never made.
- `effects/registry.go` / `effects/counters.go`: doc comments updated — the entry capture is now documented as the fallback for a departure the host did not see (a test host folding events without `Engine.emit`); the live read still wins while the object is on the battlefield, so a stale entry for a live target is inert (unchanged behaviour, pinned by the round-1 unit test).

The mechanism is structurally closed, not instance-patched: EVERY state mutation funnels through `Engine.emit` (the engine's single mutation entry point), so every departure path — destroy, sacrifice, exile, bounce, SBA sweep, a replacement-redirected move — crosses the same boundary hook, and any future reader of the target look-back inherits the correct pre-move counters. Non-targets are never captured: the look-back is only ever read through the chain's target groups.

## Files changed

- `rules/engine.go` — departure-boundary hook in `emit`, next to the existing pre-Apply `defenseBefore` read (both are MoveZone-from-battlefield pre-Apply reads; the hook runs after the replacement pass so it snapshots against the move that will actually apply).
- `rules/resolution.go` — new `snapshotDepartingTargetCounters` helper beside `resolutionTargetCounters`. Writes only `Ctx` scratch (the same surface `effects.Resolve`'s entry capture writes), never `e.resume`.
- `effects/registry.go`, `effects/counters.go` — comment updates only.
- `rules/dismantle_lki_test.go` — new regression `TestChainReadsTargetCountersChangedEarlierInResolution` (appended to this ticket's own round-1 file; no shared file touched).

## The new regression

A synthetic probe spell with Dismantle's exact chained shape:

```
A:SP$ PutCounter | ValidTgts$ Artifact.YouDontCtrl | CounterType$ P1P1 | CounterNum$ 1 | SubAbility$ DBDestroy
SVar:DBDestroy:DB$ Destroy | Defined$ Targeted | SubAbility$ DBPut
SVar:DBPut:DB$ PutCounter | Choices$ Artifact.YouCtrl | CounterType$ CHARGE | CounterNum$ X | ConditionDefined$ Targeted | ConditionPresent$ Card.HasCounters
SVar:X:Targeted$CardCounters.ALL
```

An opposing artifact with two P1P1 is targeted; the chain ADDS one more P1P1, destroys the target, then sizes the recipient's CHARGE placement from the look-back. Correct amount: **3**. With the fix reverted the stale entry snapshot sizes **2**.

## Fails without the fix

Reverted both non-test hunks (`rules/engine.go` hook + `rules/resolution.go` helper) from scratch copies, ran the one test, restored byte-identically (`cmp` clean):

```
--- FAIL: TestChainReadsTargetCountersChangedEarlierInResolution (0.62s)
    dismantle_lki_test.go:147: recipient CHARGE = 2, want 3 (the target's counters immediately before destruction: 2 + the chain's own 1)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.632s
```

The test's preconditions assert the target is an opponent battlefield artifact with exactly two P1P1 and the recipient is clean, and its postcondition asserts the target is really in the graveyard — the 3-vs-2 difference is entirely the look-back source.

## Gates run (real output)

- Targeted brief command (with the new test name included), after the fix and restore:

```
$ go test -run 'TestConditionGateTargetedGroup|TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient|TestDismantle.*Counter|TestChainReadsTargetCountersChangedEarlierInResolution' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.673s
```

  (Also ran `./effects/` once with the same regex: `ok github.com/adams-shaun/gorge/effects` — covers `TestConditionGateTargetedCountersUseLKI` and `TestConditionGateTargetedGroup`.)

- `go test ./internal/archtest/` → `ok github.com/adams-shaun/gorge/internal/archtest 3.073s` (no allowlist edits).
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → `ok github.com/adams-shaun/gorge/cmd/botbench 1.268s` — split did NOT move; no re-pin, no attribution needed (no repo deck exercises a counter-swing-then-destroy-on-a-targeted-artifact chain; Dismantle is not in any repo deck).
- `gofmt -l` on all five changed files → empty (clean).
- `go run ./cmd/gentypes -check` → exit 0.

Daemon-only gates (TestHeads, full rules acceptance, `make sim`, CR conformance, `go vet ./...`) deliberately not run in this seat. The departure hook only changes behaviour for a target whose counters differ between resolution start and its departure — a shape the repo decks' byte-identical bot split confirms they do not exercise.

## Break attempts (all held)

- Ordinary countered artifact destroyed by Dismantle, recipient receives the chosen counters — held (`TestDismantleDestroysCounteredTargetAndPlacesCounters`, round 1).
- Indestructible-target Dismantle kind-choice continuation — held.
- `TestConditionGateTargetedGroup` (Targeted condition channel) and `TestConditionGateTargetedCountersUseLKI` — held; the unit test's "live countered target" arm proves live counters still take precedence over any snapshot entry.

## Issues

- No new defects found. The entry-capture fallback remains for hosts that fold events without `Engine.emit` (effects unit-test hosts); it is documented as such and is inert whenever the engine's own emit runs, because the departure overwrite always lands at the true CR 608.2b/h boundary. If a future reader needs the look-back for a group that is NOT a Ctx target (e.g. a `Remembered$` counter read over a departed object), that is a separate, unmeasured gap — out of this brief's scope.
- Nothing here touches the Known-approximations register (no row added, grown, or closed; `knownApproximationRows` unchanged).
