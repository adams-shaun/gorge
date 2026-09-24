# Report — Fury damage distribution: player-chosen shares (fb-20260923T050453Z-49840c2d)

## What changed and why

`DealDamage` carrying `DividedAsYouChoose$ N` silently round-robined the named
total. The player now answers a real mid-resolution allocation decision.

- **`effects/damage.go`** — `effDealDamage`'s `divided` branch now builds the
  chosen-target list (`divTargets`, in `Defined$` order) ONCE, BEFORE opening the
  damage batch. If the split has not been answered yet (`!c.DamageSplitDone`) and
  there is something to divide (`len(divTargets) > 0 && total > 0`) it poses a
  `KChoose` ask: one option per chosen target (`Kind` "card"/"player", carrying
  `Obj`/`Player`), `Min == Max == total`, `Repeatable: true`,
  `ResumeKind: "damage_split"`, `ResumeSA: sa`. A winner is a multiset of option
  indices whose per-target multiplicities are the shares, so a zero share is a
  target the answer never picks. `AskAsked` suspends before the batch opens
  (nothing half-emitted); the R-9 no-host / `AskEmpty` path computes the old
  round-robin split via the new `roundRobinSplit` helper and marks
  `DamageSplitDone`. Emission reads `c.DamageSplit[i]` positionally (missing
  trailing entries read as zero). The batch/`emitFromEachSource` path is
  untouched.
- **`effects/registry.go`** — two `Ctx` transport fields, `DamageSplit []int32`
  and `DamageSplitDone bool` (own transport, like `TargetsPick`, so another
  KChoose primitive under the same SA cannot steal the answer). fx42-style
  consume-and-done: the fresh resume `Ctx` is thrown away when the resolution
  ends.
- **`rules/resolution.go`** — a new `"damage_split"` resume arm in
  `resumeResolution`'s `switch rp.kind`: it turns the answered option multiset
  into `ctx.DamageSplit` (one slot per option index, `Index` == the target's
  `Defined$` position) and sets `DamageSplitDone`, then the ordinary re-entry
  calls `effDealDamage` with the player's shares.
- **`rules/rakdos_muscle_deck_test.go`** — the old round-robin
  `TestFuryDividesDamageAmongTargets` was removed (replaced, per the brief's
  "replace/update its oracle").
- **`rules/fury_damage_split_test.go`** (new) — the replacement integration
  tests (see below).
- **`rules/replacement_updated_test.go`** — `passUntilStackEmpty` (the shared
  drain helper) gains a `damage_split` arm alongside its existing dig/vote/
  copy-target arms. It answers the ask with the round-robin multiset, so tests
  written around the pre-ask engine keep the exact board their assertions were
  written against.

No new `Decision.Validate` rule was added: `Min == Max == total` already
enforces "exactly the named total" and the option list already is "exactly the
chosen target list", while `Repeatable` already permits the multiset. One home,
reused.

### Tests added (`rules/fury_damage_split_test.go`)

- `TestFuryDividesDamageAmongTargets` — target ask offers `[0,4]`; both bears
  chosen; a DISTINCT `KChoose` (`ResumeKind "damage_split"`, `Min=Max=4`,
  `Repeatable`) appears; `Validate` rejects a 3-damage total and an out-of-range
  recipient; a `3/1` answer (not the round-robin `2/2`) is honoured and the total
  is 4; `replayCheck` proves the resolution replays from the log alone.
- `TestFuryDamageSplitAllowsZeroShare` — all 4 on one chosen target, the other
  chosen target takes 0.
- `TestFuryDamageSplitSkipsAskWithNoTargets` — zero-target election poses no
  allocation decision, deals nothing, and the trigger still resolves (no
  `unimplemented API` Note).

Each asserts its own preconditions: both bears are on the battlefield with
toughness > 4 (so a full share is not lethal — a 2/4 dies at 4/0 and its
`Damage` resets on the way to the graveyard, which is exactly the vacuity trap),
Fury is on the battlefield, and the target bounds are `[0,4]`.

## Gate commands and real output

Focused gate named by the brief:

```
$ go test -run 'TestFuryDividesDamageAmongTargets' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.462s
```

All three new tests plus every pre-existing test that resolves a
`DealDamage.DividedAsYouChoose$` carrier referenced in the suite:

```
$ go test -run 'TestFuryDividesDamageAmongTargets|TestFuryDamageSplit|TestEvokeCastPaysTheEvokeCostAndSacrifices|TestEvokeExileCostExilesTheChosenCard|TestMadnessExileByNonDiscardDoesNotOfferTheCast|TestPolukranosMonstrosityTriggerReadsX|TestKuldothaFlamefiendOptionalSacCostPays|TestKuldothaFlamefiendOptionalSacCostDeclineChangesNothing|TestAvacynsJudgmentTargetBoundCountsPlayersAndPermanents|TestMadnessDiscardSuspendsTheResolvingRepeat' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.868s
```

Goldens (gorge-context "Behaviour goldens outside `rules/`"):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.884s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

