# kgen parameter census: FDN

Every (API or trigger/static/replacement mode, parameter) pair the 286 FDN
pool cards use (281 non-basic), with the cards that use it and how
`dirac/kgen` lowers it to `dirac/kop`. Regenerate the census itself with
`go run ./dirac/cmd/kgen -census`; the per-card lowered form is
`go run ./dirac/cmd/kgen -lir`. Descriptive and AI-hint params (SpellDescription,
StackDescription, TgtPrompt, AILogic, AITgts, AIPreference, IsCurse, *Desc,
TriggerDescription, Description, KeywordLine, Secondary, Image, Ultimate,
MayPlayText, Keyword tags) are listed nowhere: the lowering ignores them.
Every other param is either read here or refuses the card ("unread params"),
so this table is complete by construction.

## Shared grammar

- **Filters** (ValidTgts$, ValidCards$, ValidCard$, Affected$, ChangeType$,
  ChangeValid$, SacValid$, IsPresent$, ConditionPresent$, Choices$, ...):
  comma alternatives become a `kop.Filter` chain through `Filter.Or`; each
  alternative is `Head.prop+prop`. Heads: card types (Types any-of; Card =
  any; Permanent = the permanent types), a subtype (Filter.Sub), Emblem
  (F2Emblem), Player/Opponent/You/Any (target kinds). Props: YouCtrl/OppCtrl/
  YouDontCtrl (Ctrl), YouOwn/OppOwn/YouDontOwn (Owner), Self/Other/token/
  !token/attacking/blocking/tapped/untapped (FilterFlags), withFlying,
  withoutFlying, kicked, wasCastByYou, EquippedBy, EnchantedBy,
  ThisTurnEntered, ChosenType, ChosenColor, IsRemembered, !IsRemembered,
  nonChosenCard, AttachedTo ParentTarget, targetedBy, named<self>, Basic
  (FilterFlags2), nonType (NotTypes), nonSubtype (NotSub), a subtype (Sub),
  powerGE/LE n, cmcLE/GE/EQ n, cmcLE<SVar> (PredCMCLEV + PredV),
  counters_GEn_KIND (PredCountersGE).
- **Amounts** (NumDmg$, NumCards$, LifeAmount$, CounterNum$, TokenAmount$,
  NumAtt$/NumDef$, Amount$, ChangeNum$, DigNum$, X SVars): literal or +/-
  literal -> `kop.Const`; X -> VX/VNegX; Count$Valid F -> VCountFilter;
  TriggerCount$DamageAmount/LifeAmount -> VTriggerAmount; CardPower of self /
  target -> VSelfPower/VTargetPower; Targeted$CardToughness ->
  VTargetToughness; CardCounters.K of self -> VCountersSelf;
  LifeYouGainedThisTurn -> VLifeGainedTurn; everything else -> VExpr into
  `Table.Exprs` (ExCountGraveyard, ExSumPower, ExDistinctCMC, ExKicked(a,b),
  ExMorbid(a,b), ExAttackedThisTurn (Raid), ExLifeOf, ExLibrarySizeOf,
  ExStartingLife(+n), ExOppLifeLostThisTurn, ExCountersOn, ExCreaturesDied,
  ExRememberedCount, ExExcessDamage, ExLastLifeLost, ExTokensCreated, with
  Minus/Plus/Twice/DivideEvenlyDown as Add/Mul/Div).
