# feat(cardc): CC-36 burn-down: modes, copies, choices, delayed triggers, mana and repeated actions

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs first in wave
5, beside CC-34.

## Goal

Serve the 41 cards below (23 backend, 18 front end) by printing or lowering their modal spells, token and spell
copies, choices, delayed triggers, mana actions and repeated actions in Forge's shape.

## Cards (reason at `a0be533401`)

Backend (`unprintable:`):
- `verb:choose-modes` (10): Abrade, Apothecary Stomper, Bushwhack, Goblin Surprise, Kykar, Zephyr Awakener, Seeker's
  Folly, Slagstorm, Sylvan Scavenging, Wardens of the Cycle, Warping Wail. Charms keep `AILogic$` only where Forge
  writes it with rules meaning (PLAN.md section 5).
- `verb:create-token-copy` (4): Electroduplicate, Extravagant Replication, Homunculus Horde, Self-Reflection
- `verb:copy-spell` (2): Chain Lightning, Thousand-Year Storm; `verb:delayed-trigger` (2): Faebloom Trick, Flickerwisp
- one each: `verb:choose` Strongbox Raider, `verb:choose-number` Sanctum Prelate, `verb:double-counters` Mossborn
  Hydra, `verb:fight` Affectionate Indrik, `shape:consecutive-tokens` Wurmcoil Engine

Front end (`refused:`):
- `Action.AddManaWithModifiers` (4): Cavern of Souls, Eldrazi Temple, Giada, Font of Hope, Secluded Courtyard
- `Action.RepeatableActions` (2): Ad Nauseam, Sphinx's Tutelage
- one each: `Action.ChooseACardName` Cabal Therapy, `Action.BecomeTheMonarch` Palace Jailer,
  `Action.CastAnyNumberOfSpellsFromExileWithoutPaying` Etali, Primal Storm, `Action.ChooseAPermanentOfEachPermanentTypeAvailable`
  Liliana, Dreadhorde General, `Action.EachPlayerMayCostOrFallback` Perforating Artist, `Action.EachPlayerStartingWithAction`
  Council's Judgment, `Action.RestartTheGame...` Karn Liberated, `Action.RevealCardFromOutsideGameAndPutInHand`
  Burning Wish, `Action.RevealCardFromOutsideGameAndPutInHandOr...` Karn, the Great Creator,
  `ManaProduce.EachManaColorAmongPermanents` Bloom Tender, `CreatableToken.ThoseTokens` Doubling Season,
  `Color.TheChosenColor` Heraldic Banner

## Workspace

- Backend: `engine/cardc/gorgebe/spell.go` (modes), `step_flow.go`, `step_token.go`, `step_mana.go`,
  `step_permanent.go`, `step_player.go`.
- Front end: `engine/cardc/mtgish/fe_action_al.go`, `fe_action_mz.go`, `fe_token.go`, `fe_program.go`.
- Readers: `engine/effects/registry.go` (`effects.Supported()`), `engine/effects/charm_params.go`.

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestStepNodes|TestAddMana|TestTokenMatch|TestIfAndForEach|TestPrintersCompose|TestPermanentNodes|TestModesAttested)$'`
and, when the front end changed, `make test-one PKG=./engine/cardc/mtgish RUN='^(TestAction.*|TestProgram.*)$'`.

## Collisions

Runs beside CC-34 (conditions, values, filters). CC-37 also edits `fe_action_*.go` and lands after this one.
