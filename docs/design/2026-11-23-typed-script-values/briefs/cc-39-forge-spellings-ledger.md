# feat(cardc): CC-39 burn-down: Forge's spellings and the served-card ledger

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs last in wave
5: it lands after CC-38 and sweeps the rows the earlier tickets' newly served cards brought.

## Goal

Drive the ledger's `equivalent` rows to 0 and its static `cardc-gap` rows outside the `cond` family (CC-34's) to the
fewest the IR allows, by respelling gorgebe to Forge's form, and triage the lockstep rows.

## Design

Decided.

1. **Closed default list.** Some params mean the same written or absent, and Forge's corpus writes them either way
   (at `a0be533401`, of 3,873 Draw lines, 1,049 write `Defined$ You` and 2,408 write no `Defined$`; 983 write
   `NumCards$ 1`). For these only, gorgebe follows the served card's Forge script: it writes the param when Forge's
   matching ability writes it, and omits it when Forge omits it. The list is closed and held in one table in
   `engine/cardc/gorgebe/respell.go`: `(API, param, default)` rows, at least Draw `NumCards` 1, ChangeZone `ChangeNum`
   1, and `Defined` `You` on Draw, Discard, GainLife, LoseLife and Mill. Adding a row needs a reader citation
   (`engine/effects` `file:line`) showing absent and the default read the same. The follow step may only add or
   remove a listed param with its default value; it never changes any other text. `TestRespellDefaults` holds the
   table and that rule.
2. **Everything else is a printer fix from IR facts**: trigger `Origin$ Any` where Forge writes it, `TriggerZones$`,
   `ValidSource$`, type-word order in `Affected$` and `ValidTgts$` (`Creature.Elf`, Forge's order), `Produced$` combo
   order, `ChangeType$` and `ValidCard$` controller terms, LKI `Defined$` forms (`TriggeredCardLKICopy`,
   `TriggeredSourceSA`), SVar names Forge uses (`X`, `Y`), and `Count$` heads Forge writes (`Kicked`, `Threshold`,
   `Sacrificed$CardPower`).
3. **Lockstep rows** (15 cards and 6 unattributed witnesses at the base): replay each witness, name the first
   diverging decision, fix the printer when it is a cardc-gap, else reclass with evidence (PLAN.md section 4).
4. Rows that need an IR change stay `cardc-gap` with a note, and go under Decisions.

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestRespell|TestRespellDefaults|TestTriggerParams|TestStepNodes|TestMoveNodes)$'`
and `make test-one PKG=./engine/cardc/diff RUN='^(TestCanon|TestCardRows|TestDiffSlice)$'` (canon.go unedited).
Hand-back: `equivalent` and `cardc-gap` before and after, and every remaining row's field and cause.

## Collisions

Touches every printer. Runs beside CC-38, rebases onto it, and re-runs `cardc ledger -write` last.
