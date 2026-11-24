# feat(cardc): CC-38 burn-down: one-shot continuous effects, control and attachment

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs after CC-35
(it reuses CC-35's layer-effect printing), beside CC-39.

## Goal

Serve the 22 cards below by printing their "until end of turn" (or other duration) effects on one or each permanent,
on a player, and their control, attachment and prevention actions in Forge's shape (`Pump`, `PumpAll`, `Effect`,
`Animate`, `GainControl`, `Attach`, as each card's Forge script has it).

## Cards (reason at `a0be533401`)

- `effect:obj` (11): Adventuring Gear, Balmor, Battlemage Captain, Dauntless Veteran, Divine Resilience, Goblin
  Piledriver, Heroic Reinforcements, Inspiring Call, Overrun, Preposterous Proportions, Selfless Spirit, Zul Ashur,
  Lich Lord
- `effect:cant-be-blocked` (3): Kaito, Cunning Infiltrator, Rogue's Passage, Vampire Gourmand
- `effect:cant-block` (2): Frenzied Goblin, Sower of Chaos; `effect:add-ability` (2): Fake Your Own Death, Kellan,
  Planar Trailblazer; `effect:cant-be-targeted` Vines of Vastwood
- `verb:create-replacement` Fleeting Flight; `verb:gain-control` Involuntary Employment; `verb:attach` Celestial Armor

## Workspace

- Backend: `engine/cardc/gorgebe/step_effect.go`, `step_replace.go`, `step_permanent.go`; CC-35's layer-effect
  printing in `ability_static.go`.
- Readers: the Pump, PumpAll, Effect and Animate params in `engine/effects`.

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestEffectNodes|TestPermanentNodes|TestStatics)$'`.

## Collisions

Runs beside CC-39, which lands after it and respells across every printer.
