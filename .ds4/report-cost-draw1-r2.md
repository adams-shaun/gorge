# Report — r2 (fix round) — cost-draw1: `Cost$ Draw<X/You>` dynamic Draw cost

Fix round for the two MAJOR findings in `findings-r2.md`. Base after rebase on
main: `6f27aa55` (my round-1 test commit replayed on top of current main,
which already carries the `trig:CostedExecutor` window). Work commit:
**`dfb0ac7b`**.

## Verdict on the findings

**Both findings traced to the same root — and the duplicate-pay-ask MAJOR
turned out to be a FIXTURE ARTIFACT, not an engine bug.** Measured proof
below.

### Finding 2 (repeated pay asks) — root cause measured

The round-1 fixture moved cards with `searchMoveByName`, which ends with
`e.pending = nil` — wiping whatever ask the entry posed. Post-rebase, Titan's
entry DOES pose a real ChooseType ask (`rules/resolution.go` `case
"choosetype"`, the ct1 machinery), and `Engine.Ask`
(`rules/resolution.go:391`) parked its resume frame on `e.resume`. The wipe
destroyed only `e.pending`, leaving that stale `choosetype` frame armed.
Titan's trigger then resolved, the window posed its pay ask
(`e.choosing = chooseTriggeredCost`), and the ANSWER was hijacked by the
stale frame: `handleChoose` (`rules/turn.go`) dispatches `e.resume != nil`
**before** the `e.choosing` switch, so the window's first answer went to
`resumeResolution(rp kind "choosetype")` — which recorded the decline option's
label **as Titan's ChosenType** (measured: `ChosenType == "Do not pay"` after
declining) and left the ability unresolved. Something re-resolved the same
stack object → second window → second ask. The `e.ask` overwrite guard
(`rules/engine.go:2590`) only panics when `e.resume` AND `e.pending` are both
set, so the wiped-pending pose sailed through.

Instrumented evidence (scratch, since removed): at the first pay ask's
answer, `choosing=16 (chooseTriggeredCost) resume=true` — the mid-resolution
arm took it; at the second, `resume=false` — the choosing arm took it. In a
clean harness that ANSWERS the entry ask, the flow is: one `choosetype` ask →
one `trigger_cost_pay` ask → the discard body. `payAsks=1` measured.

So the fix for the test is to not create the stale frame: the rewritten
fixture answers Titan's ChooseType ask and asserts exactly one pay election.
No engine change — the displacement is engine-unreachable (a pending ask
blocks all engine flow until answered; `Advance` is parked on it), exactly as
the `e.ask` guard's own comment states.

### Finding 1 (vacuous Titan pin) — two real engine gaps found and fixed

With the entry ask answered "Bear" and a Grizzly Bears on the battlefield,
`drawCostCount` STILL folded to 0. Two gaps in the shares-type read:

1. **The BARE form `sharesCreatureTypeWith` (no referent argument) was
   wordUnknown** — `sharesTypeArg` requires the space-separated
   `<predicate> <referent>` shape, so the token failed closed and the
   candidate matched nothing. Corpus prevalence (measured,
   `/usr/bin/grep -rlE 'sharesCreatureTypeWith([<>,]|$)' .cards/cardsfolder`):
   exactly **2 files** — `titan_of_littjara.txt` and `plane_merge_elf.txt`
   (Kinfall: `ValidCard$ Creature.YouCtrl+sharesCreatureTypeWith` and
   `ConditionPresent$ Card.sharesCreatureTypeWith`). Both read the SOURCE,
   which is Forge's unqualified reading in a source-anchored filter.
