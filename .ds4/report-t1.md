# GenericChoice per-Defined$-player chooser — report

Ticket: `agent-20260922T234314Z-bbfff2fb`
Branch: `wt/agent-20260922T234314Z-bbfff2fb`
Commit: `c0a04453`

## What changed and why (per file)

`effects/misc.go`
- New `charmGenericPlayers` / `charmGenericPlayersRun` / `playerRoleDefined`.
  `effCharm` (registered for both `Charm` and `GenericChoice`) now calls
  `charmGenericPlayers` before its existing body. When the SA is a
  `GenericChoice` whose `Defined$` resolves to **player targets that are not
  exactly the resolving controller**, the driver asks each defined player the
  same `Choices$` in turn, binding that chooser as `Ctx.Remembered`, runs the
  chosen SVar body with that binding, and only then walks the outer
  `SubAbility$` (via the ordinary `Resolve` chain walk after `effCharm`
  returns).
- The existing single-controller path is untouched: no `Defined$`, `Defined$ You`,
  and any object-defined selector (`Targeted`, `Valid <filter>`, a mixed set)
  return false and keep the old `ResumeKind: "modes"` ask to `c.Controller`.
  A player-role selector (`Opponent`/`Player`/`Player.Opponent`/`Player.Other`/
  `You`/`TriggeredPlayer`/`TriggeredDefendingPlayer`) that resolves to zero
  players emits a Note and does nothing — it does not fall back to the
  controller.
- A nested mid-resolution ask inside a chosen body reports
  `SuspendGenericChoiceRest` so the remaining choosers survive it; the cursor
  is cleared around the body's `Resolve` (the fx41 discipline) so a nested
  `GenericChoice` resolves its own `Defined$`.

`effects/registry.go`
- `Ctx.GenericChoosers` / `Ctx.GenericChooserIndex` (the per-player cursor).
- `Host.SuspendGenericChoiceRest` and the plain-data `GenericChoiceRest`
  type, mirroring `SuspendVillainousRest` / `VillainousRest`.

`decision/decision.go`
- `Decision.ResumeGenericChoosers` / `ResumeGenericChooserIndex`, the runtime
  continuation state that carries the cursor across an ask.

`rules/resolution.go`
- `resumePoint.genericChoosers`/`genericChooserIndex`/`genericChoice`; `ask()`
  copies them off the Decision.
- `handleModes` new `"generic_players"` arm: records the chosen SVar name and
  resumes the GenericChoice SA (mirrors the `"villainous"` arm).
- `resumeResolution` re-binds `ctx.GenericChoosers/Index` (like
  `VillainousVictims`), plus switch cases `"generic_players"` (sets
  `ctx.Modes = [choice]`, advances the index PAST the answered chooser) and
  `"generic_players_rest"` (clears `ctx.Modes`, keeps the cursor).
- `SuspendGenericChoiceRest` impl and its `contFrame` fields /
  `buildContinuationChain` case, which re-enters the GenericChoice SA itself
  with `kind = "generic_players_rest"`.

`rules/clone.go`
- `Clone` copies the new Decision resume fields and `cloneResume` copies the
  cursor slice, so a clone taken at a pending per-player ask resumes
  identically.

`effects/context_test.go`
- The fake host gains the no-op `SuspendGenericChoiceRest` (required by the
  widened `Host` interface).

`rules/generic_choice_players_test.go` (new)
- `TestSeizeTheSpotlightAsksEachOpponentOnce` — real corpus Seize the
  Spotlight: opponent 1 asked first (Fame), opponent 2 second (Fortune), the
  controller never asked; chooser-bound `NoteCards$ Self` splits
  `[Fame]`/`[Fortune]`; exactly two `ModeChosen`; `SubAbility$ DBFame` runs
  once (hand +1 draw, exactly 1 Treasure).
- `TestSeizeTheSpotlightSingleOpponentAsksThatOpponent` — two-seat shape: the
  sole opponent is asked, never the controller.
- `TestSeizeTheSpotlightCloneKeepsChooserCursor` — a clone at the pending ask
  carries the cursor and completes both choosers.
- `TestGenericChoiceEmptyDefinedDoesNotAskTheController` — inline script with
  `Defined$ TriggeredPlayer` (resolves no players): exactly one no-players
  Note, no mode ask, controller life/hand untouched.
- `TestGenericChoiceNestedAskResumesRemainingChoosers` — inline script whose
  chosen `DB$ Discard | Defined$ Remembered | Mode$ TgtChoose` body poses its
  own KChoose; opponent 2 is still asked afterwards, opponent 2's DoGain
  gains them 5, and the outer `DBTail` runs exactly once.

## Commands run (real output)

Targeted suite (brief's expression + the directly affected clone/villainous
resume tests):

```
$ go test -run 'TestGenericChoice.*|TestTirelessProvisionerLandfallCreatesChosenFood|TestTirelessProvisionerLandfallCreatesChosenTreasure|TestDayOfTheDoctorChapterIVAsks|TestVillainousChoiceVisitsEveryDefinedOpponent|TestVillainousChoiceCloneKeepsTheVictimCursor|TestDalekEmperorVillainousChoiceAsksEachOpponentAndRunsTheirSacrifice|TestSeizeTheSpotlight' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.696s
```

Edited-package runs (one each):

```
$ go test ./effects/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/effects	2.536s

$ go test ./decision/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/decision	0.009s
```

Golden checks:

```
$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	2.816s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.177s
```

Format / types:

