# Round sol5 (continuation) — fix the opening-hand Effect non-Phase drop

This round addresses the single outstanding finding in `findings-sol5.md`
(the round before it was lost before reporting; its committed work was found
clean in the worktree at `b7846aa` and kept — `git status` was empty at
round start). The prior rounds' ticket work (buyback, transmute, suspend,
convoke, harmonize, cycling, flash, opening hand) was already committed and
reviewed; this round is the one MAJOR finding's fix plus its gates.

## Worktree fixture

`.cards` was already present (symlink to the shared corpus), so corpus-backed
tests ran. `.ds4/` contents were copied in by the controller.

## The finding, and what changed

**[MAJOR] rules/opening_hand.go — `registerOpeningEffectTriggers` silently
dropped every opening-hand `Effect` trigger whose mode was not `Phase`.**
Chancellor of the Annex's `DBEffect` (`Triggers$ TrigCounter`, a one-off
`Mode$ SpellCast` trigger) was discarded, so the reveal promised a counter
that never existed. Fixed by giving the delayed-trigger machinery an
event-matched half, not by special-casing the card:

- `state/game.go` — `state.DelayedTrigger` grows `EventMode`/`Trigger`
  (value fields; Clone's per-entry value copy carries them for free).
  `EventMode` is the trigger `Mode$` the registration fires on (`"SpellCast"`);
  `Trigger` is the SVar name on the source's face holding that trigger body,
  re-parsed at fire time so `ValidCard$`/`ValidActivatingPlayer$` are
  evaluated against the actual cast. Both empty for a Mode$ Phase
  registration — every already-working registration is unchanged.
- `events/apply.go`, `events/event.go` — the `DelayedRegister` case decodes
  the pair from the event's `Text` field (`"<Mode>:<Trigger>"`). The event
  gains NO field (the binary encoding and every earlier ordinal are
  untouched); this is the TriggerPush field-reuse precedent (Ruling T20-a).
  A Mode$ Phase registration's Text is the Forge Phase$ string, which never
  contains a colon, and only the colon-bearing `"SpellCast:"` encoding
  decodes differently, so every already-logged registration decodes as a
  phase one.
- `cards/parse.go` — new exported `cards.ParseTriggerLine`: one shared pipe
  grammar for a T:-shaped SVar body, replacing the opening path's private
  `openingTrigger`/`parseOpeningTrigger` (deleted) — the registration and the
  firing now parse the same body through the same helper, so the two readers
  cannot drift.
- `rules/opening_hand.go` — `registerOpeningEffectTriggers` now handles the
  `SpellCast` branch: one `DelayedRegister` per player the parent Effect's
  `EffectOwner$` selector names (new `openingEffectOwners` helper: empty/
  `You` keeps the revealer; `Opponent`/`Other` fans out to every other
  surviving seat — Chancellor's oracle is "when each opponent casts their
  first spell"; anything else fails closed, registering nothing). Still fail
  closed: non-one-off bodies, `OptionalDecider$` bodies (the DelayedPush path
  never poses an ask), unresolvable `Execute$`, and every trigger mode other
  than Phase/SpellCast. The registration emits `Step: e.G.Step` — the Step
  only satisfies Apply's validity guard; an event-matched registration never
  fires on a step.
- `rules/trigger_match.go` — `checkDelayedTriggers` skips event-matched
  registrations (they must not fire at their Step); new
  `checkEventDelayedTriggers` + `eventDelayedSpellCastMatches` fire a
  `SpellCast` registration on the spell's `PutOnStack`, hooked into
  `checkTriggers` (so the CR 601.2i deferred-cast path holds it until
  payment, exactly like a face cast trigger). Matching mirrors
  `spellCastMatches`' clause grammar (`ValidCard$`, `ValidActivatingPlayer$`,
  `PlayerTurn$`) plus the shared CR 603.4 `triggerConditionHolds` gate, with
  one deliberate difference: "you" is measured against `dt.Controller`, the
  effect owner the registration was minted for, not the source card's
  controller. `TriggerZones$` is deliberately not consulted (documented in
  the code): Forge parks the trigger on a command-zone Effect; the engine's
  registration itself is that presence — the source Chancellor stays in its
  hand. The fired pending trigger captures `Remembered` = the cast spell (the
  same `triggerRemembered` rule the face path uses, which is what
  `Defined$ TriggeredSpellAbility` resolves through) and
  `TriggerContext` = `triggerReferents`' SpellCast capture (the same
  event-provenance walk a face trigger takes).
- `rules/trigger_queue.go` — `pushTrigger`'s DelayedPush arm now stores the
  minted ability's `TriggerContext` (the DelayedPush path never did). A
  phase registration's context is the zero value, so storing it is
  behaviourally the absence every phase delayed trigger read before — no
  existing delayed-trigger behaviour moves.