- **Conditions** (CheckSVar$/SVarCompare$, IsPresent$/PresentCompare$,
  ConditionPresent$/ConditionCompare$/ConditionDefined$,
  ConditionCheckSVar$/ConditionSVarCompare$, Threshold$, Condition$
  Threshold/PlayerTurn, Kicked via Count$Kicked) -> `Table.Conds`; on a
  trigger Trigger.Cond (intervening if; NoResolvingCheck$ -> AfCheckOnTrigger),
  on a static Static.Cond2, on a replacement Repl.Cond, on an effect node an
  OpSkipUnless guarding that node only (Forge still runs the node's Sub).
- **Keywords**: Flying, First Strike, Double Strike, Deathtouch, Lifelink,
  Vigilance, Trample, Reach, Haste, Defender, Menace, Hexproof, Hexproof:Instant
  (KwHexproofFromInstants), Protection from everything -> KeywordMask; Flash ->
  KwFlash + TimingInstant; Kicker -> KwKicker + CardDef.Kicker; Flashback ->
  KwFlashback + CardDef.AltCost; AlternateAdditionalCost -> SpellCost +
  AltAddCost; MayEffectFromOpeningHand -> CardDef.Leyline. Gorge's
  engine/cards expansions are lowered from their expanded lines and folded
  back where kop has a native form: Equip -> CardDef.EquipCost, Enchant ->
  CardDef.Enchant, Ward -> KwWard + WardN/WardLife, Prowess -> KwProwess;
  Affinity (for Cats) stays a ReduceCost static; etbCounter / ETBReplacement
  are R: lines (below).
- **Defined$** -> the instruction's Obj: Self->RefSelf, You->RefController, Opponent/Player.Opponent->RefEachOpponent, Player->RefEachPlayer, Targeted/ParentTarget->RefTargetN, TargetedController->RefTargetController, TriggeredCard*/TriggeredNewCardLKICopy->RefTriggerObj, TriggeredSourceLKICopy->RefTriggerSource, TriggeredCardController/TriggeredActivator->RefTriggerPlayer, Equipped->RefAttached, Remembered->RefRemembered, DelayTriggerRememberedLKI->RefDelayedRemembered, ReplacedCard->RefReplaced, ChosenCard->RefChosen, OriginalHost->RefHost.
- **Costs** (Cost$, UnlessCost$): mana symbols (hybrid REFUSED), T, Sac<n/CARDNAME>,
  Sac<n/filter>, Discard<n/Card>, AddCounter/SubCounter<n/LOYALTY>,
  SubCounter<n/KIND>, PayLife<n>, tapXType<n/type>, tapXType<1/OriginalHost>
  (CostTapHost).

## Parameters


### api:AddTurn

| param | cards | lowering |
|---|---|---|
| NumTurns | 1: Temporal Manipulation | OpAddTurn Obj=RefController (1 only) |

### api:Animate

| param | cards | lowering |
|---|---|---|
| AddAllCreatureTypes | 1: Soulstone Sanctuary | override AllCreatureTypes |
| ConditionPresent | 1: Kellan, Planar Trailblazer | OpSkipUnless + CondPresent(Self+subtype) |
| Cost | 3: Kellan, Planar Trailblazer; Mild-Mannered Librarian; Soulstone Sanctuary | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 2: Fake Your Own Death; Soulstone Sanctuary | Obj ref (see Defined$ in Shared grammar) |
| Duration | 3: Kellan, Planar Trailblazer; Mild-Mannered Librarian; Soulstone Sanctuary | OpOverride A: DurEOT (absent) / DurPermanent |
| GameActivationLimit | 1: Mild-Mannered Librarian | AfOncePerGame |
| Keywords | 2: Kellan, Planar Trailblazer; Soulstone Sanctuary | override AddKw |
| Power | 2: Kellan, Planar Trailblazer; Soulstone Sanctuary | override HasPT SetP |
| RemoveCreatureTypes | 2: Kellan, Planar Trailblazer; Mild-Mannered Librarian | override HasSub (subtypes replaced) |
| SubAbility | 1: Mild-Mannered Librarian | the next node of the chain (lowered in order) |
| Toughness | 2: Kellan, Planar Trailblazer; Soulstone Sanctuary | override SetT |
| Triggers | 2: Fake Your Own Death; Kellan, Planar Trailblazer | override Grant: the SVar trigger lowered as a latent ability (AfLatent) |
| Types | 3: Kellan, Planar Trailblazer; Mild-Mannered Librarian; Soulstone Sanctuary | override record: card types -> AddTypes, subtypes -> SetSub (RemoveCreatureTypes) or AddSub |

### api:Attach

| param | cards | lowering |
|---|---|---|
| Cost | 8: Adventuring Gear; Celestial Armor; Embercleave; Fishing Pole; Goldvein Pick (+3) | Equip cost -> CardDef.EquipCost |
| Object | 4: Blanchwood Armor; Imprisoned in the Moon; Twinblade Blessing; Witness Protection | Self only; Equip AB -> CardDef.EquipCost (folded), Aura SP -> CardDef.Enchant (folded), ETB attach -> OpAttach |
| SorcerySpeed | 8: Adventuring Gear; Celestial Armor; Embercleave; Fishing Pole; Goldvein Pick (+3) | CostSorcery |
| SubAbility | 1: Celestial Armor | the next node of the chain (lowered in order) |
| ValidTgts | 12: Adventuring Gear; Blanchwood Armor; Celestial Armor; Embercleave; Fishing Pole (+7) | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:ChangeZone

| param | cards | lowering |
|---|---|---|
| ActivationZone | 1: Reassembling Skeleton | Ability.ActZone (Graveyard) |
| AlternativeDecider | 1: Uncharted Voyage | TargetedOwner + DestinationAlternative Library + LibraryPositionAlternative -1 -> OpToLibraryChoice |
| ChangeNum | 7: Burnished Hart; Fiend Artisan; Grim Tutor; Grow from the Ashes; Inspiration from Beyond (+2) | OpSearchEx/OpChooseMove A |
| ChangeType | 11: Burnished Hart; Bushwhack; Campus Guide; Evolving Wilds; Fiend Artisan (+6) | hidden-zone choice: OpSearchEx (library) / OpChooseMove (own graveyard) with F |
| Cost | 7: Abyssal Harvester; Burnished Hart; Evolving Wilds; Fiend Artisan; Flamewake Phoenix (+2) | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 9: Curator of Destinies; Fake Your Own Death; Flamewake Phoenix; Infernal Vessel; Kykar, Zephyr Awakener (+4) | Obj ref (see Defined$ in Shared grammar) |
| Destination | 40: Abyssal Harvester; Alesha, Who Laughs at Fate; Ambush Wolf; Banishing Light; Bigfin Bouncer (+35) | OpExile/OpExileEx, OpBounce, OpReturn/OpReturnEx (C = zone\\|MoveFlags<<3), OpToLibrary (C 0 top/1 bottom), OpToLibraryChoice, OpSearchEx, OpChooseMove; Graveyard REFUSED |
| DestinationAlternative | 1: Uncharted Voyage | see AlternativeDecider |
| Duration | 1: Banishing Light | UntilHostLeavesPlay -> OpExileUntilLeaves |
| GainControl | 1: Nine-Lives Familiar | MvUnderYourControl |
| Hidden | 2: Inspiration from Beyond; Progenitus | consumed with ChangeType (the hidden-zone choice) |
| LibraryPosition | 2: Campus Guide; Condemn | 0 top / -1 bottom (OpToLibrary C); with a library search: SrThenTop |
| LibraryPositionAlternative | 1: Uncharted Voyage | see AlternativeDecider |
| Mandatory | 3: Grim Tutor; Inspiration from Beyond; Rune-Scarred Demon | SrMandatory |
| Origin | 40: Abyssal Harvester; Alesha, Who Laughs at Fate; Ambush Wolf; Banishing Light; Bigfin Bouncer (+35) | source zone: Battlefield->Exile/Bounce/ToLibrary, Graveyard->Return/Exile, Library->SearchEx, Exile->Return |
| RememberChanged | 3: Abyssal Harvester; Kykar, Zephyr Awakener; Soul-Shackled Zombie | MvRemember / DigRemember: the moved cards become RefRemembered |
| Reveal | 1: Progenitus | REFUSED (Progenitus only) |
| Shuffle | 1: Progenitus | REFUSED (Progenitus only) |
| ShuffleNonMandatory | 2: Campus Guide; Solemn Simulacrum | read, lowers to nothing: the search's shuffle is mandatory (gorge; SrShuffleOptional retired) |
| SorcerySpeed | 1: Fiend Artisan | CostSorcery |
| StaticEffect | 2: Infernal Vessel; Valkyrie's Call | the returned object gets an override record (AddSub, AddKw) for DurPermanent: OpOverride after OpReturnEx |
| SubAbility | 9: Abyssal Harvester; Condemn; Fake Your Own Death; Grim Tutor; Kykar, Zephyr Awakener (+4) | the next node of the chain (lowered in order) |
| Tapped | 5: Burnished Hart; Evolving Wilds; Fake Your Own Death; Reassembling Skeleton; Solemn Simulacrum | MvTapped / SrTapped |
| TargetMax | 6: Ambush Wolf; Fiery Annihilation; Macabre Waltz; Mischievous Pup; Run Away Together (+1) | TargetSpec Max (with TargetMin) |
| TargetMin | 6: Ambush Wolf; Fiery Annihilation; Macabre Waltz; Mischievous Pup; Run Away Together (+1) | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| TargetsWithDifferentControllers | 1: Run Away Together | TgfDiffController |
| TargetsWithSameController | 1: Soul-Shackled Zombie | TgfSameController |
| ValidTgts | 20: Abyssal Harvester; Alesha, Who Laughs at Fate; Ambush Wolf; Banishing Light; Bigfin Bouncer (+15) | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |
| WithCountersAmount | 2: Infernal Vessel; Nine-Lives Familiar | A = counters |
| WithCountersType | 4: Infernal Vessel; Nine-Lives Familiar; Tinybones, Bauble Burglar; Valkyrie's Call | OpExileEx/OpReturnEx B.N = counter kind |

### api:ChangeZoneAll

| param | cards | lowering |
|---|---|---|
| ChangeType | 6: Abyssal Harvester; Aetherize; Angel of Finality; Curator of Destinies; Raise the Past (+1) | F (Filter.Zone = Origin) |
| Destination | 6: Abyssal Harvester; Aetherize; Angel of Finality; Curator of Destinies; Raise the Past (+1) | C zone |
| GainControl | 1: Rise of the Dark Realms | OpChangeZoneAllEx MvUnderYourControl |
| Origin | 6: Abyssal Harvester; Aetherize; Angel of Finality; Curator of Destinies; Raise the Past (+1) | F.Zone |
| SubAbility | 1: Abyssal Harvester | the next node of the chain (lowered in order) |
| ValidTgts | 1: Angel of Finality | a player target; OpChangeZoneAllEx Obj restricts to that player's cards |

### api:Charm

| param | cards | lowering |
|---|---|---|
| Choices | 9: Abrade; Apothecary Stomper; Bushwhack; Goblin Surprise; Kykar, Zephyr Awakener (+4) | Ability.NModes/ModesPick=1; each mode a block [OpCharm C=mode A=len]; mode targets carry TargetSpec.Mode |

### api:ChooseCard

| param | cards | lowering |
|---|---|---|
| ChoiceZone | 2: Chandra, Flameshaper; Strongbox Raider | F.Zone |
| Choices | 3: Chandra, Flameshaper; Liliana, Dreadhorde General; Strongbox Raider | OpChooseCard F (zone = ChoiceZone) |
| ChooseEach | 1: Liliana, Dreadhorde General | Liliana -9: OpChooseEachType Obj=RefEachOpponent; the following SacrificeAll takes F2NotChosenCard |
| ControlledByPlayer | 1: Liliana, Dreadhorde General | Chooser (with ChooseEach) |
| Cost | 1: Liliana, Dreadhorde General | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 1: Liliana, Dreadhorde General | Obj ref (see Defined$ in Shared grammar) |
| ForgetOtherRemembered | 2: Chandra, Flameshaper; Strongbox Raider | consumed: the unchosen remembered cards are only cleaned up afterwards |
| Mandatory | 3: Chandra, Flameshaper; Liliana, Dreadhorde General; Strongbox Raider | required (an optional choice REFUSED) |
| Planeswalker | 1: Liliana, Dreadhorde General | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| Reveal | 1: Liliana, Dreadhorde General | consumed (2-player: no rules effect) |
| SubAbility | 3: Chandra, Flameshaper; Liliana, Dreadhorde General; Strongbox Raider | the next node of the chain (lowered in order) |

### api:ChooseColor

| param | cards | lowering |
|---|---|---|
| Defined | 1: Heraldic Banner | You: OpChooseColor in an ETB replacement program |

### api:ChooseType

| param | cards | lowering |
|---|---|---|
| Defined | 2: Banner of Kinship; Secluded Courtyard | Obj ref (see Defined$ in Shared grammar) |
| SubAbility | 1: Banner of Kinship | the next node of the chain (lowered in order) |
| Type | 2: Banner of Kinship; Secluded Courtyard | Creature: OpChooseType in an ETB replacement program (options: the matchup subtype table) |

### api:Cleanup

| param | cards | lowering |
|---|---|---|
| ClearChosenCard | 3: Chandra, Flameshaper; Liliana, Dreadhorde General; Strongbox Raider | OpClearChosen |
| ClearRemembered | 8: Abyssal Harvester; Etali, Primal Storm; Faebloom Trick; Fishing Pole; Kellan, Planar Trailblazer (+3) | OpClearRemembered |
| SubAbility | 1: Sphinx's Tutelage | the next node of the chain (lowered in order) |

### api:CopyPermanent

| param | cards | lowering |
|---|---|---|
| AddKeywords | 2: Chandra, Flameshaper; Electroduplicate | CpHaste (Haste only) |
| AddTypes | 1: Abyssal Harvester | B.N subtype index+1 <<8 |
| AtEOTTrig | 2: Chandra, Flameshaper; Electroduplicate | Sacrifice -> CpSacAtEnd |
| Controller | 1: Abyssal Harvester | You only |
| Cost | 1: Chandra, Flameshaper | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 2: Abyssal Harvester; Homunculus Horde | Obj ref (see Defined$ in Shared grammar) |
| Planeswalker | 1: Chandra, Flameshaper | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| RememberTokens | 1: Abyssal Harvester | CpRemember |
| SubAbility | 1: Abyssal Harvester | the next node of the chain (lowered in order) |
| ValidTgts | 4: Chandra, Flameshaper; Electroduplicate; Extravagant Replication; Self-Reflection | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:CopySpellAbility

| param | cards | lowering |
|---|---|---|
| Amount | 1: Thousand-Year Storm | REFUSED (Thousand-Year Storm: spell copies) |
| Defined | 1: Thousand-Year Storm | REFUSED |
| MayChooseTarget | 1: Thousand-Year Storm | REFUSED |

### api:Counter

| param | cards | lowering |
|---|---|---|
| SubAbility | 2: An Offer You Can't Refuse; Refute | the next node of the chain (lowered in order) |
| TargetType | 3: An Offer You Can't Refuse; Essence Scatter; Refute | Spell -> TgtSpell |
| ValidTgts | 3: An Offer You Can't Refuse; Essence Scatter; Refute | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:DamageAll

| param | cards | lowering |
|---|---|---|
| NumDmg | 2: Seismic Rupture; Slagstorm | A |
| ValidCards | 2: Seismic Rupture; Slagstorm | OpDamageAll F |
| ValidPlayers | 1: Slagstorm | Player only (Slagstorm) -> OpDealDamage Obj=RefEachPlayer |

### api:DealDamage

| param | cards | lowering |
|---|---|---|
| ConditionCheckSVar | 1: Hidetsugu's Second Rite | OpSkipUnless + CondValue(A=value, Op, B) |
| ConditionSVarCompare | 1: Hidetsugu's Second Rite | CondValue Op/B |
| Cost | 3: Chandra, Flameshaper; Fanatical Firebrand; Heartfire Immolator | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| DamageMap | 1: Drakuseth, Maw of Flames | DmgMapped on each DealDamageEx until OpResolveDamageMap |
| DamageSource | 2: Bite Down; Felling Blow | ParentTarget with amount = its power -> OpBite Obj=source C=target ref |
| Defined | 4: Boltwave; Firebrand Archer; Firespitter Whelp; Spitfire Lagac | Obj ref (see Defined$ in Shared grammar) |
| DividedAsYouChoose | 1: Chandra, Flameshaper | TargetSpec.Divide + OpDealDamageDivided |
| ExcessSVar | 1: Goblin Negotiation | DmgTrackExcess; the excess is ExExcessDamage |
| ExcessSVarCondition | 1: Goblin Negotiation | Card.targetedBy only |
| NumDmg | 18: Abrade; Bite Down; Boltwave; Burst Lightning; Chandra, Flameshaper (+13) | A (value) |
| Planeswalker | 1: Chandra, Flameshaper | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| ReplaceDyingDefined | 1: Fiery Annihilation | Targeted -> DmgExileIfDiesThisTurn |
| SubAbility | 5: Drakuseth, Maw of Flames; Fiery Annihilation; Goblin Negotiation; Incinerating Blast; Joust Through | the next node of the chain (lowered in order) |
| TargetMax | 2: Chandra, Flameshaper; Drakuseth, Maw of Flames | TargetSpec Max (with TargetMin) |
| TargetMin | 2: Chandra, Flameshaper; Drakuseth, Maw of Flames | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| TargetUnique | 1: Drakuseth, Maw of Flames | TgfDistinct |
| ValidTgts | 14: Abrade; Bite Down; Burst Lightning; Chandra, Flameshaper; Drakuseth, Maw of Flames (+9) | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:DelayedTrigger

| param | cards | lowering |
|---|---|---|
| Execute | 2: Kykar, Zephyr Awakener; Nine-Lives Familiar | the latent ability C (AfLatent) |
| Phase | 2: Kykar, Zephyr Awakener; Nine-Lives Familiar | End of Turn only: OpDelayedProgram (next end step) |
| RememberObjects | 2: Kykar, Zephyr Awakener; Nine-Lives Familiar | Obj (RefRemembered / RefTriggerObj) -> RefDelayedRemembered inside |
| SubAbility | 1: Kykar, Zephyr Awakener | the next node of the chain (lowered in order) |
| ValidPlayer | 1: Nine-Lives Familiar | Player only (any player's end step) |

### api:Destroy

| param | cards | lowering |
|---|---|---|
| Cost | 2: Cathar Commando; Vivien Reid | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Planeswalker | 1: Vivien Reid | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| SubAbility | 2: Bake into a Pie; Stroke of Midnight | the next node of the chain (lowered in order) |
| ValidTgts | 11: Abrade; Bake into a Pie; Broken Wings; Cathar Commando; Hero's Downfall (+6) | OpDestroy Obj=target |

### api:DestroyAll

| param | cards | lowering |
|---|---|---|
| ValidCards | 1: Day of Judgment | OpDestroyAll F |

### api:Dig

| param | cards | lowering |
|---|---|---|
| ChangeNum | 9: Chandra, Flameshaper; Etali, Primal Storm; Genesis Wave; Gutless Plunderer; Kellan, Planar Trailblazer (+4) | B: N, All -> -1, Any -> -2 |
| ChangeValid | 5: Genesis Wave; Gutless Plunderer; Loot, Exuberant Explorer; Squad Rallier; Vivien Reid | F (library zone) |
| Cost | 3: Loot, Exuberant Explorer; Squad Rallier; Vivien Reid | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 4: Chandra, Flameshaper; Etali, Primal Storm; Kellan, Planar Trailblazer; Strongbox Raider | Obj: You / Player (each player: Etali) |
| DestinationZone | 8: Chandra, Flameshaper; Etali, Primal Storm; Genesis Wave; Gutless Plunderer; Kellan, Planar Trailblazer (+3) | C&7 (default Hand) |
| DestinationZone2 | 3: Genesis Wave; Gutless Plunderer; Squad Rallier | rest zone (C>>3)&7 (default Library bottom) |
| DigNum | 9: Chandra, Flameshaper; Etali, Primal Storm; Genesis Wave; Gutless Plunderer; Kellan, Planar Trailblazer (+4) | A |
| ForceRevealToController | 2: Squad Rallier; Vivien Reid | DigRevealToCtrl |
| LibraryPosition | 2: Gutless Plunderer; Squad Rallier | 0 with DestinationZone Library -> DigTakeToTop; -1 rest -> bottom (default) |
| Optional | 4: Gutless Plunderer; Loot, Exuberant Explorer; Squad Rallier; Vivien Reid | DigOptional |
| Planeswalker | 1: Vivien Reid | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| RememberChanged | 4: Chandra, Flameshaper; Etali, Primal Storm; Kellan, Planar Trailblazer; Strongbox Raider | MvRemember / DigRemember: the moved cards become RefRemembered |
| RestRandomOrder | 2: Loot, Exuberant Explorer; Squad Rallier | DigRandomOrder |
| Reveal | 1: Genesis Wave | DigReveal |
| SubAbility | 4: Chandra, Flameshaper; Etali, Primal Storm; Kellan, Planar Trailblazer; Strongbox Raider | the next node of the chain (lowered in order) |

### api:DigUntil

| param | cards | lowering |
|---|---|---|
| Amount | 1: Consuming Aberration | A |
| Defined | 1: Consuming Aberration | Obj ref (see Defined$ in Shared grammar) |
| FoundDestination | 1: Spinner of Souls | C&7; absent -> DuFoundWithRevealed |
| RevealRandomOrder | 1: Spinner of Souls | DuRandomOrder |
| RevealedDestination | 2: Consuming Aberration; Spinner of Souls | (C>>3)&7 |
| RevealedLibraryPosition | 1: Spinner of Souls | -1 -> DuRestBottom |
| Valid | 2: Consuming Aberration; Spinner of Souls | F |

### api:Discard

| param | cards | lowering |
|---|---|---|
| Cost | 1: Tinybones, Bauble Burglar | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 10: Arbiter of Woe; Burglar Rat; Dreadwing Scavenger; Icewind Elemental; Kiora, the Rising Tide (+5) | Obj ref (see Defined$ in Shared grammar) |
| DiscardValid | 1: Pilfer | F (hand) |
| NumCards | 9: Dreadwing Scavenger; Icewind Elemental; Kiora, the Rising Tide; Macabre Waltz; Pilfer (+4) | A |
| SorcerySpeed | 1: Tinybones, Bauble Burglar | CostSorcery |
| SubAbility | 1: Arbiter of Woe | the next node of the chain (lowered in order) |
| ValidTgts | 2: Pilfer; Seeker's Folly | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:Draw

| param | cards | lowering |
|---|---|---|
| Cost | 6: Ravenous Amulet; Spectral Sailor; Sphinx's Tutelage; Strix Lookout; Thrill of Possibility (+1) | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 8: Garruk's Uprising; Phyrexian Arena; Scrawling Crawler; Solemn Simulacrum; Spectral Sailor (+3) | Obj ref (see Defined$ in Shared grammar) |
| NumCards | 13: Arcane Epiphany; Inspiring Call; Kiora, the Rising Tide; Lunar Insight; Niv-Mizzet, Visionary (+8) | A (default 1) |
| SorcerySpeed | 1: Ravenous Amulet | CostSorcery |
| SubAbility | 13: Arbiter of Woe; Dreadwing Scavenger; Icewind Elemental; Inspiring Call; Kaito, Cunning Infiltrator (+8) | the next node of the chain (lowered in order) |
| UnlessCost | 1: Incinerating Blast | OpUnlessPaid (payer may pay; if paid skip the node); costs: mana, Discard<1/Card>, Sac<1/filter>, PayLife<N> |
| UnlessPayer | 1: Incinerating Blast | OpUnlessPaid Obj (the payer ref) |
| UnlessSwitched | 1: Incinerating Blast | OpIfPaid (the node runs only if paid) |

### api:Effect

| param | cards | lowering |
|---|---|---|
| Cost | 4: Kaito, Cunning Infiltrator; Rogue's Passage; Vivien Reid; Zul Ashur, Lich Lord | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Duration | 3: Kaito, Cunning Infiltrator; Strongbox Raider; Vivien Reid | absent EOT; UntilTheEndOfYourNextTurn -> DurYourNextTurn; Permanent -> emblem |
| ExileOnMoved | 4: Kaito, Cunning Infiltrator; Rogue's Passage; Vampire Gourmand; Zul Ashur, Lich Lord | consumed: the EOT grant ends when the object leaves (per-object flags) |
| ForgetOnMoved | 3: Chandra, Flameshaper; Kellan, Planar Trailblazer; Strongbox Raider | consumed (same) |
| Name | 2: Kaito, Cunning Infiltrator; Vivien Reid | emblem (descriptive) |
| Planeswalker | 2: Kaito, Cunning Infiltrator; Vivien Reid | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| RememberObjects | 7: Chandra, Flameshaper; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer; Rogue's Passage; Strongbox Raider (+2) | the Obj of the produced op |
| Stackable | 1: Kaito, Cunning Infiltrator | consumed |
| StaticAbilities | 8: Chandra, Flameshaper; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer; Rogue's Passage; Strongbox Raider (+3) | CantBlockBy(IsRemembered) -> OpGrantKeyword KwUnblockable (EOT); Continuous MayPlay -> OpMayPlay; Duration Permanent -> emblem (OpCreateEmblem) |
| SubAbility | 4: Chandra, Flameshaper; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer; Strongbox Raider | the next node of the chain (lowered in order) |
| TargetMax | 1: Kaito, Cunning Infiltrator | TargetSpec Max (with TargetMin) |
| TargetMin | 1: Kaito, Cunning Infiltrator | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| TgtZone | 1: Zul Ashur, Lich Lord | TargetSpec Kind TgtCard with Filter.Zone |
| Triggers | 1: Kaito, Cunning Infiltrator | emblem trigger: a latent ability moved into Table.Emblems |
| ValidTgts | 3: Kaito, Cunning Infiltrator; Rogue's Passage; Zul Ashur, Lich Lord | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:Fight

| param | cards | lowering |
|---|---|---|
| Defined | 2: Affectionate Indrik; Bushwhack | the fighter (TriggeredCardLKICopy on an ETB -> RefSelf); OpFight Obj, C = target ref |
| ValidTgts | 2: Affectionate Indrik; Bushwhack | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:GainControl

| param | cards | lowering |
|---|---|---|
| AddKWs | 1: Involuntary Employment | Haste only, on an until-EOT steal (part of OpGainControlEOT; OpGainControl GcHasteEOT); refused with another duration |
| LoseControl | 1: Involuntary Employment | EOT with Untap + Haste -> OpGainControlEOT; otherwise OpGainControl A.N = DurEOT, absent DurPermanent, LeavesPlay DurWhileSource, LeavesPlay,LoseControl DurWhileControlSource, UntilTheEndOfYourNextTurn DurYourNextTurn (CR 613 layer 2); any other list refuses |
| SubAbility | 1: Involuntary Employment | the next node of the chain (lowered in order) |
| Untap | 1: Involuntary Employment | part of OpGainControlEOT |
| ValidTgts | 1: Involuntary Employment | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:GainLife

| param | cards | lowering |
|---|---|---|
| ConditionCompare | 1: Scavenging Ooze | Cond Op/A (default GE 1) |
| ConditionDefined | 2: Scavenging Ooze; Soul-Shackled Zombie | CondRefMatches Ref |
| ConditionPresent | 2: Scavenging Ooze; Soul-Shackled Zombie | OpSkipUnless guarding the node; Conds: CondPresent (count of F) or CondRefMatches with ConditionDefined |
| Defined | 12: Arbiter of Woe; Authority of the Consuls; Bloodthirsty Conqueror; Condemn; Dwynen, Gilt-Leaf Daen (+7) | Obj ref (see Defined$ in Shared grammar) |
| LifeAmount | 26: Apothecary Stomper; Arbiter of Woe; Authority of the Consuls; Bloodfell Caves; Bloodthirsty Conqueror (+21) | A; AFLifeLost (Number$0) -> ExLastLifeLost |
| SubAbility | 2: Soul-Shackled Zombie; Tatyova, Benthic Druid | the next node of the chain (lowered in order) |

### api:GenericChoice

| param | cards | lowering |
|---|---|---|
| Choices | 1: Perforating Artist | OpChooseBranch A=n + OpBranch blocks |
| Defined | 1: Perforating Artist | OpRepeatEachPlayer over the choosers |
| FallbackAbility | 1: Perforating Artist | the last OpBranch (index n) |
| TempRemember | 1: Perforating Artist | Chooser: Remembered inside the branches -> RefEach |

### api:ImmediateTrigger

| param | cards | lowering |
|---|---|---|
| Execute | 1: Faebloom Trick | latent ability C (its own targets) |
| SubAbility | 1: Faebloom Trick | the next node of the chain (lowered in order) |
| TriggerAmount | 1: Faebloom Trick | A (Remembered$Amount/DivideEvenlyDown.2 -> ExTokensCreated /2) |

### api:LoseLife

| param | cards | lowering |
|---|---|---|
| ConditionDefined | 1: Soul-Shackled Zombie | CondRefMatches Ref |
| ConditionPresent | 1: Soul-Shackled Zombie | OpSkipUnless guarding the node; Conds: CondPresent (count of F) or CondRefMatches with ConditionDefined |
| Cost | 3: Lathril, Blade of the Elves; Midnight Snack; Ravenous Amulet | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 11: Arbiter of Woe; Exsanguinate; Lathril, Blade of the Elves; Marauding Blight-Priest; Painful Quandary (+6) | Obj ref (see Defined$ in Shared grammar) |
| LifeAmount | 15: Arbiter of Woe; Exsanguinate; Grim Tutor; Lathril, Blade of the Elves; Marauding Blight-Priest (+10) | A |
| SubAbility | 6: Arbiter of Woe; Exsanguinate; Lathril, Blade of the Elves; Sanguine Syphoner; Soul-Shackled Zombie (+1) | the next node of the chain (lowered in order) |
| UnlessCost | 2: Painful Quandary; Perforating Artist | OpUnlessPaid (payer may pay; if paid skip the node); costs: mana, Discard<1/Card>, Sac<1/filter>, PayLife<N> |
| UnlessPayer | 2: Painful Quandary; Perforating Artist | OpUnlessPaid Obj (the payer ref) |
| ValidTgts | 2: Midnight Snack; Vengeful Bloodwitch | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:Mana

| param | cards | lowering |
|---|---|---|
| Amount | 7: Chandra, Flameshaper; Elvish Archdruid; Forest; Island; Mountain (+2) | ManaN, or AfManaDynamic + ManaV |
| Cost | 27: Bloodfell Caves; Bloom Tender; Blossoming Sands; Chandra, Flameshaper; Dismal Backwater (+22) | a loyalty cost makes it an AbActivated with OpAddMana (CR 605.1a) |
| Planeswalker | 1: Chandra, Flameshaper | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| Produced | 27: Bloodfell Caves; Bloom Tender; Blossoming Sands; Chandra, Flameshaper; Dismal Backwater (+22) | G/W/... -> ManaColors; Combo X Y -> mask; Any -> ColAny; Chosen -> AfManaChosen; Special EachColorAmong REFUSED |
| RestrictValid | 2: Giada, Font of Hope; Secluded Courtyard | Spell.X -> ManaRestrict filter; Activated.X REFUSED |
| SubAbility | 1: Chandra, Flameshaper | the next node of the chain (lowered in order) |

### api:Mill

| param | cards | lowering |
|---|---|---|
| Defined | 3: Billowing Shriekmass; Inspiration from Beyond; Sphinx's Tutelage | Obj ref (see Defined$ in Shared grammar) |
| NumCards | 3: Billowing Shriekmass; Inspiration from Beyond; Sphinx's Tutelage | A |
| RememberMilled | 1: Sphinx's Tutelage | REFUSED (Sphinx's Tutelage only) |
| SubAbility | 1: Inspiration from Beyond | the next node of the chain (lowered in order) |

### api:MultiplyCounter

| param | cards | lowering |
|---|---|---|
| Cost | 1: Zimone, Paradox Sculptor | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| CounterType | 1: Mossborn Hydra | OpDoubleCounters C (absent: 255 = every kind) |
| Defined | 1: Mossborn Hydra | Obj ref (see Defined$ in Shared grammar) |
| TargetMax | 1: Zimone, Paradox Sculptor | TargetSpec Max (with TargetMin) |
| TargetMin | 1: Zimone, Paradox Sculptor | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| ValidTgts | 1: Zimone, Paradox Sculptor | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:PeekAndReveal

| param | cards | lowering |
|---|---|---|
| Defined | 1: Curator of Destinies | Obj ref (see Defined$ in Shared grammar) |
| NoReveal | 1: Curator of Destinies | required |
| PeekAmount | 1: Curator of Destinies | OpPeek A (remembered) |
| RememberPeeked | 1: Curator of Destinies | required |
| SubAbility | 1: Curator of Destinies | the next node of the chain (lowered in order) |

### api:PermanentCreature

| param | cards | lowering |
|---|---|---|
| Cost | 1: Arbiter of Woe | the permanent spell's additional cost -> CardDef.SpellCost |

### api:Play

| param | cards | lowering |
|---|---|---|
| Amount | 1: Etali, Primal Storm | All (Etali shape) |
| Controller | 1: Etali, Primal Storm | You (Etali shape) |
| Optional | 1: Etali, Primal Storm | True (Etali shape) |
| SubAbility | 1: Etali, Primal Storm | the next node of the chain (lowered in order) |
| Valid | 1: Etali, Primal Storm | Etali shape only -> OpCastFree Obj=RefRemembered C=Exile |
| ValidSA | 1: Etali, Primal Storm | Spell (Etali shape) |
| ValidZone | 1: Etali, Primal Storm | Exile -> C |
| WithoutManaCost | 1: Etali, Primal Storm | True (Etali shape) |

### api:Pump

| param | cards | lowering |
|---|---|---|
| Cost | 5: Ajani, Caller of the Pride; Axgard Cavalry; Frenzied Goblin; Shivan Dragon; Sower of Chaos | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 13: Adventuring Gear; Beast-Kin Ranger; Celestial Armor; Courageous Goblin; Crackling Cyclops (+8) | Obj ref (see Defined$ in Shared grammar) |
| KW | 13: Ajani, Caller of the Pride; Ashroot Animist; Axgard Cavalry; Battlesong Berserker; Celestial Armor (+8) | OpGrantKeyword (C low 16 bits, B.N high 16); HIDDEN can't block -> KwCantBlock; prevent combat damage -> KwPreventCombatDamageTo; Flashback -> KwFlashbackOwnCost |
| NumAtt | 20: Adventuring Gear; Ashroot Animist; Battlesong Berserker; Beast-Kin Ranger; Bulk Up (+15) | OpPumpUntilEOT A (Double -> power of the target) |
| NumDef | 10: Adventuring Gear; Ashroot Animist; Drake Hatcher; Elementalist Adept; Giant Growth (+5) | OpPumpUntilEOT B |
| Planeswalker | 1: Ajani, Caller of the Pride | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| PumpZone | 1: Sphinx of Forgotten Lore | target zone (Graveyard: Sphinx of Forgotten Lore) |
| SubAbility | 4: Bite Down; Bushwhack; Fake Your Own Death; Fleeting Distraction | the next node of the chain (lowered in order) |
| TargetMax | 1: Divine Resilience | TargetSpec Max (with TargetMin) |
| TargetMin | 1: Divine Resilience | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| TgtZone | 1: Sphinx of Forgotten Lore | TargetSpec Kind TgtCard with Filter.Zone |
| ValidTgts | 17: Ajani, Caller of the Pride; Ashroot Animist; Axgard Cavalry; Battlesong Berserker; Bite Down (+12) | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:PumpAll

| param | cards | lowering |
|---|---|---|
| KW | 6: Balmor, Battlemage Captain; Goblin Bushwhacker; Heroic Reinforcements; Inspiring Call; Overrun (+1) | C (keywords below bit 16) |
| NumAtt | 9: Balmor, Battlemage Captain; Claws Out; Dauntless Veteran; Goblin Bushwhacker; Goblin Surprise (+4) | A |
| NumDef | 6: Claws Out; Dauntless Veteran; Heroic Reinforcements; Overrun; Preposterous Proportions (+1) | B |
| ValidCards | 10: Balmor, Battlemage Captain; Claws Out; Dauntless Veteran; Goblin Bushwhacker; Goblin Surprise (+5) | F |

### api:PutCounter

| param | cards | lowering |
|---|---|---|
| ConditionCompare | 1: Scavenging Ooze | Cond Op/A (default GE 1) |
| ConditionDefined | 1: Scavenging Ooze | CondRefMatches Ref |
| ConditionPresent | 1: Scavenging Ooze | OpSkipUnless guarding the node; Conds: CondPresent (count of F) or CondRefMatches with ConditionDefined |
| Cost | 5: Ajani, Caller of the Pride; Fishing Pole; High-Society Hunter; Hungry Ghoul; Treetop Snarespinner | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| CounterNum | 39: Ajani's Pridemate; Ajani, Caller of the Pride; Alesha, Who Laughs at Fate; Apothecary Stomper; Armasaur Guide (+34) | A |
| CounterType | 41: Ajani's Pridemate; Ajani, Caller of the Pride; Alesha, Who Laughs at Fate; Apothecary Stomper; Armasaur Guide (+36) | C counter kind (P1P1, LOYALTY, Stun, INCUBATION, BAIT, SOUL, FELLOWSHIP, REVIVAL, STASH) |
| Defined | 27: Ajani's Pridemate; Alesha, Who Laughs at Fate; Banner of Kinship; Brineborn Cutthroat; Clinquant Skymage (+22) | Obj ref (see Defined$ in Shared grammar) |
| ETB | 7: Banner of Kinship; Giada, Font of Hope; Gnarlid Colony; Goblin Boarders; Mossborn Hydra (+2) | only inside an ETB replacement program (OpEnterTapped / OpEnterWithCounters); elsewhere REFUSED |
| Planeswalker | 1: Ajani, Caller of the Pride | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| SorcerySpeed | 1: Treetop Snarespinner | CostSorcery |
| SubAbility | 5: Felling Blow; Fleeting Flight; Mild-Mannered Librarian; Scavenging Ooze; Snakeskin Veil | the next node of the chain (lowered in order) |
| TargetMax | 3: Ajani, Caller of the Pride; Felidar Savior; Zimone, Paradox Sculptor | TargetSpec Max (with TargetMin) |
| TargetMin | 3: Ajani, Caller of the Pride; Felidar Savior; Zimone, Paradox Sculptor | TargetSpec: 0..1 -> UpTo; N..M -> Min/Max; Kicked(0,1) -> TgfKickedRange |
| ValidTgts | 12: Ajani, Caller of the Pride; Apothecary Stomper; Armasaur Guide; Felidar Savior; Felling Blow (+7) | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:RemoveCounter

| param | cards | lowering |
|---|---|---|
| CounterNum | 1: Fishing Pole | A |
| CounterType | 1: Fishing Pole | C counter kind |
| Defined | 1: Fishing Pole | Obj ref (see Defined$ in Shared grammar) |
| RememberRemoved | 1: Fishing Pole | OpRemoveCounterRem (removed count -> ExRememberedCount) |
| SubAbility | 1: Fishing Pole | the next node of the chain (lowered in order) |

### api:Repeat

| param | cards | lowering |
|---|---|---|
| MaxRepeat | 1: Sphinx's Tutelage | REFUSED |
| RepeatCheckSVar | 1: Sphinx's Tutelage | REFUSED |
| RepeatSVarCompare | 1: Sphinx's Tutelage | REFUSED |
| RepeatSubAbility | 1: Sphinx's Tutelage | REFUSED (Sphinx's Tutelage) |
| SubAbility | 1: Sphinx's Tutelage | the next node of the chain (lowered in order) |
| ValidTgts | 1: Sphinx's Tutelage | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:ReplaceCounter