```
$ gofmt -l effects/misc.go effects/registry.go effects/context_test.go rules/resolution.go rules/clone.go rules/generic_choice_players_test.go decision/decision.go
(clean)
$ go run ./cmd/gentypes -check
(no output — ok)
```

`.cards` was present (symlink to `/home/sadams/projects/gorge/.cards`), so the
corpus-backed Seize tests ran for real (`rules` run took 0.7 s under the
targeted `-run`; the corpus `CorpusRegistry` did not skip — the Seize board
loaded).

## Fails without the fix

Reverted only the `effCharm` hook (`if charmGenericPlayers(h, c, sa) { return }`)
in `effects/misc.go`, ran the five new tests, then restored the file
byte-identically (`cmp` clean):

```
--- FAIL: TestSeizeTheSpotlightAsksEachOpponentOnce (1.17s)
    generic_choice_players_test.go:107: first GenericChoice chooser = seat 0, want opponent 1 (never the controller)
--- FAIL: TestSeizeTheSpotlightSingleOpponentAsksThatOpponent (0.00s)
    generic_choice_players_test.go:198: chooser = seat 0, want the sole opponent 1 (never the controller 0)
--- FAIL: TestGenericChoiceEmptyDefinedDoesNotAskTheController (0.00s)
    generic_choice_players_test.go:243: non-priority decision &{... Kind:modes ... Player:0 ... ResumeKind:modes ...} while draining the stack
--- FAIL: TestGenericChoiceNestedAskResumesRemainingChoosers (0.00s)
    generic_choice_players_test.go:310: first chooser = &{... Player:0 Kind:modes ...}, want opponent 1 with generic_players
--- FAIL: TestSeizeTheSpotlightCloneKeepsChooserCursor (0.00s)
    generic_choice_players_test.go:366: first chooser = seat 0, want opponent 1
FAIL
```

Each new test asserts its own precondition: the Seize board asserts the spell
is in hand and each opponent's bear is on the battlefield before casting; the
nested test asserts each opponent holds ≥2 cards so the Discard body's
TgtChoose ask is a real choice; the empty-Defined test asserts the handler ran
via the exact no-players Note (and that the controller's life/hand are
untouched).

## Heads / ratchet / botbench

- No chain-head movement measured or expected; `rules/heads_test.go` was not
  edited (the brief forbids it).
- No ratchet movement: `knownUnsupported` and `knownUnsupportedParams` tests
  are not in the targeted expression and no `.cards` card became newly
  supported by this change (it is a dispatch-path change, not a new
  primitive).
- `cmd/botbench` `TestConstructedDefaultIsByteIdentical` PASSED with the
  pinned numbers unchanged — no repo-deck card exercises a multi-player
  `GenericChoice`, so the split did not move.
- AGENTS.md's Known-approximations table has no GenericChoice row, so nothing
  was deleted and `knownApproximationRows` (18) is untouched.

## Deviations from the brief

None material. The brief said "when its Defined$ resolves to multiple
players"; the implementation also takes over when it resolves to a SINGLE
non-controller player (`Defined$ Opponent` in a two-seat game), because the
brief's own symptom (Seize the Spotlight, "each opponent") and the
`TestSeizeTheSpotlightSingleOpponentAsksThatOpponent` case both require the
opponent to be asked rather than the controller. The explicitly protected
"one-player GenericChoice forms" (`Defined$ You`, e.g. Tireless Provisioner)
are unchanged and still assert `ResumeKind == "modes"`.

## Issues

- **`GenericChoice` with an object-role `Defined$` still asks the controller.**
  10 corpus files use `Defined$ Targeted` or `Defined$ ParentTarget` on a
  `GenericChoice` (bronze_tablet, inspirit_flagship_vessel, decoy_gambit,
  remorseless_punishment, forbidden_ritual, face_to_face, tempest_efreet,
  torment_of_venom, tergrid_god_of_fright_tergrids_lantern, thrull_wizard).
  This ticket deliberately leaves those on the existing controller ask
  (the all-players guard falls back). Where the target is a player, Forge
  would ask that player; that remains a real defect. Not fixed here (out of
  the brief's "per-player Defined$" scope); a follow-up ticket should extend
  the takeover to a `Defined$` that resolves to a single player target even
  through `Targeted`.
- **`TempRemember$ Chooser` (25 carriers) and `ShowChoice$ True` (38 carriers)
  are unread.** Seize the Spotlight carries `TempRemember$ Chooser` (the
  chooser binding hint) and `ShowChoice$ True` (reveal the choice). The
  per-chooser result here is correct via `Ctx.Remembered`, but the two
  parameters themselves are inert. Not a defect this ticket introduced.
- **`TriggeredTarget` is ambiguous** (a trigger target can be a permanent or a
  player). The implementation only takes over when the resolved set is all
  players; an unbound `TriggeredTarget` keeps the old controller ask rather
  than the no-op Note. Named here so it is not mistaken for a covered shape.
- No CR-lane test is warranted: this is a dispatch-path fix, not a rules
  interaction; the existing `rules/generic_choice_players_test.go` covers it.

## Open concerns

- Per-chooser `CharmNum$`/multi-pick is not implemented (single pick per
  chooser). Measured: 0 corpus `GenericChoice` carriers use `CharmNum$`
  (`/usr/bin/grep -rlE 'GenericChoice.*CharmNum\$' .cards/cardsfolder | wc -l`
  → 0), so the single-pick shape is complete for the corpus.
- `GenericChoice` carriers total 153 files; 119 of those spell the API as
  `DB$ GenericChoice` and the rest as `SP$ GenericChoice` / other call sites.
