# feat(cardc): CC-37 burn-down: library searches, looks, reveals and zone moves

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs after CC-34
and CC-36, beside CC-35.

## Goal

Serve the 25 cards below by printing or lowering their library searches, looks, reveals, hand reveals and zone moves
in Forge's shape (`ChangeZone`, `Dig`, `DigUntil`, `RevealHand`, `PeekAndReveal` and the rest, as each card's Forge
script has it).

## Cards (reason at `a0be533401`)

Backend (`unprintable:`):
- `verb:search` (8): Campus Guide, Expedition Map, Ghost Quarter, Micromancer, Path to Exile, Recruiter of the Guard,
  Stoneforge Mystic, Sylvan Scrying
- `verb:move` (6): Infernal Vessel, Raise the Past, Relic of Progenitus, Rise of the Dark Realms, Uncharted Voyage,
  Valkyrie's Call
- `verb:reveal-hand` (4): Duress, Pilfer, Thought-Knot Seer, Thoughtseize
- `verb:look` (2): Delver of Secrets, Jace, the Mind Sculptor; `verb:dig-word` (2): Squad Rallier, Vivien Reid
- `verb:reveal` Goblin Guide; `verb:reveal-until` Spinner of Souls

Front end (`refused:`): `LookAtTopOfLibraryAction.SeparateIntoFaceUpFileAndFaceDownPile` Curator of Destinies.

## Workspace

- Backend: `engine/cardc/gorgebe/step_move.go`, `step_dig.go`, `step_zones_test.go`.
- Front end: `engine/cardc/mtgish/fe_action_al.go` (`registerLookAt`), `fe_action_mz.go`.
- Readers: the ChangeZone, Dig and DigUntil params in `engine/effects` (`effects.Supported()`, the `*_params.go` files).

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestMoveNodes|TestDigNodes|TestStepNodes)$'`
and, when the front end changed, `make test-one PKG=./engine/cardc/mtgish RUN='^(TestAction.*)$'`.

## Collisions

Runs beside CC-35 (statics, costs, keywords). Lands after CC-36, which also edits `fe_action_*.go`.