| param | cards | lowering |
|---|---|---|
| Amount | 1: Doubling Season | REFUSED (repl-hard) |

### api:ReplaceEffect

| param | cards | lowering |
|---|---|---|
| VarName | 1: Twinflame Tyrant | REFUSED (repl-hard) |
| VarValue | 1: Twinflame Tyrant | REFUSED (repl-hard) |

### api:ReplaceToken

| param | cards | lowering |
|---|---|---|
| Type | 1: Doubling Season | REFUSED (repl-hard) |

### api:Sacrifice

| param | cards | lowering |
|---|---|---|
| Amount | 2: Blasphemous Edict; Liliana, Dreadhorde General | A |
| Cost | 1: Liliana, Dreadhorde General | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Defined | 2: Blasphemous Edict; Liliana, Dreadhorde General | Obj ref (see Defined$ in Shared grammar) |
| Planeswalker | 1: Liliana, Dreadhorde General | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| SacValid | 2: Blasphemous Edict; Liliana, Dreadhorde General | F |

### api:SacrificeAll

| param | cards | lowering |
|---|---|---|
| SubAbility | 1: Liliana, Dreadhorde General | the next node of the chain (lowered in order) |
| ValidCards | 1: Liliana, Dreadhorde General | OpSacrificeAll F |

