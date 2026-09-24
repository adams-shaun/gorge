# Report — agent-20260922T193437Z-a964eea4

Effect `Triggers$` registration: identity half (`EffectOwner$`, `OneOff$`) and
the generic-registration pin.

Round 2. Round 1 (branch commits `648ed4eb1` + `2f7c0ccf6`) landed
`EffectOwner$` and a `SpellCast`/`ChangesZone`-only `OneOff$`. The reviewer
returned one MAJOR: `OneOff$ True` is honored only for `SpellCast`/
`ChangesZone`, leaving the `Phase` arm and the generic matcher arm recurring.

## What changed this round

### `effects/misc.go`
- **New shared helper `effectOneShotDelayedMode(mode)`** (one home). It names
  the event modes with an end-to-end non-repeat delayed dispatch: the
  `events.Apply` DelayedRegister decode recognizes the mode prefix without the
  `|EF` marker, AND `rules.checkEventDelayedTriggers` has a non-EffectRepeat
  arm that fires and removes it. That set is exactly
  `SpellCast, ChangesZone, ChangesController, DamageDone, AttackersDeclared`,
  the same set `effDelayedTrigger` admits.
- **`effEffect`'s `oneOff`** now uses `effectOneShotDelayedMode(tr.Mode)`
  instead of `(tr.Mode == "SpellCast" || tr.Mode == "ChangesZone")`. A
  `OneOff$ True` `DamageDone`/`ChangesController`/`AttackersDeclared` body
  now registers without `|EF` (`EffectRepeat=false`) and is consumed by its
  first firing.
- **`effDelayedTrigger`'s mode guard** now uses the same helper (it was a
  hand-written list of the identical set), so the two paths cannot drift and
  a mode cannot be one-shot on one path and inert on the other.
- **Stale loop comment corrected**: it still claimed `DamageDone` was a
  "mode with no delayed-event matcher" and named this ticket as the future
  owner of the generic registration, which had already landed under
  `cli-20260922T225138Z-504a0e97` (merged `5056e555`). Now it says every
  matcher-backed mode registers through the generic arm.

### `rules/effect_oneoff_modes_test.go` (new file)
- `TestEffectOneOffDamageDoneConsumesOnFirstFiring`: arms a `OneOff$ True`
  `DamageDone` body through the generic path, asserts the registration is
  non-repeat, fires once, and that a second matching damage fires nothing.
- `TestEffectOneOffPhaseIsOneShot`: asserts a `Mode$ Phase` registration is
  the one-shot no-`EventMode` shape, fires once at its upkeep, and is gone
  afterwards.

## The MAJOR — fixed and answered

> [MAJOR] `OneOff$ True` is honored only for `SpellCast` and `ChangesZone`;
> the `Phase` arm and the generic matcher arm still append `|EF`, creating
> recurring registrations.

- **Generic matcher arm** — fixed: the OneOff decision now covers every mode
  with a wired non-repeat dispatch, not just `SpellCast`/`ChangesZone`. Pinned
  by `TestEffectOneOffDamageDoneConsumesOnFirstFiring`, which FAILS with the
  fix reverted (output below).
- **`Phase` arm** — rebutted, and pinned. The finding's premise is wrong: the
  `Phase` arm never appended `|EF`. Its emitted `Text` is
  `tr.Params["Phase"] + expiry + odSuffix`, with no `efMarker` term, and
  `events.Apply` sets `EffectRepeat` only from a `|EF` suffix. A phase
  registration is therefore inherently one-shot — the `DelayedPush` that fires
  it removes it (`events.Apply`, `!dt.EffectRepeat`). `OneOff$` is not even
  consulted on that arm because a phase promise has no recurring form.
  `TestEffectOneOffPhaseIsOneShot` pins both the shape and the consumption. It
  passes with and without this round's fix, because the `Phase` arm was
  already correct; it is a regression guard against a future `|EF` on the arm,
  not a demonstration of the fix.
