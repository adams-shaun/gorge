# CR 200s: Parts of a card

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by the dirac kernel (two players, FDN and BLB pools). Seeded from the CR audit of 2026-10-20 at `36c2469`; kept current by every change that touches a rule. `inherits gorge` means the kernel copies gorge's behaviour, so gorge's row in `engine/rules/crcoverage/` decides it. See [README](README.md).

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 200.1 | n/a | - | - | descriptive list of card parts |
| 200.2 | follows | `dirac/chars.go#State.pt` `dirac/chars.go#State.cardAt` | - | characteristics derived per CR 613 / fixed order |
| 200.3 | follows | `dirac/chars.go#State.cardAt` | - | copies/tokens carry characteristics only |
| 201.1 | n/a | - | - | print layout |
| 201.2 | follows | `dirac/kop/table.go#CardDef.Name` `dirac/value_validagg.go#State.validAgg` `dirac/filters_gen.go#State.predPermGen` `dirac/filter_props.go#State.wordPerm2` `dirac/filter_props.go#State.wordCard2` | `TestValidAggCountHeads` `TestNamedotherLockstep` `TestFilterWordsAttackingEnchantedLockstep` | English Oracle names from the corpus; "differently named" (Count$Valid ...$DifferentCardNames): validAgg AggDistinctNames counts the distinct names among the battlefield matches; a named<X> filter naming another card, or the own name spelled with '_' (gorge wordNamed -> sharesName, a literal name: Rufus Shinra's Darkstar, Dragonstorm Forecaster's search, Gruff Triplets), is PredNamed against X's pool entry, -1 when no object here can have it; on the battlefield it reads nameCardAt (a layer-3 rename, CR 707.2; face-down none, CR 708.2); doesNotShareNameWith OtherYourBattlefield (no other permanent its controller controls shares the name; Yenna) and sharesNameWith Valid Permanent.Legendary (another legendary permanent shares the name; Key to the Side-Door) compare nameCardAt |
| 201.2a | follows | `dirac/filters_gen.go#State.nameCardAt` `dirac/filter_samename.go#State.sameNameT0` | `TestPauperkwMaelstromPulseLockstep` | gorge: face-down permanent shares its printed name (probe); "the target and all other permanents with the same name" (Maelstrom Pulse, PredSameNameT0) compares nameCardAt |
| 201.2b | n/a | - | - | gorge: face-down counted by printed name; dirac: no carrier |
| 201.2c | n/a | - | - | same root as 201.2a |
| 201.3 | n/a | - | - | SILENT: interchangeable names not modelled in play |
| 201.3a | n/a | - | - | SILENT; same as 201.3 |
| 201.3b | n/a | - | - | decklist alias lookup |
| 201.3c | n/a | - | - | print indicator |
| 201.4 | n/a | - | - | universe = registry cards, no tokens |
| 201.4a | n/a | - | - | ValidCards$ over printed face |
| 201.4b | n/a | - | - | only a split card's first half is nameable |
| 201.4c-201.4f | n/a | - | - | flip/back/meld/adventure names never offered |
| 201.4g | n/a | - | - | SILENT; no interchangeable-name model |
| 201.5 | follows | `dirac/resolve.go#State.objRef` | - | self-reference by name = this object (Self / RefSelf) |
| 201.5a-201.5c | n/a | - | - | settle: Gutter Grime token + second Gutter Grime scenario |
| 201.6 | n/a | - | - | flavour name has no game effect |
| 202.1 | n/a | - | - | layout |
| 202.1a | follows | `dirac/mana.go#State.pay` | - | - |
| 202.1b | follows | `dirac/kop/table.go#CardDef.Cost` | - | no mana cost |
| 202.2 | follows | `dirac/kop/table.go#CardDef.Colors` | - | - |
| 202.2a-202.2c | follows | `dirac/kop/filters.go#ColorMask` | - | - |
| 202.2d | follows | `dirac/kop/costs.go#ManaCost.Hyb` | - | hybrid/Phyrexian colours |
| 202.2e | follows | `dirac/kop/table.go#CardDef.Colors` | - | indicator read only when cost colourless; no corpus card has both |
| 202.2f | follows | `dirac/chars.go#State.colorsAt` | - | - |
| 202.3 | follows | `dirac/kop/costs.go#ManaCost.MV` | - | subrule gaps below |
| 202.3a | follows | `dirac/kop/costs.go#ManaCost.MV` | - | - |
| 202.3b | follows | `dirac/prep.go#prepare` `dirac/kop/costs.go#ManaCost.MV` | `TestBackFaceManaValueIsFront` `TestCopyOfBackFaceMV0` `TestFacesTransformLockstep` | a transforming DFC's back face has the front face's mana value (CR 712.8e); prep copies the front `total` into the back face's `pc.total`, so a CMC predicate reads 3 for Restoration of Eiganjo's back and 1 for Okiba Reckoner Raid's back; a copy's back face stays 0 (gorge mana_value.go's CopyFace branch) |
| 202.3c | n/a | - | - | SILENT; melded permanent MV 0 |
| 202.3d | n/a | - | - | split card off stack uses first half's MV (probe) |
| 202.3e | follows | `dirac/value_objectmv.go#State.objMV` `dirac/static_cda.go#State.stackXMV` `dirac/filters_gen.go#State.matchCardOne` | `TestMVOfStackSpellCountsX` | a spell's MV on the stack includes the chosen X; every Ref-based MV read (ExMVOf, targetMV, rememberedMV) goes through objMV |
| 202.3f | follows | `dirac/kop/costs.go#ManaCost.MV` | `TestTwobridMV` `TestTwobridLockstep` | a twobrid pip ({2/B}) counts 2 (`ManaCost.MV` for the `HybTwobrid` marker), as gorge's `Face.Cmc` does at this pin (`face.go` twobridManaValue): {2/B}{2/B}{2/B} is 6, Karn, Gilded Guardian 10; the Eliminate (cmcLE3) and Despark / Kin-Tree (cmcGE4) counters of `TestTwobridLockstep` read it |
| 202.3g | follows | `dirac/kop/costs.go#ManaCost.MV` | `TestPhyLockstep` | a Phyrexian pip counts 1 (`ManaCost.MV` with `HybPhyrexian`) |
| 202.4 | follows | `dirac/kop/costs.go#ManaCost.MV` | - | additional costs not in mana value |
| 203.1 | n/a | - | - | no game effect |
| 204.1 | n/a | - | - | layout |
| 204.2 | follows | `dirac/kop/table.go#CardDef.Colors` | - | - |
| 205.1 | follows | `dirac/chars_gen.go#State.hasSubGen` `dirac/static_layerwalk.go#lfxApply4` `dirac/featview.go#State.FeatViewTypes` | `TestRmcreatsubLockstep` `TestRmcreatsubMutation` | an attached override with RemoveCreatureTypes$ and no RemoveCardTypes$ (Noggle the Mind, Honest Work) drops the creature subtypes (genPrep.creatSub) and the all-creature-types mark in timestamp order, keeps card types and artifact/land subtypes (CR 205.1a), then adds its subtype (gorge applyTypeEffect) |
| 205.1a | follows | `dirac/chars_gen.go#State.hasSubGen` `dirac/static_layerwalk.go#lfxApply4` `dirac/kgen/lower_trigger.go#lOverride.keepSubtypes` | `TestL7KeptSubtypes` `TestCardTypeRemovalKeepsSubtypes` `TestAnimateRemoveCreatureTypesKeepsTreasure` | a RemoveCardTypes$ override keeps the subtypes of a card type it adds, by gorge's `effects.SubtypesKeptBy`; a kept creature set refuses by name; an Animate's RemoveCreatureTypes$ alone strips creature types only |
| 205.1b | follows | `dirac/chars.go#State.typesAt` | - | - |
| 205.2 | definition | - | - | heading |
| 205.2a | follows | `dirac/value_validagg.go#State.validAgg` `dirac/value_graveagg.go#State.graveAgg` `dirac/value_gravecounts.go#State.graveTypeAgg` `dirac/zone_exiledwithstatic.go#State.exileTypeAgg` | `TestValidAggCountHeads` `TestGravecountsLockstep` `TestGravecountsMutation` `TestExileTypesAgg` `TestExwStaticsLockstep` | Kindred is a card type in both folds (gorge, cardTypeWords; the graveyard fold no longer masks TKindred out, a Kindred Instant in a graveyard counts 2 card types, 0 permanent types); "card types among" (Count$Valid ...$CardTypes): validAgg AggCardTypes reads the matches' face card types (gorge o.Face.Types), not the layer-derived ones; the graveyard twin (Count$ValidGraveyard ...$CardTypes / $CardTypesPermanent, ExGraveAgg AggCardTypes / AggPermTypes: Tarmogoyf, Broodspinner, Matzalantli) reads the cards' printed types, the permanent set being CR 110.4's six; the exile twin (Count$ValidExile Card.ExiledWithSource$CardTypes, ExCountExile N 1: Keen-Eyed Curator) reads the linked exiled cards' printed types over both exile zones, in the static's context |
| 205.2b | follows | `dirac/kop/filters.go#TypeMask` `dirac/kgen/filter.go#lClause.prop` `dirac/filter_typesall.go#State.predPermCold` `dirac/filter_typesall.go#predCardCold` `dirac/kgen/lower_choosetype.go#lowerCtx.chooseCardType` `dirac/act_choosetype.go#chooseCardTypeOptions` `dirac/act_choosetype.go#State.chosenCardTypeHas` | `TestCycleForms` `TestTypesallFilterForms` `TestTypesAllPredicate` `TestTypesallLockstep` `TestPauperkwWindingWayLockstep` | Filter.Types is any-of; a type property beside a disjoint type head ("Land.Artifact", "Artifact.Creature": an object with BOTH types, gorge's type head AND wordType property) keeps the union in Types and carries the conjunction as PredTypesAll (typesall): derived types on the battlefield (an Ensoul Artifact-animated artifact is an artifact creature), printed off it; a conjunction beside another predicate refuses (one Pred slot); a mid-resolution "choose a card type" (ChooseType Type$ Card with ValidTypes$ card types: Winding Way) asks the listed types in gorge's vocabulary order, each option the label's canonical hash, and a Card.ChosenType filter then reads the chosen card type (printed types off the battlefield) |
| 205.2c | follows | `dirac/kop/table.go#CardDef.Types` | - | tokens/copies carry card types |
| 205.3 | definition | - | - | heading |
| 205.3a-205.3c | follows | `dirac/kop/table.go#CardDef.Subtypes` | - | - |
| 205.3d | n/a | - | - | SILENT; scripts usually filter by card type |
| 205.3e | partial | `dirac/kop/table.go#Table.Subtypes` | - | "Time Lord" split into Time + Lord; dirac inherits vocabulary |
| 205.3f | follows | `dirac/kop/table.go#Table.Subtypes` | - | Oracle subtypes from corpus |
| 205.3g-205.3k | follows | `dirac/kop/table.go#Table.Subtypes` | - | vocabulary from corpus |
| 205.3m | partial | `dirac/kop/table.go#Table.Subtypes` `dirac/filter_creaturetypes.go` | `TestAllCreatureTypesLeaveOtherSubtypes` | extra/split tokens vs CR list; dirac inherits; a filter-named creature type no pool card carries (gorge CreatureTypeWords: Spider-Ham's Jackal) is in the table (dirac/kgen knownSubtype). AddType$ ChosenType grants the source's chosen creature type (Adaptive Automaton, Lifecraft Engine; kop.Static.AddChosen, `dirac/act_choosetype.go#State.chosenGrant` `dirac/act_choosetype.go#State.outsideChosenHas`, `TestChoosetypeLockstep`): a type outside the matchup's subtypes compared across two different sources is loud (StallBoard). A Continuous AddAllCreatureTypes$ static sets the override's AllCreatureTypes in layer 4 (`dirac/static_layerwalk.go#lfxApply4`, `TestAllcreatLockstep`) |
| 205.3n-205.3r | n/a | - | - | planes/dungeon/battle subtype lists |
| 205.4 | definition | - | - | heading |
| 205.4a | follows | `dirac/kop/filters.go#TypeMask` | - | dirac TBasic/TLegendary bits |
| 205.4b | follows | `dirac/kop/filters.go#TypeMask` | - | - |
| 205.4c | follows | `dirac/kop/filters.go#TypeMask` | - | basic land |
| 205.4d | follows | `dirac/turn.go#State.legendRule` | - | legend rule |
| 205.4e | n/a | - | - | SILENT (probe); no FDN/BLB legendary instant/sorcery; |
| 205.4f | n/a | - | - | - |
| 205.4g | n/a | - | - | no snow in pools |
| 205.4h | n/a | - | - | no Archenemy |
| 206.1-206.5 | n/a | - | - | no game effect / errata note |
| 207.1-207.4 | n/a | `dirac/value_gravecounts.go#State.graveTypes` `dirac/kgen/value_filterheads.go#lowerCtx.metalcraftValue` `dirac/kgen/lower_condkeys.go#lowerCtx.nodeConditionZone` `dirac/kgen/lower_condkeys.go#lowerCtx.replCondKeys` | `TestGraveTypes` `TestValueHeadForms` `TestCondkeysLockstep` `TestCondkeysMutation` | text-box presentation; ability words (207.2c) carry no rules meaning: Valiant$ is lowered as a trigger qualifier (603.2 row; gorge); delirium's condition (four or more card types in your graveyard) is ExGraveTypes (trigger Delirium$, static/activation Condition, Count$Delirium, and a replacement's Delirium$ True); revolt's ability word gates a replacement as Revolt$ True, the turn's left-battlefield flag; metalcraft's (three or more artifacts you control) is ExCountAtLeast over Artifact.YouCtrl (Count$Metalcraft.<yes>.<no>, Galvanic Blast) |
| 207.5 | n/a | - | - | Cryptic Spires circle |
| 208.1 | follows | `dirac/chars.go#State.pt` | - | - |
| 208.2 | follows | `dirac/chars.go#State.pt` `dirac/kgen/lower_trigger.go#lowerCtx.continuous` | `TestFilterPredicateAndTriggerParamForms` | *-P/T on the battlefield via CDA; a CDA that sets one half (*/4) keeps the other printed (gorge CDASetPT) |
| 208.2a | follows | `dirac/filters_gen.go#State.matchCardOne` `dirac/static_cda.go#State.cardCDAPT` | - | a filter's P/T predicate reads the CDA in every zone |
| 208.2b | n/a | - | - | not line-checked |
| 208.3 | n/a | - | - | printed-PT noncreature (Vehicle) treated as having P/T |
| 208.3a | n/a | - | - | - |
| 208.4-208.4b | follows | `dirac/chars.go#State.basePT` | - | - |
| 208.5 | follows | `dirac/kop/table.go#CardDef.Power` | - | - |
| 209.1 | follows | `dirac/zones.go#State.enter` | - | - |
| 209.2 | follows | `dirac/legal.go#State.canActivate` `dirac/mana_window.go#State.payAbilityCosts` | - | - |
| 210.1 | n/a | - | - | - |
| 211.1 | n/a | - | - | no Vanguard |
| 212.1 | n/a | - | - | no Vanguard |
| 213.1-213.1g | n/a | - | - | no game effect |