### api:Surveil

| param | cards | lowering |
|---|---|---|
| Amount | 6: Cephalid Inkmage; Lightshell Duo; Rune-Sealed Wall; Uncharted Voyage; Vanguard Seraph (+1) | A |
| Cost | 1: Rune-Sealed Wall | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |

### api:Tap

| param | cards | lowering |
|---|---|---|
| Defined | 12: Authority of the Consuls; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul; Dismal Backwater (+7) | Obj ref (see Defined$ in Shared grammar) |
| ETB | 12: Authority of the Consuls; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul; Dismal Backwater (+7) | only inside an ETB replacement program (OpEnterTapped / OpEnterWithCounters); elsewhere REFUSED |
| SubAbility | 1: Grappling Kraken | the next node of the chain (lowered in order) |
| ValidTgts | 2: Faebloom Trick; Grappling Kraken | a target slot: TargetSpec{Kind, Filter} (TgtObject/TgtCard/TgtPlayer/TgtAny/TgtSpell); the op acts on RefTargetN |

### api:Token

| param | cards | lowering |
|---|---|---|
| ConditionCheckSVar | 1: Fishing Pole | OpSkipUnless + CondValue(A=value, Op, B) |
| ConditionPresent | 1: Sylvan Scavenging | OpSkipUnless guarding the node; Conds: CondPresent (count of F) or CondRefMatches with ConditionDefined |
| Cost | 5: Ajani, Caller of the Pride; Drake Hatcher; Kaito, Cunning Infiltrator; Krenko, Mob Boss; Liliana, Dreadhorde General | ability cost -> kop.Cost (mana, CostTap/SacSelf/Sac/Discard/AddCtr/RemoveCtr/PayLife/TapX/TapHost); on an AB$ inside a trigger body: an optional resolution cost -> OpIfPaid covering the rest of the chain |
| Planeswalker | 3: Ajani, Caller of the Pride; Kaito, Cunning Infiltrator; Liliana, Dreadhorde General | AfLoyalty (loyalty ability: once per turn, sorcery speed) |
| RememberOriginalTokens | 1: Faebloom Trick | TokRemember |
| SubAbility | 3: Faebloom Trick; Fishing Pole; Heroic Reinforcements | the next node of the chain (lowered in order) |
| TokenAmount | 13: Ajani, Caller of the Pride; An Offer You Can't Refuse; Brass's Bounty; Faebloom Trick; Goblin Negotiation (+8) | A (Excess -> ExExcessDamage) |
| TokenOwner | 26: Ajani, Caller of the Pride; An Offer You Can't Refuse; Bake into a Pie; Brass's Bounty; Cat Collector (+21) | Obj |
| TokenScript | 39: Ajani, Caller of the Pride; An Offer You Can't Refuse; Arahbo, the First Fang; Bake into a Pie; Brass's Bounty (+34) | OpCreateTokenEx C = pool token index (remapped per matchup) |
| TokenTapped | 1: Revenge of the Rats | TokTapped |

