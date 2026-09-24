# Report — fb-20260923T050453Z-49840c2d (Fury divided damage, fix round)

## What this round did

This is a FIX round answering `findings-t2.md` (VERDICT: REQUEST_CHANGES). The base
feature (a real player allocation ask for `DealDamage` with
`DividedAsYouChoose$`) already landed in the prior commit `8a716cd92`. This round
fixes both findings.

### MAJOR — single-target division posed an unanswerable ask (`effects/damage.go`)

Prior code gated the ask on `if len(divTargets) > 0 && total > 0`, so a division
with exactly **one** legal target posed a mandatory `Min == Max == total`,
`Repeatable` question with exactly one legal answer — a decision nobody could
answer differently. That is the invariant the sibling distribution primitives
honour (`putCounterChoose`'s strict-supersets rule, `bolster`'s
`len(cands) == 1` fast path), and it is the reporter's own repro (target answer
selected one target).

Fix: the ask now requires `len(divTargets) > 1`. With exactly one chosen target
the split is filled directly (`c.DamageSplit = []int32{total}`,
`DamageSplitDone = true`) so the positional emission loop still deals the whole
total. Zero targets / zero total remain the silent no-op.

### MINOR — `DamageSplitDone` never consumed (`effects/damage.go`)

`DamageSplitDone` was set but never cleared, so a single resolution running
`DealDamage` with `DividedAsYouChoose$` twice would silently reuse the first
call's shares against a possibly different target list. Fix: a `defer` at the
top of the divided emission walk resets `c.DamageSplit = nil` and
`c.DamageSplitDone = false` on every return path. This is latent — measured,
zero corpus cards contain two `DividedAsYouChoose` lines, and the two carriers
with repeats put the divided primitive outside the repeat — so it is named here
and in the commit message rather than closed with an unprovable synthetic test.

### New test (`rules/fury_damage_split_test.go`)

`TestFuryDamageSplitSingleTargetFillsWholeTotal` drives Fury's ETB to the target
ask, chooses **only Bear1** (Fury is also a legal target, so both are offered),
then drains the stack with a new local helper `drainStackPassing` that **fatals
on any non-priority decision**. The shared `passUntilStackEmpty` would silently
auto-answer a `damage_split` ask through its own arm, hiding the defect, so this
test uses a drain that cannot. It asserts the sole target took the whole
scripted total (4) and the game replays.

Preconditions asserted: the bear is on the battlefield (the zone `effDealDamage`
reads), its toughness is > 4 (so a full share is not lethal and cannot reset its
`Damage`), and Fury's target ask really offered the bear alongside Fury
(`len(td.Options) >= 2`), so choosing only the bear is a genuine single-recipient
division.

## Fails without the fix

Reverting the gate (`len(divTargets) > 1` -> `len(divTargets) > 0`) and running
the new test:

```
--- FAIL: TestFuryDamageSplitSingleTargetFillsWholeTotal (0.54s)
    fury_damage_split_test.go:246: unexpected mid-resolution ask while draining: &{Seq:54 Player:0 Kind:choose Prompt:Assign 4 damage Min:4 Max:4 Options:[{Index:0 Kind:card Label:Bear1 Obj:41 ...}] ... ResumeKind:damage_split ...}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.560s
```

The non-test file was copied to `.ds4/scratch/damage.go.bak`, the hunk reverted,
the test run, then restored and verified byte-identical (`cmp` clean,
`RESTORED-CLEAN`). The finding independently verified the two-share tests
(`TestFuryDividesDamageAmongTargets`, `TestFuryDamageSplitAllowsZeroShare`) fail
when the ask is gated off.

## Gates run (real output)

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	5.237s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.899s

$ go test -run 'TestFuryDividesDamageAmongTargets' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.625s

$ gofmt -l effects/damage.go rules/fury_damage_split_test.go
(no output)

$ go vet ./effects/ ./rules/
(no output)

$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	22.091s

$ go test -run 'TestFury' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.756s
```

`.cards` is present as a symlink (`ls -la .cards` -> link to
`/home/sadams/projects/gorge/.cards`), so the corpus-backed tests ran rather
than skipped (the Fury run took ~0.75 s, not a vacuous 2 ms). The botbench
split is unchanged — the gate is `ok` with the existing pinned numbers, so no
re-pin and no attribution is owed.

`TestHeads`, `make sim`, `make report`, CR conformance and `go vet ./...` are
daemon gates; not run here per the brief and the seat rules.

## Deviations

None. Scope stayed on `DealDamage` + `DividedAsYouChoose$`; no
`PutCounter`/`PreventDamage` allocation, no AGENTS.md row (none existed to
delete), no ratchet edit, `knownApproximationRows` unchanged.

## Issues

- **Latent `DamageSplitDone` reuse is unlikely but not impossible to trigger.**
  The consume-and-reset I added only guards a *single* `effDealDamage` call
  reached twice within one `Ctx` (repeat/chained subs). A card that builds two
  *separate* resolutions each carrying a divided `DealDamage` is unaffected
  (fresh `Ctx` per resolution). Measured prevalence of any such shape: 0 of the
  corpus (`/usr/bin/grep -rlE 'DB\$ DealDamage.*DividedAsYouChoose\$'
  .cards/cardsfolder | wc -l` -> 16, none with a second divided line). No
  follow-up ticket warranted today; named for the ledger.
- **The single-target ask defect had no CR-lane test.** A CR 601.2d assertion
  ("the division is announced as part of casting; each target must be assigned
  at least 1") would pin that a one-target division poses no ask. Not written —
  the brief did not ask for it. Name it for the controller if a ledger entry is
  wanted.
- No ledger entry matched this work; nothing deleted.

## Commits

- `ad15815ed` — fix(effects): fill single-target DividedAsYouChoose share without an ask
