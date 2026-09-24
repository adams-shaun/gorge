# T1 report — K:Ninjutsu (agent-20260919T192459Z-df7ae824)

## What changed and why

`K:Ninjutsu:<cost>` (CR 702.49) is now implemented as the hand-zone activated
ability the keyword means: "{cost}, Return an unblocked attacker you control
to hand: Put this card onto the battlefield from your hand tapped and
attacking." `effects.Supported()` now reports `kw:Ninjutsu`.

| file | change |
|---|---|
| `cards/kw_ninjutsu.go` (new) | Expands a printed `K:Ninjutsu:<cost>` line into `AB$ ChangeZone \| Defined$ Self \| Origin$ Hand \| Destination$ Battlefield \| Tapped$ True \| Attacking$ True \| Cost$ <cost> Return<1/Creature.YouCtrl+attacking+unblocked> \| ActivationZone$ Hand \| ActivationPhases$ Declare Blockers,Combat Damage,EndCombat \| Keyword$ Ninjutsu`. The colon parameter is split at the FIRST field (Equip's convention), so Yuriko's `U B:Commander` rider never leaks into the mana cost. Registered via `registerKeyword(kwNinjutsu, "Ninjutsu")`. |
| `effects/filter.go` | New generic `unblocked` predicate: `o.IsAttacking && len(o.BlockedBy) == 0` (CR 509.1h). This is the filter half of the Return cost spec; it fails closed for a non-attacker. |
| `rules/cast.go` | (a) `pendingCast.ninjutsuDefender` captures the returned attacker's `Attacking` defender in the `returncost` answer arm, while the creature is still a battlefield object (CR 702.49b: the same player the returned creature was attacking). (b) `activationPushEvent` rides that defender on the `AbilityPush` event's `IDs` as a `PlayerRef`. (c) `activationIsNinjutsu`/`saHasKeyword` helpers. (d) `"kw:Ninjutsu"` registered in the `effects.RegisterNonAPI` init list. |
| `rules/stack.go` | In the activated-ability resolution Ctx build, a `Keyword$ Ninjutsu` resolving SA re-binds its `Remembered` player target to `ctx.DefendingPlayer`, so `effects/zone.go`'s `Attacking$ True` rider emits `TokenAttacks` against the right defender. |
| `cards/kw_registry_test.go` | Added `"Ninjutsu"` to `expandedHeads` (the two-way registry ratchet requires it; `TestNoKeywordIsRegisteredThatTheSwitchNeverExpanded`). |
| `rules/ninjutsu_test.go` (new) | Four tests, below. |

Structural choice (asked for by the dispatch's "fix the class, not the
instance"): the unblocked-attacker restriction is a general filter predicate,
not a ninjutsu-only special case, so any future `Return<.../Creature....+unblocked>`
or `ValidTgts$ ...+unblocked` spec is covered by construction. The
defender→resolution binding goes through the existing `AbilityPush`/`Remembered`/
`Ctx.DefendingPlayer` machinery, not a new engine-side map or a `state.Object`
field, so replay derives it identically.

## Corpus claim re-measured

The brief said **45 cards** carry `K:Ninjutsu`. Re-measured at this worktree's
`FORGE_REF`:

```
/usr/bin/grep -rlE '^K:Ninjutsu' .cards/cardsfolder | wc -l   ->  45
/usr/bin/grep -rhE '^K:Ninjutsu' .cards/cardsfolder | wc -l   ->  45
```

The claim **held**. 44 lines are plain `<mana cost>`; exactly one
(`yuriko_the_tigers_shadow.txt`) carries the `:Commander` rider.

## Gates run (real output pasted)

### Targeted tests — `go test -count=1 -run 'TestNinjutsu' -v ./rules/`

```
=== RUN   TestNinjutsuRealCardEntersTappedAndAttacking
--- PASS: TestNinjutsuRealCardEntersTappedAndAttacking (0.41s)
=== RUN   TestNinjutsuCommanderVariantSplitsTheRiderField
--- PASS: TestNinjutsuCommanderVariantSplitsTheRiderField (0.00s)
=== RUN   TestNinjutsuNotOfferedBeforeDeclareBlockers
--- PASS: TestNinjutsuNotOfferedBeforeDeclareBlockers (0.00s)
=== RUN   TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked
--- PASS: TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.429s
```

### `cards` keyword-registry ratchet

```
go test -run 'TestEveryExpandedKeywordHasAnExpander|TestNoKeywordIsRegisteredThatTheSwitchNeverExpanded|TestAnUnregisteredKeywordIsNotExpanded' ./cards/
ok  	github.com/adams-shaun/gorge/cards	0.003s
```

### `effects` filter/predicate tests

```
go test -run 'TestCompiled|TestPredicate|TestFilter' ./effects/
ok  	github.com/adams-shaun/gorge/effects	14.674s
```

### Behaviour goldens outside `rules/` (system-t1.md mandate)

```
go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.661s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.657s
```

### Format / generated-code lint (the Go half of `make lint`)

```
gofmt -l cards/kw_ninjutsu.go cards/kw_registry_test.go effects/filter.go rules/cast.go rules/stack.go rules/ninjutsu_test.go
(no output — clean)

go run ./cmd/gentypes -check
(no output — clean)
```

### Corpus presence

`.cards` existed in this worktree (symlink to
`/home/sadams/projects/gorge/.cards`) — verified before the first run, so the
runs above are real, not corpus-skipped.

## Fails without the fix

**(1) The positive real-card test fails with the expander neutered** (registration
kept, so it is a behaviour failure, not the registration precondition).
`cards/kw_ninjutsu.go` was copied to `.ds4/scratch/kw_ninjutsu.go.bak`, the
`parseSA`/append block replaced by a bare `return`:

```
--- FAIL: TestNinjutsuRealCardEntersTappedAndAttacking (0.42s)
    ninjutsu_test.go:93: ninjutsu ability not offered at the declare-blockers step: &{Seq:83 Player:0 Kind:priority ... Options:[{Index:0 Kind:pass ...} {Index:1 Kind:concede ...}]}
FAIL
```

Restored and checked: `cmp cards/kw_ninjutsu.go .ds4/scratch/kw_ninjutsu.go.bak` → `IDENTICAL`; `go build ./...` → `BUILD_OK`.

**(2) The Yuriko colon-field test fails without the field split**
(`cost := strings.TrimSpace(param)` instead of `strings.Cut(param, ":")`),
backed up to `.ds4/scratch/kw2.bak`, run, then restored:

```
--- FAIL: TestNinjutsuCommanderVariantSplitsTheRiderField (0.41s)
    ninjutsu_test.go:159: the commander rider leaked into the ninjutsu cost: "U B:Commander Return<1/Creature.YouCtrl+attacking+unblocked>"
FAIL
```

Restored: `cmp ... .ds4/scratch/kw2.bak` → `RESTORED_IDENTICAL`.

## New tests (each can fail; preconditions asserted)

- `TestNinjutsuRealCardEntersTappedAndAttacking` — real corpus **Walker of
  Secret Ways** (`Ninjutsu {1}{U}`). Asserts `effects.Supported()["kw:Ninjutsu"]`,
  that the face prints the keyword, that the card is in hand, and that the Bear
  is an **unblocked** attacker of seat 1 before the offer; then pays the Return
  cost with the Bear and asserts the ninja entered **tapped** and **attacking
  seat 1**, plus a full log-only replay check.
- `TestNinjutsuCommanderVariantSplitsTheRiderField` — real **Yuriko, the
  Tiger's Shadow**; asserts the keyword is printed, the ability expanded, and
  the cost parses with zero `Unknown`.
- `TestNinjutsuNotOfferedBeforeDeclareBlockers` — CR 702.49a window; asserts the
  step precondition then that the hand ability is withheld.
- `TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked` — asserts the ability IS
  offered while the Bear is unblocked (precondition), then withholds once a
  blocker is recorded.

No `T:`/`S:`/`R:` registry ratchet or trigger-mode ratchet is affected (no new
trigger mode). `knownUnsupported` (`rules/acceptance_test.go`),
`knownUnsupportedParams`, `knownUnmodelledCountHeads` and `knownApproximationRows`
are untouched; `kw:Ninjutsu` was never a row in AGENTS.md's closing register, so
nothing is deleted there.

## Head / ratchet movement

None measured and none expected on the repo decks: none of the 14 constructed
or 10 Commander decks in `internal/testutil/decks/` carry a `K:Ninjutsu` card
with the offered shape as far as this ticket changed (the acceptance ratchet
and `TestHeads` are daemon gates and were not run per the seat budget). No
chain head was re-pinned.

## Deviations from the brief

- **The rebase-first directive was not executed.** The branch was already even
  with `main` (`git rev-list --left-right --count main...HEAD` → `0  0`), so a
  rebase is a no-op; and `system-t1.md` explicitly forbids `git rebase` in a
  seat. The daemon rebases at landing.
- **No deck file added.** The brief's "Done means" says the primitive ratchet is
  the defender *once the deck file is added*; it does not ask this ticket to add
  one, so none was added (adding a repo deck would move the acceptance ratchet
  and `TestHeads` and is a separate decision).

## Issues

1. **Commander ninjutsu's command-zone activation is not offered** (Yuriko, the
   Tiger's Shadow; the one corpus carrier, `.cards/cardsfolder/y/yuriko_the_tigers_shadow.txt`).
   CR 903.8's commander ninjutsu may also be activated from the command zone;
   `rules/legal.go`'s activated-ability offer walk enumerates only
   battlefield/graveyard/hand, and `abilityZoneOK` (rules/legal.go) rejects
   `Command`. Setting `ActivationZone$ Command` would withhold the whole ability,
   so the expander keeps `Hand` and Yuriko's **hand** activation works. Closing
   the command-zone half needs `Command` added to the offer walk (and a
   `castRestricted`/`castSuppressed` review for command-zone sources) — a
   separable change. Corpus prevalence: 1 card, 1 raw line.
2. **The `unblocked` predicate is new and general but only ninjutsu uses it.**
   The corpus's other `attacking`-family specs (e.g. `ValidTgts$ Creature.attacking`)
   were already modelled; `unblocked` had no prior carrier, so its only live
   consumer is this keyword. If a future card wants "unblocked" outside a cost,
   the predicate is already there.
3. **Blocked-attacker negative is asserted with a direct `BlockedBy` write**
   (`TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked`) rather than a driven
   `KBlockers` ask, so that test does not carry a `replayCheck`. The gate itself
   (`costCandidates` → filter) is the same code the positive test exercises via
   replay; only the negative's setup is synthetic. A follow-up could drive a
   real block for that case if desired — not required by the brief.