- `rules/paramcensus_test.go` — two new trigger-bucket attribution roots
  (`Engine.registerOpeningEffectTriggers`, `Engine.checkEventDelayedTriggers` —
  ATTRIBUTION only, per that table's contract); the now-read
  `param:api:Effect.EffectOwner` labels retired from `Palace Jailer` and
  `Chandra, Awakened Inferno` (the census demanded exactly this: "a table
  entry the build now reads is stale and must be deleted").

## Corpus prevalence (measured this round)

```text
$ /usr/bin/grep -rl 'K:MayEffectFromOpeningHand' .cards/cardsfolder | wc -l
30

$ /usr/bin/grep -rl 'K:MayEffectFromOpeningHand' .cards/cardsfolder | while read f; do /usr/bin/grep -l 'DB$ Effect' "$f"; done | wc -l
8     # 5 Chancellors + Devourer of Destiny + Providence + Sphinx of Foresight

# their Effect children's Triggers$ bodies, re-parsed for Mode$:
#   TrigCounter (annex)     = Mode$ SpellCast  <- the only non-Phase shape
#   TrigDrain (dross)       = Mode$ Phase/Upkeep
#   TrigToken (forge)       = Mode$ Phase/Upkeep
#   TrigMill (spires)       = Mode$ Phase/Upkeep
#   TrigMana (tangle)       = Mode$ Phase/Main1
#   UpkeepTrig (devourer)   = Mode$ Phase/Upkeep
#   TrigSetLife (providence)= Mode$ Phase/Upkeep
#   TrigOpenScry (sphinx)   = Mode$ Phase/Upkeep
```

So the class this implements is exactly the whole measured non-Phase
population; every other opening Effect child was already (and remains)
registered through the Phase branch.

## Tests added (`rules/opening_effect_spellcast_test.go`, real card scripts)

- `TestChancellorOfTheAnnexRegistersOneSpellCastTriggerPerOpponent` —
  registration shape (EventMode/Trigger/Execute/Controller/Source) in a
  2-seat game, plus the `EffectOwner$ Opponent` fan-out in a 3-seat game
  (one registration per other seat, seat order).
- `TestChancellorOfTheAnnexCountersOnlyEachOpponentsFirstSpell` — the
  revealer's own cast does not fire the opponent-owned trigger and does not
  consume it; seat 1's first spell (empty pool) poses the `{1}` unless-pay
  ask TO SEAT 1, the decline counters it into the graveyard with no draw,
  exactly one `DelayedPush`, the registration is consumed; a `ModeChosen`
  event records the answer; the second spell resolves normally (one draw, no
  further `DelayedPush`).
- `TestChancellorOfTheAnnexOpeningRevealDrivesTheCounter` — the REAL pregame
  flow (`New` → the `opening_yes` decision → `handleOpening`), then the
  opponent's first spell countered through the ordinary stack; additionally
  asserts a log-alone reconstruction (`replayFromLog` + `diffGames`, events
  folded into a fresh Game with no engine) matches the live game exactly —
  the proof that `EventMode`/`Trigger` survive the event log rather than
  living only in engine fields.

## Gates (exact commands and real output)

```text
$ go test ./rules/ -run 'TestChancellorOfTheAnnex' -v
=== RUN   TestChancellorOfTheAnnexRegistersOneSpellCastTriggerPerOpponent
--- PASS: TestChancellorOfTheAnnexRegistersOneSpellCastTriggerPerOpponent (0.46s)
=== RUN   TestChancellorOfTheAnnexCountersOnlyEachOpponentsFirstSpell
--- PASS: TestChancellorOfTheAnnexCountersOnlyEachOpponentsFirstSpell (0.49s)
=== RUN   TestChancellorOfTheAnnexOpeningRevealDrivesTheCounter
--- PASS: TestChancellorOfTheAnnexOpeningRevealDrivesTheCounter (0.49s)
ok      github.com/adams-shaun/gorge/rules      1.441s

$ go test ./rules/ -run 'TestGemstoneCavernsOpeningHandEffect|TestSuspendDoesNotCastThroughCantBeCast|TestVedalkenOrreryAppliesOutsideHand|TestConvokeOverSelectionIsRejectedAndResubmitted|TestParamCensusScanIsComplete' -v
--- PASS: TestSuspendDoesNotCastThroughCantBeCast (0.47s)
--- PASS: TestConvokeOverSelectionIsRejectedAndResubmitted (0.46s)
--- PASS: TestVedalkenOrreryAppliesOutsideHand (0.46s)
--- PASS: TestGemstoneCavernsOpeningHandEffect (0.47s)
--- PASS: TestParamCensusScanIsComplete (0.02s)
ok      github.com/adams-shaun/gorge/rules      1.901s
        (the findings file's other four break attempts all still hold)

$ go test ./rules/ -run 'TestDelayed|TestResume|TestChancellor|TestGemstone|TestImpatient|TestOpeningHand' -v
... 18 runs, all PASS (phase delayed triggers, resume-drain invariants,
    Gemstone, Impatient Iguana, the old Chancellor phase pin) ...

$ go test ./cards/ ./state/ ./events/ ./replay/ ./host/ ./view/
ok      github.com/adams-shaun/gorge/cards      6.255s
ok      github.com/adams-shaun/gorge/state      0.010s
ok      github.com/adams-shaun/gorge/events     0.011s
ok      github.com/adams-shaun/gorge/replay     11.490s
ok      github.com/adams-shaun/gorge/host       20.681s
ok      github.com/adams-shaun/gorge/view       2.631s

$ go test ./rules/
ok      github.com/adams-shaun/gorge/rules      137.418s
        (first full run exposed two stale census labels, fixed below; second run green)

$ go test ./rules/ -run 'TestEveryRepoDeckIsFullySupported$' -v
    acceptance_test.go:158: ratchet: 36 of 579 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.45s)

$ go test ./rules/ -run 'TestHeads$' -v
--- PASS: TestHeads (1.11s)
ok      github.com/adams-shaun/gorge/rules      1.121s

$ make sim 2>&1 | /usr/bin/grep -c 'replay OK'
20

$ gofmt -l . && go vet ./... && go run ./cmd/gentypes -check && echo STATIC-OK
STATIC-OK
```

## Heads / ratchet movement

- **TestHeads: no movement** (passed unchanged, `1.11s`). No repo-deck card
  carries `Chancellor of the Annex`/`Gemstone Caverns`/`Impatient Iguana`/
  `Providence`/`Devourer of Destiny` (measured `grep -l` over
  `internal/testutil/decks/*.json` — no match), and the change touches no
  phase-delayed-trigger behaviour (their stored `TriggerContext` is the zero
  value they always read).
- **Primitive ratchet: 36 of 579**, no entry retired or added by this round
  (the brief's entries were retired in earlier rounds; the 41→36 delta
  between rounds came from the earlier rounds' merges, not this commit).
- **Param census**: retired `param:api:Effect.EffectOwner` from `Palace
  Jailer` and `Chandra, Awakened Inferno` — a real read was added (the
  opening Effect path now resolves `EffectOwner$`), which is the census's
  own rule. See Issues for why those two cards' own shapes are still unread.
- Commit-time budgets (pre-commit hook): rules 131.3s 839 tests budget 300s —
  no budget raise needed; the standing grant was not exercised.

## Deviations from the brief

None beyond the brief's own scope: this round implements the finding's
requirement ("supported one-off non-phase opening triggers, at least
SpellCast, through the ordinary delayed-trigger path") and keeps everything
else fail-closed. No `knownUnsupported` entry was added anywhere.

## Issues

1. **`EffectOwner$` selectors beyond the opening-hand shapes remain unread
   in `effEffect`.** `rules/opening_hand.go`'s `openingEffectOwners`
   resolves `""`/`You`/`Opponent`/`Other` and fails closed on anything else;
   the measured exotic values live on NON-opening cards whose Effect path
   (`effects/misc.go` `effEffect`) has never read `EffectOwner$`:
   `Palace Jailer`'s `EffectOwner$ TargetedOwner` (1 corpus file) and
   `Chandra, Awakened Inferno`'s `EffectOwner$ Player.Opponent` (1 file).
   The census label retirement above is per-PRIMITIVE, not per-card-path, so
   those two cards' labels retired while their own shapes are still unread —
   nobody should read the retired labels as "Palace Jailer's Effect is now
   EffectOwner-aware". This is the existing "Effect" known-approximation row
   (M4: full Effect duration/ownership grammar) in AGENTS.md; no new ledger
   entry needed, but a reviewer grepping `EffectOwner` should land here.
2. **An event-matched (non-phase) opening trigger carrying `OptionalDecider$`
   fails closed at registration.** Measured population at the current pin:
   0 (the corpus's only non-Phase opening Effect child is Chancellor of the
   Annex's, which is mandatory). If a future script adds one, the DelayedPush
   path never poses an ask, so the registration is withheld rather than made
   mandatory — the conservative direction, but it means such a card would
   silently do nothing until the Delayed arm grows an optional ask.
3. **`TriggerZones$` is not consulted for event-matched delayed
   registrations** (deliberate, documented at `checkEventDelayedTriggers`):
   the registration itself stands in for the command-zone Effect presence.
   If a future event-matched registration ever needs a REAL zone gate (its
   source live in a specific zone at fire time), that is unread today.
   Measured population at the current pin: 1 (Chancellor of the Annex,
   `TriggerZones$ Command`, whose source legitimately stays in hand).

## Commits

- `ae71805` fix(rules): register opening-hand Effect SpellCast triggers per opponent