Chain-head golden — measured, UNMOVED (no re-pin, no attribution needed):

```
$ go test -run TestHeads ./rules/
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

Lint (Go half of `make lint`, plus the type generator):

```
$ gofmt -l <changed files>          # no output
$ go vet ./effects/ ./rules/        # no output
$ go run ./cmd/gentypes -check      # no output
```

Corpus presence: `.cards` exists in this worktree as a symlink to the shared
corpus (verified at session start), so the corpus-backed tests ran rather than
skipped.

## Fails without the fix

Proof method: `cp effects/damage.go .ds4/scratch/damage.go.fixed`, then in the
real file gate the ask behind `if false && Ask(h, d) == AskAsked {` (so the
round-robin stand-in always runs and no allocation decision is ever posed), run
the new tests, confirm FAIL, then restore with `cp` and `cmp`:

```
$ cmp effects/damage.go .ds4/scratch/damage.go.fixed
damage.go restored byte-identical
```

Failing output with the fix reverted:

```
--- FAIL: TestFuryDividesDamageAmongTargets (0.45s)
    fury_damage_split_test.go:101: allocation ask missing after target selection:
--- FAIL: TestFuryDamageSplitAllowsZeroShare (0.00s)
    fury_damage_split_test.go:144: allocation ask missing after target selection:
```

(`TestFuryDamageSplitSkipsAskWithNoTargets` still passes with the revert — it
asserts the zero-target no-ask path, which the revert does not remove.)

## Head / ratchet movement

- **`TestHeads`: UNMOVED** at 2/4/6/8. The acceptance goldens seat from the
  closed 12-deck `LegacyDeckNames()` list (`internal/testutil/decks.go`), and
  the three repo decks that carry a `DealDamage.DividedAsYouChoose$` card
  (Forked Bolt in foundations-reign-of-dragons, Fury in
  rakdos-muscle-scam-exe, Pyrokinesis in vivi-ornitier-cedh) are not among
  those 12, so no pinned head can see the extra decision events. Measured
  directly, not assumed.
- **`TestConstructedDefaultIsByteIdentical`: UNMOVED** (the default pair
  avengers-assemble:death-n-taxes has no carrier).
- **Ratchet (`knownUnsupported` / `knownUnsupportedParams`): untouched.** No
  table entry is implicated; the corpus measurement in the brief (16 DB-form
  files) held. No Known-approximations row names `DividedAsYouChoose` for
  `DealDamage` (grepped `AGENTS.md` and `internal/testutil/agentsdoc_test.go`),
  so there was nothing to delete and `knownApproximationRows` is unchanged.

## Deviations from the brief

- The brief said to extend/update the existing Fury test. Because a shared file
  append is a merge-conflict magnet (dispatch: "New tests go in a new file"), the
  old `TestFuryDividesDamageAmongTargets` was DELETED from
  `rules/rakdos_muscle_deck_test.go` and a same-named replacement (plus two
  siblings) lives in the new `rules/fury_damage_split_test.go`. The focused gate
  command is unchanged.
- `rules/replacement_updated_test.go` (a test helper only) was changed. This is
  required to keep pre-existing tests green: the new ask is an event the old
  drains did not know. The arm reproduces the old round-robin board exactly, so
  no existing assertion changed meaning.

## Open concerns

- **Bot answer equivalence.** For a `Min == Max == total`, `Repeatable` ask,
  `botpolicy`'s existing repair fills by cycling the ungrouped options in order,
  i.e. exactly the old round-robin split. So bot games' damage outcomes are
  unchanged; bot-driven games do gain the extra decision/`DecisionMade` events,
  which is why `TestHeads` was measured rather than assumed. No live gate moved.
- **Large totals.** A card whose total is large (e.g. `Comet Storm` X=10) now
  asks for 10 picks rather than one allocation widget. Functional and
  deterministic, but the wire shape is a multiset of per-unit picks. A richer
  per-target-amount transport would need a new `Intent` field; that was out of
  scope and is not required for correctness.

## Issues

- **PutCounter / PreventDamage `DividedAsYouChoose$` allocation is untouched**
  (explicitly out of scope per the brief). `effects/counters.go` has the
  `Choices$`-based distribute shape (`putCounterPickDistribute`, `vow1`) and
  `effects/prevent_damage.go:146` still documents a round-robin stand-in over
  the named total. These are separate primitives and separate tickets.
  Corpus prevalence of the broad `DealDamage.*DividedAsYouChoose$` pattern is 73
  carrier files (`/usr/bin/grep -rlE 'DealDamage.*DividedAsYouChoose\$'
  .cards/cardsfolder | wc -l` → 73); the PutCounter/PreventDamage families are a
  comparable further set.
- No CR-lane test is added here. The existing conformance lane does not cover
  divided-damage allocation; CR 601.2d / CR 608.2b ("division is announced as
  part of casting and each target receives at least 1 if the total allows") would
  be the natural citation for a future lane test, but the brief did not ask for
  it.