2. **The referent's chosen type was invisible to the read.** Titan's
   `S:Mode$ Continuous | Affected$ Card.Self | AddType$ ChosenType` static
   makes Titan BE the chosen type, but that grant is materialised only inside
   the layer walk (rules' `resolveChosenTypes`); `hasType(r, word)` reads the
   printed face (`Creature Illusion`) and never sees "Bear".

Fixes in `effects/filter.go` (single-home, in the ONE classifier the matcher
and `UnknownPredicates` share):

- `wordPredicate` classifies the bare token as
  `(wordSharesCreatureType, "Self")`. The two-token form is unchanged; the
  bare card-type siblings (`sharesCardTypeWith` etc.) have no corpus carrier
  and stay unknown (fail closed).
- `sharesCreatureTypeWith` additionally probes the referent's recorded choice
  when its face carries the "is the chosen type in addition to its other
  types" static (new helper `faceIsTheChosenType`: a `Continuous` static with
  `Affected$` naming Self and `AddType$`/`AddTypes$` value exactly
  `ChosenType`). Measured self-grant carriers: Titan of Littjara, Adaptive
  Automaton, Metallic Mimic, Roaming Throne, Multiversal Passage, Thran
  Portal. A referent with no recorded choice grants nothing (the layer walk's
  own fail-closed direction); a non-creature recorded choice is kept out by
  `CreatureTypeWords`.

## Per-file changes

- `effects/filter.go` — the two fixes above (`wordPredicate` bare-form arm;
  `faceIsTheChosenType` + the chosen-type referent probe in
  `sharesCreatureTypeWith`).
- `rules/draw_x_cost_test.go` — rewritten:
  - new `cleanMoveByName` (a `searchMoveByName` that does NOT wipe the
    pending ask) and `titanBearFixture` (Bears on board → Titan enters →
    ChooseType answered "Bear", with the recorded choice asserted);
  - `TestTitanOfLittjaraDrawXCost` rewritten: asserts the fold is **exactly
    1** up front (the vacuous-zero precondition the finding named), asserts
    exactly ONE pay/decline election whose next ask is the discard body, and
    that paying draws **exactly 1** Draw event followed by the body's
    one-card discard;
  - new `TestTitanOfLittjaraDrawXDecline`: the decline arm on the same
    configured board — election still posed, no draw, no discard, hand and
    library unchanged;
  - the false "the chosen-type machinery is independent of this test's
    assertions" comment is gone; `TestDrawXCostSVarFoldsAndDraws` (Champion
    positive control) and `TestDrawXUnresolvableWithheld` are unchanged.

## Gates run (real output)

