# Report — Dismantle loses its post-destruction counter condition

Ticket `agent-20260922T215327Z-0900a39d`. Branch `wt/agent-20260922T215327Z-0900a39d`.

## What changed and why

Dismantle's `Destroy` moves its targeted artifact to the graveyard before the
chained `DBPutCounter` evaluates `ConditionDefined$ Targeted | ConditionPresent$
Card.HasCounters` and sizes `CounterNum$ X` from `X:Targeted$CardCounters.ALL`.
The Move fold clears the live object's counters, so both reads saw zero and the
placement was skipped.

The chosen fix is **narrowly scoped targeted counter LKI**, mirroring the
existing `Ctx.TargetControllerLKI` mechanism exactly (CR 608.2b/h look-back,
captured at the start of resolution because a target may leave at any point):

- **`effects/registry.go`** — new `Ctx.TargetCountersLKI
  map[state.ObjID][]state.Counter`, captured in the same `effects.Resolve`
  entry block as `TargetControllerLKI` ("before the first effect can move a
  target") for every object target that is on the battlefield and carries at
  least one counter. Only battlefield objects with counters are captured; a
  map entry is exactly "the counters this target had when resolution began".
  Also new `CloneTargetCountersLKI` (deep copy, inner slices included).
- **`effects/counters.go`** — new shared helper `targetCountersLKI(c, id, o)`:
  returns `(nil, false)` while `o` is still a battlefield permanent (live
  counters authoritative, existing behaviour untouched); once the object has
  left, returns the snapshot counters. A missing entry fails closed to the
  live read.
- **`effects/conditions.go`** — in `conditionMet`'s `Targeted` member loop,
  substitute a shallow object copy carrying the snapshot counters before
  `MatchesObjectCtx`, so `Card.HasCounters` reads the look-back value. Only the
  counter field is substituted; every other characteristic stays live (or the
  existing `Ctx.LKI` trigger snapshot).
- **`effects/count.go`** — in `evalRefProperty`'s `CardCounters.` case,
  substitute the same snapshot for the object target, so
  `Targeted$CardCounters.ALL` (and `.P1P1` etc.) sizes the placement. The
  trigger snapshot (`lki`) is checked first and stays authoritative.
- **`rules/resolution.go`, `rules/clone.go`** — carry the snapshot across a
  mid-resolution suspension, exactly as `TargetControllerLKI` is carried: a
  new `resumePoint.targetCountersLKI` captured from the live chain Ctx
  (`resolutionTargetCounters(e.resolutionCtx)`), restored into the rebuilt Ctx
  in `resumeResolution`, inherited by continuation frames, and cloned with the
  frame in `clonePendingFrames`/`clonePendingTriggers`.

No unrelated `ConditionDefined$` family was changed, no predicate was widened,
and no card-name branch exists.

### Structural coverage of the class
The snapshot is taken for **every** object target at resolution start and the
read helper applies whenever that target is no longer on the battlefield, so
any later mover — destroy, sacrifice, exile, bounce, fight — is covered, not
just `effDestroy`. The gate and the amount read share the one
`targetCountersLKI` helper, so they cannot disagree.

## Gates run (real output)

`gofmt -l <changed Go files>` — empty:
```
(no output)
```

`go run ./cmd/gentypes -check` — exit 0, no output.

`go test ./internal/archtest/` — no allowlist edits:
```
ok  	github.com/adams-shaun/gorge/internal/archtest	2.304s
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`:
```
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.226s
```
The split did NOT move (no deck exercises Dismantle — see below), so no re-pin.

Exact targeted command from the brief (with the added regression name; the
regex `TestDismantle.*Counter` already matches it):
```
$ go test -run 'TestConditionGateTargetedGroup|TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient|TestDismantle.*Counter' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.628s
```

`-v` confirmation that both real-card tests RAN (not skipped; `.cards` symlink
was present in this worktree from the start):
```
=== RUN   TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient
--- PASS: TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient (0.69s)
=== RUN   TestDismantleDestroysCounteredTargetAndPlacesCounters
--- PASS: TestDismantleDestroysCounteredTargetAndPlacesCounters (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.726s
```

New effects unit test:
```
$ go test -run 'TestConditionGateTargetedCountersUseLKI' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.002s
```

## Fails without the fix

Fix reversed with `git apply -R .ds4/scratch/fix.patch` (all six non-test
files), the new regression run, then the patch re-applied and each file
`cmp`-verified byte-identical to a `.ds4/scratch` copy (RESTORED_IDENTICAL).

```
$ go test -run 'TestDismantleDestroysCounteredTargetAndPlacesCounters' ./rules/
--- FAIL: TestDismantleDestroysCounteredTargetAndPlacesCounters (0.63s)
    dismantle_lki_test.go:63: never reached a non-priority decision within 30 passes
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.646s
```
Behavioural failure, not a compile error: with the gate unmet, `DBPutCounter`
is skipped, no counter-kind ask is posed, and the stack empties to a priority
decision. The test's own preconditions (target on the battlefield with exactly
two P1P1; recipient on the battlefield with zero P1P1/CHARGE; target in the
graveyard with its live counters cleared before the kind answer) are asserted
before the behaviour under test.

The effects unit test also fails without the fix (it binds
`Ctx.TargetCountersLKI`, which the revert removes).

## Brief premise checks (re-measured)
- Corpus census `ConditionDefined$ Targeted | ConditionPresent$ Card.HasCounters`:
  `1` — the single match is `d/dismantle.txt`. Brief held.
- No repo deck mentions Dismantle (`grep -ril dismantle internal/testutil/decks/`
  → none), consistent with the botbench split not moving.
- No Known-approximations row is specifically about this shape; nothing was
  deleted, nothing was added, and `knownApproximationRows` is untouched.

## Deviations from the brief
None. The brief offered "targeted LKI support shared by the relevant
condition/count reads" or a narrow ordering fix; this is the former, sharing
one helper between the condition gate and the amount read.

## Issues (found, not fixed — out of this brief's scope)
- **Only counters are looked back up for a departed target.** `Ctx.TargetCountersLKI`
  substitutes the counter field alone, so a chained sub that reads a departed
  target's *other* battlefield characteristics (P/T, mana value, colours,
  types) through a `Targeted$` ref still reads the live graveyard object (or
  the printed face). Dismantle needs only counters, and widening the snapshot
  to a full `*state.Object` LKI copy is a separate, larger change. Corpus
  shapes that would need it: `Targeted$CardPower` / `Targeted$CardToughness` /
  `Targeted$CardManaCost` after a `Destroy`+`SubAbility$` chain. This is not in
  the frozen table and is not a new row; noting it here per the report contract.
  A CR-lane test citing CR 608.2b/h could name the departed-target P/T read.
- **`effects/conditions.go`'s `ConditionNotPresent$ Targeted` path** still
  enumerates targets live (`conditionNotPresentMet`), so a departed target with
  counters would not affect a `NotPresent$ Card.HasCounters` gate. No corpus
  line carries that shape (the census found only Dismantle's positive form), so
  this is latent, not a live defect.