- **Remaining recurring modes under `OneOff$`** — deliberately out of scope
  and unchanged: a mode outside `effectOneShotDelayedMode` has neither an
  `events.Apply` decode for a non-`|EF` registration nor a
  `checkEventDelayedTriggers` non-repeat arm, so dropping `|EF` would make the
  body INERT, not one-shot. Corpus measurement of the `OneOff$ True` bodies
  (43 lines): 19 `Mode$ SpellCast`, 17 `Mode$ Phase`, 5 `Mode$ ChangesZone`
  are all one-shot-handled; the remaining 5 (`Untaps` 2, `SpellAbilityCast`,
  `LandPlayed`, `AbilityCast`) are unsupported trigger modes that already
  emit the "continuous effect trigger ... unimplemented" Note. So no corpus
  card regresses and none is newly one-shot. This residual is named under
  `## Issues` (needs new matcher + non-repeat dispatch, the broader
  printed-trigger gap the brief puts out of scope).

### Palace Jailer / `BecomeMonarch` exception (noted by the reviewer)
The reviewer flagged that the `BecomeMonarch` arm keeps `Player: c.Controller`
as "an explicit exception to the brief's 'every registering arm'". This is
deliberate and was landed in round 1 with its justification in the arm's
comment: the registration controller is the monarch relation's ANCHOR
(`rules.checkEventDelayedTriggers` reads `Player.OpponentOf Remembered`
against `dt.Controller`), and that anchor must be the SOURCE's controller —
the Jailer's player — not the exiled creature's owner. Resolving
`EffectOwner$ TargetedOwner` there would return the creature when the Jailer's
own controller takes the crown, the opposite of Palace Jailer's text
("until an opponent becomes the monarch"). `rules/monarch_jailer_multiseat_test.go`
pins both halves. `TargetedOwner` itself is a supported `Defined$` selector
for every other arm, covered by `TestEffectTriggerOwnerTargetedOwner`.

## Done-means checklist

- [x] `EffectOwner$` read in the generic `effEffect` `Triggers$` path (round
      1, `648ed4eb1`); owner is the registration controller; unresolvable
      selector fails closed with a Note and registers nothing
      (`TestEffectTriggerOwnerUnresolvableFailsClosed`).
- [x] `OneOff$ True` cannot double-fire when the pregame path already
      registered the same body; pregame still works
      (`TestEffectOneOffDoesNotDoubleFireFromOpening`,
      `TestChancellorOfTheAnnexCountersOnlyEachOpponentsFirstSpell`). Home
      chosen: the PREGAME path defers to the generic path via
      `openingEffectAlreadyRegistered` (round 1), so the generic one-shot
      registration is the single home; the pregame pass re-registers only
      when the generic path withheld the body.
- [x] Generic registration pinned on the real corpus
      (`TestEffectGenericRegistrationHoldsOnCorpus`, Bonus Round) plus
      `rules/effect_event_modes_test.go` (DamageDone/SpellCast/ChangesZone
      recurring) and `rules/effect_frame_trigger_test.go` (self-exile frame).
- [x] Lifetime guard at `effects/misc.go:429` untouched; the guard text is
      byte-for-byte unchanged (this round's diff touches only the comment
      above the OneOff block, the OneOff expression, the helper, the
      `effDelayedTrigger` guard and the loop comment — no guard line).
- [x] New tests assert their preconditions and are shown to FAIL without the
      fix (below).
- [x] `go test ./internal/archtest/` passes.
- [x] `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`
      passes (no movement — confirmed the brief's prediction that the 226
      carrier names do not intersect the repo-deck universe).
- [x] Targeted rules run green (command and output below).
- [x] Known-approximations row: NOT closed. The `Effect` row at AGENTS.md:217
      is a compound row also covering `StaticAbilities$` modes and other
      grant keywords; this ticket touches only the `Triggers$` OneOff
      identity and does not close it. Left unchanged.
- [x] Report names selectors, OneOff home, headline pin, head/ratchet/botbench
      movement.

## Gates — exact commands and real output

