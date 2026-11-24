# feat(cardc): CC-35 burn-down: statics, cost changes, costs, replacements and keyword arguments

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Shared rules, measurements and Done list: `cc-34-39-burn-down.md` beside this file; read it first. Runs after CC-34
and CC-36, beside CC-37.

## Goal

Serve the 48 cards below (34 backend, 14 front end) by printing or lowering their static abilities, cost changes,
costs, enter replacements and keyword arguments in Forge's shape.

## Cards (reason at `a0be533401`)

Backend (`unprintable:`):
- `static:self-cost-less` (5): Arcane Epiphany, Embercleave, Ghalta, Primal Hunger, Luminous Rebuke, Tolarian Terror
- `static:adjust-pt` (4): Batterskull, Goldvein Pick, Quick-Draw Katana, Sword of Fire and Ice
- `static:add-ability` (2): Akroma's Memorial, Swiftfoot Boots; `static:additional-cost` (2): Arbiter of Woe, Thrill
  of Possibility; `static:spells-cost-less` (2): Archmage of Runes, Mocking Sprite; `static:must-attack` (2): Flamewake
  Phoenix, Juggernaut
- one each: `static:body` Skyknight Squire, `static:cant-be-blocked-by` Steel Leaf Champion, `static:cda-power`
  Consuming Aberration, `static:extra-land` Loot, Exuberant Explorer, `static:hexproof` Crystal Barricade,
  `static:no-untap` Slumbering Cerberus
- `keyword:Enchant` (5): Blanchwood Armor, Imprisoned in the Moon, Rancor, Twinblade Blessing, Witness Protection;
  `keyword:Hexproof` Elenda, Saint of Dusk; `keyword:Affinity` Claws Out
- `repl:etb-counter-cond` Goblin Boarders; `repl:would-enter` Authority of the Consuls; `cost:return-to-hand` Daze;
  `activated:instant-only` Lion's Eye Diamond

Front end (`refused:`):
- `Cost.ItsManaCost` (2): Snapcaster Mage, Sphinx of Forgotten Lore; `Cost.Or` Eaten Alive; `Cost.PayManaX:value`
  Mausoleum Wanderer; `Cost.PutTopCardOfLibraryOfTypeOnBattlefield` Matter Reshaper
- `Rule.AsPutIntoAGraveyardFromAnywhere` Progenitus; `Rule.DeckConstruction` Hare Apparent;
  `Rule.IfCardIsInOpeningHand` Leyline Axe
- `PlayerEffect.MayPlayALandFromAmongCardsInGraveyardsOnceEachTurn` Muldrotha, the Gravetide;
  `PlayerEffect.MayPlayExiledCardsAndMaySpendManaAsThoughAnyTypeToCast` Tinybones, Bauble Burglar
- `ProtectableColor.TheChosenColor` Mother of Runes; `SettableColor.Devoid` World Breaker;
  `ReplacementActionWouldEnter.ChooseACardName` + `Rule.ActivatedAbilityEffect` (2): Phyrexian Revoker, Pithing Needle

Also the static diff's `cost` family rows (Forge's cost descriptions and type spellings in `Cost$`, `ActivationLimit$`).

## Workspace

- Backend: `engine/cardc/gorgebe/ability_static.go`, `ability_replacement.go`, `ability_activated.go`, `cost.go`,
  `keyword_args.go`.
- Front end: `engine/cardc/mtgish/fe_static.go`, `fe_cost.go`, `fe_rule.go`, `fe_replace.go`, `fe_keyword.go`.
- Readers: `engine/effects/activation_params.go`, the static and cost readers under `engine/rules` and `engine/cards`.

## Done means

The shared list, plus `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestStatics|TestReplacements|TestKeywordArgs|TestActivated|TestCostText)$'`
and, when the front end changed, `make test-one PKG=./engine/cardc/mtgish RUN='^(TestStatic.*|TestRule.*|TestCost.*)$'`.

## Collisions

Runs beside CC-37 (library and zone verbs: `step_move.go`, `step_dig.go`). CC-38 (one-shot effects) runs after it and
reuses its layer-effect printing.