### api:TwoPiles

| param | cards | lowering |
|---|---|---|
| Chooser | 1: Curator of Destinies | Opponent, or absent: the chooser is Defined$ (lower_twopiles.go) |
| ChosenPile | 1: Curator of Destinies | ChangeZone Defined$ Remembered from the library to the hand -> OpTwoPiles C=Hand |
| Defined | 1: Curator of Destinies | Obj ref (see Defined$ in Shared grammar) |
| DefinedCards | 1: Curator of Destinies | Remembered (the peeked cards) |
| FaceDown | 1: Curator of Destinies | One (Curator shape) |
| Separator | 1: Curator of Destinies | You, or Opponent for the controller's pick (TpOppSeparates) |
| UnchosenPile | 1: Curator of Destinies | ChangeZoneAll to Graveyard (the library's order), or ChangeZone Defined$ Remembered to Graveyard (TpUnchosenPileOrder) |
| Zone | 1: Curator of Destinies | Library |

### api:Untap

| param | cards | lowering |
|---|---|---|
| Defined | 1: Slumbering Cerberus | Obj ref (see Defined$ in Shared grammar) |

### api:Ward

| param | cards | lowering |
|---|---|---|
| UnlessCost | 5: Cackling Prowler; Koma, World-Eater; Sire of Seven Deaths; Tolarian Terror; Zul Ashur, Lich Lord | folded: KwWard + CardDef.WardN (mana) / WardLife |

### repl:AddCounter

| param | cards | lowering |
|---|---|---|
| ActiveZones | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| EffectOnly | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| ReplaceWith | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| ValidCard | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |

### repl:Counter

| param | cards | lowering |
|---|---|---|
| Layer | 2: Curator of Destinies; Koma, World-Eater | CantHappen -> ReplCantBeCountered |
| ValidCard | 2: Curator of Destinies; Koma, World-Eater | Card.Self |
| ValidSA | 2: Curator of Destinies; Koma, World-Eater | Spell |

### repl:CreateToken

| param | cards | lowering |
|---|---|---|
| ActiveZones | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| EffectOnly | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| ReplaceWith | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |
| ValidToken | 1: Doubling Season | REFUSED (repl-hard: Doubling Season) |

### repl:DamageDone

| param | cards | lowering |
|---|---|---|
| ActiveZones | 2: Crystal Barricade; Twinflame Tyrant | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |
| IsCombat | 1: Crystal Barricade | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |
| Prevent | 1: Crystal Barricade | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |
| ReplaceWith | 1: Twinflame Tyrant | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |
| ValidSource | 1: Twinflame Tyrant | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |
| ValidTarget | 2: Crystal Barricade; Twinflame Tyrant | REFUSED (repl-hard: Twinflame Tyrant, Crystal Barricade) |

### repl:GameLoss

| param | cards | lowering |
|---|---|---|
| ActiveZones | 1: Herald of Eternal Dawn | Battlefield |
| Layer | 1: Herald of Eternal Dawn | CantHappen -> ReplCantLoseGame |
| ValidPlayer | 1: Herald of Eternal Dawn | You |

### repl:GameWin

| param | cards | lowering |
|---|---|---|
| ActiveZones | 1: Herald of Eternal Dawn | Battlefield |
| Layer | 1: Herald of Eternal Dawn | CantHappen -> ReplOppsCantWinGame |
| ValidPlayer | 1: Herald of Eternal Dawn | Opponent |

### repl:Moved

| param | cards | lowering |
|---|---|---|
| ActiveZones | 2: Authority of the Consuls; Giada, Font of Hope | Battlefield (others-enter replacements) |
| CheckSVar | 2: Gnarlid Colony; Goblin Boarders | Repl.Cond |
| Destination | 22: Authority of the Consuls; Banner of Kinship; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul (+17) | Battlefield only (Graveyard REFUSED: repl-hard) |
| ReplaceWith | 22: Authority of the Consuls; Banner of Kinship; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul (+17) | the program: Tap ETB -> ReplEntersTapped/OpEnterTapped; PutCounter ETB -> ReplEntersWithCounters/OpEnterWithCounters; ChooseColor/ChooseType |
| ReplacementResult | 21: Authority of the Consuls; Banner of Kinship; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul (+16) | consumed |
| ValidCard | 22: Authority of the Consuls; Banner of Kinship; Bloodfell Caves; Blossoming Sands; Diregraf Ghoul (+17) | Card.Self -> ReplEnters*; Card.Self+cond -> Repl.Cond; another filter -> ReplOthersEnterProgram |

### repl:Untap

| param | cards | lowering |
|---|---|---|
| Layer | 1: Slumbering Cerberus | CantHappen -> ReplDoesntUntap |
| ValidCard | 1: Slumbering Cerberus | Card.Self |
| ValidStepTurnToController | 1: Slumbering Cerberus | You |

### stat:AlternativeCost

| param | cards | lowering |
|---|---|---|
| Cost | 1: Blasphemous Edict | Ability.Cost |
| EffectZone | 1: Blasphemous Edict | All only |
| IsPresent | 1: Blasphemous Edict | Cond2 |
| PresentCompare | 1: Blasphemous Edict | Cond2 Op/A |
| ValidSA | 1: Blasphemous Edict | Spell.Self only |

### stat:CantBlock

| param | cards | lowering |
|---|---|---|
| ValidCard | 1: Vampire Soulcaller | Filter |

### stat:CantBlockBy

| param | cards | lowering |
|---|---|---|
| Condition | 1: Cephalid Inkmage | Cond2 |
| ValidAttacker | 5: Cephalid Inkmage; Juggernaut; Kaito, Cunning Infiltrator; Rogue's Passage; Vampire Gourmand | Filter |
| ValidBlocker | 1: Juggernaut | Filter2 (absent: unblockable) |

### stat:CastWithFlash

| param | cards | lowering |
|---|---|---|
| Caster | 1: High Fae Trickster | You only |
| ValidCard | 1: High Fae Trickster | Filter |
| ValidSA | 1: High Fae Trickster | Spell only |

### stat:Continuous

| param | cards | lowering |
|---|---|---|
| AddAbility | 2: Fishing Pole; Imprisoned in the Moon | Static.Grant / override Grant (latent ability) |
| AddKeyword | 17: Akroma's Memorial; Celestial Armor; Crystal Barricade; Dreadwing Scavenger; Elenda, Saint of Dusk (+12) | Kw |
| AddPower | 18: Anthem of Champions; Arahbo, the First Fang; Banner of Kinship; Billowing Shriekmass; Blanchwood Armor (+13) | A or StDynamic VA |
| AddToughness | 15: Anthem of Champions; Arahbo, the First Fang; Banner of Kinship; Billowing Shriekmass; Blanchwood Armor (+10) | B or VB |
| AddType | 5: Imprisoned in the Moon; Infernal Vessel; Skyknight Squire; Valkyrie's Call; Witness Protection | subtype grant (staticSubs) / override set |
| AdjustLandPlays | 1: Loot, Exuberant Explorer | StatExtraLandPlay |
| Affected | 42: Akroma's Memorial; Anthem of Champions; Arahbo, the First Fang; Banner of Kinship; Billowing Shriekmass (+37) | Static.Filter (You: player statics) |
| AffectedZone | 9: Chandra, Flameshaper; Kellan, Planar Trailblazer; Muldrotha, the Gravetide; Omniscience; Quilled Greatwurm (+4) | StatMayPlayFrom Zone |
| CharacteristicDefining | 1: Consuming Aberration | StatCDAPT |
| CheckSVar | 1: Elenda, Saint of Dusk | Cond2 CondValue |
| Condition | 6: Billowing Shriekmass; Dreadwing Scavenger; Inspiring Paladin; Muldrotha, the Gravetide; Quick-Draw Katana (+1) | Threshold/PlayerTurn -> Cond2 |
| EffectZone | 2: Muldrotha, the Gravetide; Quilled Greatwurm | StFunctionsInGraveyard |
| MayPlay | 8: Chandra, Flameshaper; Kellan, Planar Trailblazer; Muldrotha, the Gravetide; Omniscience; Quilled Greatwurm (+3) | StatMayPlayFrom |
| MayPlayIgnoreType | 1: Tinybones, Bauble Burglar | StAnyManaType |
| MayPlayLimit | 1: Muldrotha, the Gravetide | StOncePerTurn |
| MayPlayWithoutManaCost | 1: Omniscience | StWithoutManaCost |
| RaiseCost | 1: Quilled Greatwurm | StRemove6Counters |
| RemoveAllAbilities | 2: Imprisoned in the Moon; Witness Protection | override LoseAbilities |
| RemoveCardTypes | 2: Imprisoned in the Moon; Witness Protection | override HasTypes |
| RemoveCreatureTypes | 1: Witness Protection | override HasSub |
| SVarCompare | 1: Elenda, Saint of Dusk | Cond2 Op/B |
| SetColor | 2: Imprisoned in the Moon; Witness Protection | override SetColors |
| SetMaxHandSize | 1: Niv-Mizzet, Visionary | StatNoMaxHandSize |
| SetName | 1: Witness Protection | consumed: no name-reading filter sees it (noted bend) |
| SetPower | 2: Consuming Aberration; Witness Protection | CDA VA / override SetP |
| SetToughness | 2: Consuming Aberration; Witness Protection | CDA VB / override SetT |

### stat:MustAttack

| param | cards | lowering |
|---|---|---|
| ValidCreature | 2: Flamewake Phoenix; Juggernaut | Filter |

### stat:ReduceCost

| param | cards | lowering |
|---|---|---|
| Activator | 2: Archmage of Runes; Mocking Sprite | You only |
| Amount | 8: Arcane Epiphany; Archmage of Runes; Claws Out; Embercleave; Ghalta, Primal Hunger (+3) | A / VA |
| EffectZone | 6: Arcane Epiphany; Claws Out; Embercleave; Ghalta, Primal Hunger; Luminous Rebuke (+1) | All -> Zone=ZoneCommand (works on the card itself in hand) |
| IsPresent | 1: Arcane Epiphany | Cond2 |
| Type | 8: Arcane Epiphany; Archmage of Runes; Claws Out; Embercleave; Ghalta, Primal Hunger (+3) | Spell only |
| ValidCard | 8: Arcane Epiphany; Archmage of Runes; Claws Out; Embercleave; Ghalta, Primal Hunger (+3) | Filter |
| ValidTarget | 1: Luminous Rebuke | StTargetsFilter2 + Filter2 |

### trig:AttackersDeclared

| param | cards | lowering |
|---|---|---|
| AttackingPlayer | 2: Armasaur Guide; Battlesong Berserker | You only -> WhoseYou |
| Execute | 2: Armasaur Guide; Battlesong Berserker | effect |
| TriggerZones | 2: Armasaur Guide; Battlesong Berserker | Zone |
| ValidAttackers | 1: Armasaur Guide | Filter2-like Filter |
| ValidAttackersAmount | 1: Armasaur Guide | GEn -> Trigger.N |

### trig:Attacks

| param | cards | lowering |
|---|---|---|
| Execute | 16: Alesha, Who Laughs at Fate; Ashroot Animist; Courageous Goblin; Crypt Feaster; Dauntless Veteran (+11) | effect |
| IsPresent | 2: Courageous Goblin; Ruby, Daring Tracker | Cond |
| NoResolvingCheck | 2: Courageous Goblin; Ruby, Daring Tracker | AfCheckOnTrigger |
| OptionalDecider | 2: Frenzied Goblin; Kiora, the Rising Tide | TfOptional |
| Threshold | 2: Crypt Feaster; Kiora, the Rising Tide | Cond CondThreshold |
| TriggerZones | 7: Ashroot Animist; Courageous Goblin; Etali, Primal Storm; Frenzied Goblin; High-Society Hunter (+2) | Zone |
| ValidCard | 16: Alesha, Who Laughs at Fate; Ashroot Animist; Courageous Goblin; Crypt Feaster; Dauntless Veteran (+11) | Filter / TfSelf |

### trig:BecomesTarget

| param | cards | lowering |
|---|---|---|
| Execute | 5: Cackling Prowler; Koma, World-Eater; Sire of Seven Deaths; Tolarian Terror; Zul Ashur, Lich Lord | the ward body (folded) |
| ValidTarget | 5: Cackling Prowler; Koma, World-Eater; Sire of Seven Deaths; Tolarian Terror; Zul Ashur, Lich Lord | Card.Self + Ward$ True only: folded into KwWard |
| Ward | 5: Cackling Prowler; Koma, World-Eater; Sire of Seven Deaths; Tolarian Terror; Zul Ashur, Lich Lord | see ValidTarget |

### trig:ChangesZone

| param | cards | lowering |
|---|---|---|
| CheckSVar | 4: Gorehorn Raider; Gutless Plunderer; Skyship Buccaneer; Strongbox Raider | Trigger.Cond CondValue |
| Destination | 79: Adventuring Gear; Affectionate Indrik; Ambush Wolf; Angel of Finality; Apothecary Stomper (+74) | see Origin |
| Execute | 79: Adventuring Gear; Affectionate Indrik; Ambush Wolf; Angel of Finality; Apothecary Stomper (+74) | the effect chain |
| IsPresent | 2: Dwynen's Elite; Garruk's Uprising | Trigger.Cond CondPresent |
| OptionalDecider | 7: Affectionate Indrik; Campus Guide; Micromancer; Reclamation Sage; Solemn Simulacrum (+2) | TfOptional |
| Origin | 78: Adventuring Gear; Affectionate Indrik; Ambush Wolf; Angel of Finality; Apothecary Stomper (+73) | Any->Battlefield = TrigETB; Battlefield->Graveyard = TrigDies; else REFUSED |
| TriggerController | 1: Fake Your Own Death | TriggeredCardController only (granted trigger) |
| TriggerZones | 18: Adventuring Gear; Authority of the Consuls; Beast-Kin Ranger; Dazzling Angel; Elfsworn Giant (+13) | Trigger.Zone (absent/Battlefield -> ZoneNone) |
| ValidCard | 79: Adventuring Gear; Affectionate Indrik; Ambush Wolf; Angel of Finality; Apothecary Stomper (+74) | Trigger.Filter; Card.Self -> TfSelf; Self + others -> Tf2SelfOrFilter |

### trig:CounterAddedOnce

| param | cards | lowering |
|---|---|---|
| ActivationLimit | 1: Exemplar of Light | 1 -> TfOncePerTurn |
| CounterType | 2: Exemplar of Light; Wildwood Scourge | Trigger.N |
| Execute | 2: Exemplar of Light; Wildwood Scourge | effect |
| TriggerZones | 2: Exemplar of Light; Wildwood Scourge | Zone |
| ValidCard | 2: Exemplar of Light; Wildwood Scourge | Filter / TfSelf |
| ValidSource | 1: Exemplar of Light | You only -> WhoseYou |

### trig:DamageDealtOnce

| param | cards | lowering |
|---|---|---|
| CombatDamage | 1: Quilled Greatwurm | TfCombat |
| Execute | 1: Quilled Greatwurm | effect |
| PlayerTurn | 1: Quilled Greatwurm | Tf2YourTurn |
| TriggerZones | 1: Quilled Greatwurm | Zone |
| ValidSource | 1: Quilled Greatwurm | Filter |

### trig:DamageDone

| param | cards | lowering |
|---|---|---|
| CombatDamage | 8: Drake Hatcher; Eager Trufflesnout; Goldvein Pick; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer (+3) | True TfCombat / False Tf2NonCombat |
| Execute | 8: Drake Hatcher; Eager Trufflesnout; Goldvein Pick; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer (+3) | effect |
| TriggerZones | 6: Drake Hatcher; Goldvein Pick; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer; Lathril, Blade of the Elves (+1) | Zone |
| ValidSource | 8: Drake Hatcher; Eager Trufflesnout; Goldvein Pick; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer (+3) | Filter / TfSelf |
| ValidTarget | 8: Drake Hatcher; Eager Trufflesnout; Goldvein Pick; Kaito, Cunning Infiltrator; Kellan, Planar Trailblazer (+3) | Player -> TfToPlayer; Opponent -> +Tf2ToOpponent |

### trig:Discarded

| param | cards | lowering |
|---|---|---|
| Execute | 1: Tinybones, Bauble Burglar | effect |
| TriggerZones | 1: Tinybones, Bauble Burglar | Zone |
| ValidCard | 1: Tinybones, Bauble Burglar | Filter (graveyard) |

### trig:Drawn

| param | cards | lowering |
|---|---|---|
| Execute | 6: Clinquant Skymage; Erudite Wizard; Homunculus Horde; Mischievous Mystic; Scrawling Crawler (+1) | effect |
| Number | 3: Erudite Wizard; Homunculus Horde; Mischievous Mystic | 2 -> TfSecondDraw |
| TriggerZones | 6: Clinquant Skymage; Erudite Wizard; Homunculus Horde; Mischievous Mystic; Scrawling Crawler (+1) | Zone |
| ValidCard | 6: Clinquant Skymage; Erudite Wizard; Homunculus Horde; Mischievous Mystic; Scrawling Crawler (+1) | Card.YouCtrl/YouOwn -> WhoseYou, Card.OppOwn -> WhoseOpp |

### trig:LifeGained

| param | cards | lowering |
|---|---|---|
| Execute | 6: Ajani's Pridemate; Cat Collector; Exemplar of Light; Fiendish Panda; Marauding Blight-Priest (+1) | effect |
| FirstTime | 2: Cat Collector; Vanguard Seraph | Tf2FirstTime |
| PlayerTurn | 1: Cat Collector | Tf2YourTurn |
| TriggerZones | 6: Ajani's Pridemate; Cat Collector; Exemplar of Light; Fiendish Panda; Marauding Blight-Priest (+1) | Zone |
| ValidPlayer | 6: Ajani's Pridemate; Cat Collector; Exemplar of Light; Fiendish Panda; Marauding Blight-Priest (+1) | Whose |

### trig:LifeLost

| param | cards | lowering |
|---|---|---|
| Execute | 1: Bloodthirsty Conqueror | effect |
| TriggerZones | 1: Bloodthirsty Conqueror | Zone |
| ValidPlayer | 1: Bloodthirsty Conqueror | Whose |

### trig:Phase

| param | cards | lowering |
|---|---|---|
| CheckSVar | 9: Alesha, Who Laughs at Fate; Cackling Prowler; Midnight Snack; Needletooth Pack; Perforating Artist (+4) | Cond |
| Execute | 16: Alesha, Who Laughs at Fate; Cackling Prowler; Extravagant Replication; Flamewake Phoenix; Midnight Snack (+11) | effect |
| IsPresent | 2: Flamewake Phoenix; Nessian Hornbeetle | Cond |
| Phase | 16: Alesha, Who Laughs at Fate; Cackling Prowler; Extravagant Replication; Flamewake Phoenix; Midnight Snack (+11) | Upkeep/End of Turn/BeginCombat -> Step |
| TriggerZones | 16: Alesha, Who Laughs at Fate; Cackling Prowler; Extravagant Replication; Flamewake Phoenix; Midnight Snack (+11) | Zone (Graveyard: Flamewake Phoenix) |
| ValidPlayer | 15: Alesha, Who Laughs at Fate; Cackling Prowler; Extravagant Replication; Flamewake Phoenix; Midnight Snack (+10) | Whose |

### trig:SpellCast

| param | cards | lowering |
|---|---|---|
| Execute | 16: Archmage of Runes; Balmor, Battlemage Captain; Brineborn Cutthroat; Consuming Aberration; Crackling Cyclops (+11) | effect |
| OpponentTurn | 1: Brineborn Cutthroat | Tf2OppTurn |
| TriggerZones | 12: Archmage of Runes; Balmor, Battlemage Captain; Brineborn Cutthroat; Consuming Aberration; Crackling Cyclops (+7) | Zone |
| ValidActivatingPlayer | 16: Archmage of Runes; Balmor, Battlemage Captain; Brineborn Cutthroat; Consuming Aberration; Crackling Cyclops (+11) | Whose |
| ValidCard | 15: Archmage of Runes; Balmor, Battlemage Captain; Brineborn Cutthroat; Crackling Cyclops; Drake Hatcher (+10) | Filter (stack) |

### trig:Untaps

| param | cards | lowering |
|---|---|---|
| Execute | 1: Fishing Pole | effect |
| TriggerZones | 1: Fishing Pole | Zone |
| ValidCard | 1: Fishing Pole | Filter |

## Lowering decisions that bend semantics (need sign-off)

- **Damage maps** (Drakuseth): the 4 + 3 + 3 damage is emitted as DealDamageEx
  nodes flagged DmgMapped and closed by OpResolveDamageMap; dirac must
  deal them as one simultaneous event to match gorge.
- **Triggered card identity**: TriggeredCard, TriggeredCardLKICopy and
  TriggeredNewCardLKICopy all lower to RefTriggerObj (dirac resolves it to
  the object wherever it now is; LKI reads use its last-known values).
- **SetName** (Witness Protection's "named Legitimate Businessperson") is
  dropped: the pool's only name filter (Hare Apparent) compares card ids.
- **ChooseType** offers the matchup's subtype table, not every creature type
  gorge offers; a type no card in the game has is equivalent to choosing it.
- **Prowess / Ward / Equip / Enchant** are folded back to kop's native forms
  (KwProwess, KwWard+WardN/WardLife, EquipCost, Enchant) instead of their
  expanded trigger/ability lines.
- **ChooseCard ForgetOtherRemembered** is dropped (the remembered set is only
  cleaned up afterwards in every pool use).
- **First-time life gain** (Tf2FirstTime) and **ActivationLimit 1** (TfOncePerTurn)
  are separate flags; both mean once per turn.