Targeted rules run (the `TestEffectOneOff` additions join the brief's set):

```
$ go test -run 'TestEffectTriggerMatchesRegistrationOwner|TestEffectTriggerOwner|TestEffectTriggerExpires|TestEffectEventModes|TestEffectTriggerBodySelfExile|TestEffectOneOff|TestEffectGenericRegistration|TestEffectDamageDoneTrigger|TestEffectSpellCastTrigger|TestEffectChangesZoneTrigger' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.500s
```

Behaviour goldens:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.449s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.643s
```

Edited package (effects), run once:

```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	16.651s
```

Format / generated types (the Go half of `make lint`):

```
$ gofmt -l effects/misc.go rules/effect_oneoff_modes_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
```

## Fails without the fix

Reverted only the `oneOff` expression in `effects/misc.go` to the round-1
form `(... (tr.Mode == "SpellCast" || tr.Mode == "ChangesZone"))`, ran the new
generic-arm test, then restored the file byte-identically (`cmp` clean):

```
$ cp effects/misc.go .ds4/scratch/misc.go.fixed
$ (edit: oneOff -> SpellCast||ChangesZone)
$ go test -run 'TestEffectOneOffDamageDoneConsumesOnFirstFiring' ./rules/
--- FAIL: TestEffectOneOffDamageDoneConsumesOnFirstFiring (0.00s)
    effect_oneoff_modes_test.go:45: precondition: OneOff$ True body registered with the recurring |EF marker: {ID:0 Phase:main1 Source:81 Controller:0 Execute:TrigPain Remembered:[] MinTurn:0 EventMode:DamageDone Trigger:TrigDamage EffectRepeat:true ValidPlayer: MaxTurn:1 OptionalSpec: SourceIncarnation:0 TrackSource:false}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.003s
FAIL
$ cp .ds4/scratch/misc.go.fixed effects/misc.go && cmp effects/misc.go .ds4/scratch/misc.go.fixed
RESTORED_BYTE_IDENTICAL
```

`TestEffectOneOffPhaseIsOneShot` passes both with and without the fix — it
pins behaviour this round did not change, because the `Phase` arm was already
one-shot (the finding's premise about it was wrong, see above). It is not
offered as a failing-without-fix demonstration.

## Head / ratchet / botbench movement

None. `TestConstructedDefaultIsByteIdentical` passes with no re-pin.
`rules/heads_test.go` (`TestHeads`, `TestEveryRepoDeck…`) are daemon gates and
were not run; the brief predicted no movement (the 226 carrier card names do
not intersect the 1042 repo-deck names), and no repo deck exercises a
`OneOff$ True` Effect trigger, so the event stream is unchanged for the deck
set. `knownUnsupported` / `knownUnsupportedParams` are untouched; no new
`Mode$` matcher was registered, so `addedAfterTheSplit` needs no entry.

## Deviations from the brief

- The brief's "Scope note (audit 2026-09-24)" said the round-1 branch
  `648ed4eb` "implements `EffectOwner$` plus `OneOff$` for
  SpellCast/ChangesZone". This round completes `OneOff$` to every mode with a
  non-repeat dispatch; the `EffectOwner$` half was already complete and is
  unchanged.
- `effectOneShotDelayedMode` deliberately omits `BecomeMonarch`: that arm has
  its own anchor semantics (see the reviewer note above) and is consumed by
  its firing through a different path.

## Issues

- **`OneOff$ True` on the 5 corpus bodies whose mode has no matcher**
  (`Untaps` ×2, `SpellAbilityCast`, `LandPlayed`, `AbilityCast`; 43 total
  `OneOff$ True` lines, measured
  `/usr/bin/grep -rhE 'OneOff\$ True' .cards/cardsfolder | grep -oE 'Mode\$ [A-Za-z]+' | sort | uniq -c`).
  These modes have no `trigMatchers` entry, so `effEffect`'s default arm
  already emits the "continuous effect trigger … unimplemented" Note and
  registers nothing — the OneOff read is unreachable for them. Making them
  one-shot requires a new matcher plus a `rules.checkEventDelayedTriggers`
  non-repeat arm and an `events.Apply` decode entry, i.e. the broader
  printed-trigger gap the brief puts out of scope. No CR-lane test suggested:
  this is a registration gap, not a rules-violation shape.
- **`EffectOwner$` on the `BecomeMonarch` arm is ignored by design.** A reader
  auditing "every registering arm" will see `Player: c.Controller` there and
  may file it as a bug. It is not: the anchor is the monarch relation's, and
  `rules/monarch_jailer_multiseat_test.go` pins it. Named here so it is not
  rediscovered. If a future ticket wants `EffectOwner$` to matter for
  `BecomeMonarch`, it must first decide which relation the anchor names.