Targeted gate (the brief's command, extended with the new decline test):
```
$ go test -run 'TestDrawXCostSVarFoldsAndDraws|TestDrawXUnresolvableWithheld|TestTitanOfLittjaraDrawXCost|TestTitanOfLittjaraDrawXDecline|TestParseCostReportsUnmodelledCostTokens|TestParseUnlessCostDrawComponents|TestDrawCostDrawsThePayer' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.683s
```

Fails without the fix (hunks reverted via a scratch copy of
`effects/filter.go`, file restored byte-identically with `cmp` afterwards):
```
--- FAIL: TestTitanOfLittjaraDrawXCost (0.61s)
    draw_x_cost_test.go:183: drawCostCount(Titan) = 0, true; want exactly 1 (the one other Bear sharing the chosen type)
--- FAIL: TestTitanOfLittjaraDrawXDecline (0.00s)
    draw_x_cost_test.go:251: drawCostCount(Titan) = 0, true; want exactly 1
FAIL	github.com/adams-shaun/gorge/rules	0.648s
```

Behaviour goldens (the two ~2 s checks):
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.968s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.212s
```

Package touched outside `rules/` (effects — one full run, once):
```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.631s
```

Lint:
```
$ gofmt -l effects/filter.go rules/draw_x_cost_test.go     (no output)
$ go run ./cmd/gentypes -check                              (clean)
$ go vet ./...                                              (exit 0)
```

## Head / ratchet movement

None, with cause: no `Draw<X/…>` carrier and neither bare-form carrier
(Titan of Littjara, Plane-Merge Elf) is in `internal/testutil/decks/*.json`
(re-verified each name → 0 hits), so `TestHeads` and
`knownUnsupported`/`knownUnsupportedParams` cannot move by construction. The
botbench constructed-default golden is byte-identical (the changed filter
form is exercised by no repo deck). `heads_test.go` and the ratchet tables
were not edited.

## Deviations from the brief

- The bare-form filter classification + chosen-type referent read are
  production changes the brief's scope did not name — they are the minimum
  the findings demand (the brief's own Done means requires "pay → N Draw
  events (N = other creatures sharing a type)", which is unimplementable
  against a filter that reads 0 with the type configured). Blast radius:
  2 corpus carriers, no repo decks, goldens unchanged.
- Done means' Titan test is split into `TestTitanOfLittjaraDrawXCost` (pay
  arm) + `TestTitanOfLittjaraDrawXDecline` (decline arm) — same coverage,
  one assertion per arm, both failing on the vanilla filter.

## Fails without the fix

See the gate section above: both new Titan tests fail on vanilla
`effects/filter.go` with `drawCostCount(Titan) = 0, true; want exactly 1`;
restored file verified byte-identical (`cmp` clean) and the targeted gate
green again.

## Issues

- **Stale-resume hijack class (engine-adjacent, not fixed here).**
  `handleChoose` (`rules/turn.go`) dispatches `e.resume != nil` before the
  `e.choosing` switch, so an answered KChoose is consumed by ANY armed resume
  frame even when a different flow (`e.choosing`) posed the pending ask. In
  engine flow a pending ask blocks everything, so this is reachable only by a
  probe that wipes `e.pending` while a mid-resolution ask is outstanding —
  but the state `e.resume != nil && e.pending == nil` IS a legitimate engine
  state (the replacement-order flow's park), and a future asker that poses
  while that park is armed would be hijacked the same way. The `e.ask`
  overwrite guard deliberately does not cover that shape. Worth a ticket
  deciding whether `triggeredCostPaymentAsk` (and every `e.ask` caller that
  is not itself a mid-resolution ask) should assert `e.resume == nil`.
- **`trigcosted-double-pay-ask.md` (filed last round) should be CLOSED** —
  its symptom is the fixture artifact measured above, not an engine bug. I
  cannot edit the ledger; the controller should retire the filed ticket.
- **`etbreplacement-choosetype-not-run.md` (filed last round) is STALE** —
  post-rebase, Titan's `K:ETBReplacement:Other:ChooseCT` DOES pose a real
  ask (`ResumeKind: "choosetype"`, options from the owner's creature types)
  and the answer records `ChosenType` (asserted in the new fixture). The
  controller should verify against Cavern of Souls' probe and retire it.
- **Plane-Merge Elf's Kinfall is still inert on the `ConditionPresent$` half:**
  effects' own ConditionPresent/ConditionDefined evaluator does not evaluate
  filter predicates (it runs its sub unconditionally — the known gap in
  AGENTS.md's castprov1 note), so `ConditionPresent$
  Card.sharesCreatureTypeWith` never gates `TrigPumpAll`. The bare-form fix
  makes the token CLASSIFIED (UnknownPredicates silent) but the condition
  evaluator is a separate walk. The `ValidCard$` half of the same trigger
  now works. Would need a CR-lane test citing the trigger-condition rule to
  stop being invisible.
- **Out-of-scope carriers (unchanged from round 1, restated for the ledger):**
  `Armor Wars` carries `UnlessCost$ Draw<X/You>` — `ParseUnlessCost`'s
  hard-decline for Draw is pinned correct by `TestParseUnlessCostDrawComponents`
  (the unless-pay answer carries no X binding); the three unmodelled X count
  heads (`Count$YourCountersExperience` — Katara, `TriggeredPlayersTargets$Amount`
  — Hordewing Skaab, `TriggeredCard$CastTotalManaSpent` — Uncover the Moon
  Letters) resolve `ok=true, n=0` from `effects.EvalCountOK` — fail-OPEN,
  opposite `fixLifeXCost`'s fail-closed contract (withheld, not mispriced,
  only if EvalCountOK learns to refuse them); the `Count$xPaid` announced
  Draw form has 0 corpus carriers and stays declining.
- **Ticket-premise note on "if you do" election semantics** (from the brief's
  scope boundary, confirmed by the pay trace): the pay election IS the
  may-draw election; paying `Draw<X>` with a folded X of 0 runs the body with
  zero draws (the outcome a decline gives, plus the body's own events). The
  ticket's "zero matches … discards nothing" holds only for the decline arm.
