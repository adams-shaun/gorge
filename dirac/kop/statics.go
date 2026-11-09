package kop

// StaticKind is a static ability's kind. Append-only.
type StaticKind uint8

const (
	StatNone            StaticKind = iota
	StatContinuous                 // objects matching Filter get +A/+B and keywords Kw (anthem / keyword grant)
	StatReduceCost                 // spells matching Filter cost A generic less (B = colour pips reduced, rarely)
	StatCantBlock                  // creatures matching Filter can't block
	StatCantBlockBy                // creatures matching Filter can't be blocked by creatures matching Filter2
	StatAlternativeCost            // the card may be cast for alternative cost Ability.Cost (condition C)
	StatCastWithFlash              // spells matching Filter may be cast any time you could cast an instant
	StatMustAttack                 // creatures matching Filter attack each combat if able
	StatCantAttack                 // creatures matching Filter can't attack
	StatIncreaseCost               // spells matching Filter cost A more (Ward-like taxes)

	// --- appended by dirac/kgen (fields in table.go) ---
	StatCDAPT          // characteristic-defining P/T: the source's P/T are VA/VB
	StatNoMaxHandSize  // the controller has no maximum hand size
	StatExtraLandPlay  // the controller may play an additional land each turn
	StatMayPlayFrom    // the controller may play cards matching Filter from zone Zone (StFlags qualify)
	StatOverride       // objects matching Filter get Table.Overrides[Override-1]
	StatPlayerHexproof // the controller has hexproof (Crystal Barricade)
	StatCantBeCast     // spells matching Filter can't be cast (StRaiseYou: by the controller only; StRaiseOpp: by its opponents only; Cond2) (gorge castRestricted)
	StatControl        // the source's controller controls the objects matching Filter (the source Aura's enchanted permanent; layer 2, CR 613.1b): Control Magic, Kitnap
	StatPanharmonicon  // a triggered ability of an object of the controller matching Filter triggers an additional time (CR 603.2d; gorge statics_echo.go panharmoniconEchoes)
	StatCount
)

// Static is a static ability.
type Static struct {
	Kind    StaticKind
	Filter  FilterID
	Filter2 FilterID // CantBlockBy: the blockers that can't block; ReduceCost (spell or activated ability): the target it needs
	A, B    int16    // P/T deltas, cost reduction
	Kw      KwMask
	Cond    uint8 // reserved condition opcode (0 = always)

	// Appended by dirac/kgen.
	Cond2    CondID      // the static applies while Table.Conds[Cond2] holds (0 = always)
	VA, VB   ValueRef    // the amounts when StDynamic (P/T deltas, cost reduction, CDA P/T)
	StFlags  StaticFlags // kind-specific qualifiers
	AddSub   SubMask     // subtypes granted (Table.Subtypes indexes)
	Grant    uint8       // the source card's ability Grant-1 is granted to the affected objects (0 = none)
	Zone     Zone        // StatMayPlayFrom: the zone played from; StatReduceCost: ZoneNone = battlefield only, ZoneCommand = any zone (the card itself)
	Override uint16      // StatOverride: Table.Overrides[Override-1]
	// Ward: a StatContinuous grant's ward {Ward} (Kw holds KwWard; CR
	// 702.21a): each such static is its own ward instance
	Ward uint8
	// AddChosen: AddType$ ChosenType (Adaptive Automaton, Lifecraft
	// Engine): the affected objects also have the source permanent's
	// chosen creature type (perm.chosen); no choice grants nothing (gorge
	// resolveChosenTypes). kernel act_choosetype.go.
	AddChosen bool
	// OutlivesLoss: a StatContinuous pump that rides beside a StatOverride
	// with a part in layers 1 to 5 (Undercover Skrull's +2/+2 beside its
	// all creature types): the whole effect began before layer 6, so it keeps
	// applying after its source loses all abilities (CR 613.6).
	OutlivesLoss bool
	// PwVsSrc: StatCantBlockBy only: the blocker must also have power
	// less than (1) or greater than (2) the static's host's power (the
	// ValidBlocker$ powerLTX / powerGTX with X = Count$CardPower; kernel
	// static_powercompare.go). 0 = no such compare.
	PwVsSrc int8
	// OvGate: the static applies only while a live override record on its
	// source grants it, i.e. Override.GrantStatics has this one bit (an
	// Animate staticAbilities$ grant of a permission or an extra land play:
	// Nog, Ambitious Cadet's Officer step). 0 = a printed static.
	OvGate uint8
}

// Override is a fixed-order override record (the plan's replacement for a
// general layer system): applied fields replace or extend the object's
// characteristics while it lasts.
type Override struct {
	HasPT      bool
	SetP, SetT int8
	// ValPT: PA/TA are evaluated at resolution (Animate Power$/Toughness$)
	// and the resulting values replace SetP/SetT in the applied record.
	ValPT            bool
	PA, TA           ValueRef
	HasTypes         bool
	SetTypes         TypeMask // card types (supertypes are kept)
	AddTypes         TypeMask
	HasSub           bool
	SetSub           SubMask // Table.Subtypes indexes
	AddSub           SubMask
	AllCreatureTypes bool
	HasColors        bool
	SetColors        ColorMask
	LoseAbilities    bool
	AddKw            KwMask
	Grant            [2]uint8 // the source card's ability index+1 granted (0 = none)
	// NameCard is CardID+1 of a card definition carrying the override's
	// layer-3 name and its type line (Witness Protection's "Legitimate
	// Businessperson"); 0 = the name is unchanged.
	NameCard uint16
	// KeepP / KeepT: a HasPT set leaves power / toughness alone (a static
	// that sets base toughness only: Maha, Its Feathers Night).
	KeepP, KeepT bool
	// RemoveKw: the keywords the effect removes (layer 6, CR 613.1f; a
	// static's RemoveKeyword$: Starforged Sword's "loses flying"). On a
	// StatContinuous static, Static.Override names an Override carrying
	// only this removal (the static's other parts stay Continuous).
	RemoveKw KwMask
	// WhileHost: the record lasts only while the effect's source (its
	// host) stays on the battlefield (Duration$ UntilHostLeavesPlay; the
	// applied record keeps the host's Obj and ver in its P/T fields, so a
	// WhileHost override sets no P/T).
	WhileHost bool
	// WhileTapped: a WhileHost record that also ends at the host's first
	// untap (Duration$ UntilUntaps, CR 611.2b: Hedge Whisperer), and starts
	// only when the host is tapped as it resolves. Its P/T is SetP/SetT
	// (the record's P/T fields hold the host).
	WhileTapped bool
	// CantAttack: the permanent can't attack (HiddenKeywords$ CARDNAME can't
	// attack or block., beside AddKw KwCantBlock; KwMask has no free bit).
	CantAttack bool
	// NoCost: a copy exception's "with no mana cost" (CR 707.9b; embalm's
	// RemoveCost$): the copy's mana value is 0.
	NoCost bool
	// RemoveCreatureSub: RemoveCreatureTypes$ without RemoveCardTypes$
	// (gorge chars/types.go applyTypeEffect): drop the creature subtypes
	// (genPrep.creatSub) and the all-creature-types mark, keep the rest,
	// then add AddSub.
	RemoveCreatureSub bool
	// AddColors: colours added to the object's own (an Animate Colors$ without
	// OverwriteColors$; a copy exception's
	// AddColors$, CR 707.9b; applied after a SetColors).
	AddColors ColorMask
	// RemoveLegendary: a copy exception's NonLegendary$ (CR 707.9b): the
	// copy's type line loses Legendary; gorge copypermanent.go removeLegendary.
	RemoveLegendary bool
	// LivePT: PA/TA are re-evaluated at every read instead of once at
	// resolution. An Animate staticAbilities$ SetPower$/SetToughness$ grant
	// (Beorn's Hospitality) evaluates them for the affected permanent; a
	// StatOverride static evaluates them from its source and the source's
	// controller (an Equipment's "your life total"). ValPT is then false.
	LivePT bool
	// KeepSub and KeepTypes: what a HasSub or HasTypes replacement leaves of a
	// card type the effect adds (CR 205.1a; Imprisoned in the Moon's land stays
	// a Forest). KeepTypes holds the subtypes TypeMask carries as bits.
	KeepSub   SubMask
	KeepTypes TypeMask
	// GrantStatics: the source card's statics whose OvGate bit is set here
	// are in force on the object while this record lasts (an Animate
	// staticAbilities$ grant that no layer reads: a may-play permission, an
	// extra land play).
	GrantStatics uint8
	// PTDelta: PA/TA (ValPT) are a +N/+M pump added after counters (layer
	// 7c, CR 613.4c) instead of a base P/T; HasPT is false. A PumpAll
	// Duration$ UntilYourNextTurn NumAtt$/NumDef$ (V'Ger, the Intruder).
	PTDelta bool
}

// Emblem is a command-zone object with abilities (triggered or static).
type Emblem struct {
	Abilities []Ability
	// UntilEOT: an Effect without a Duration$ (gorge: it leaves the
	// command zone at cleanup); the kernel drops it in cleanupGen
	UntilEOT bool
	// UntilNextTurn: Duration$ UntilYourNextTurn (gorge ends it at the
	// cleanup before the controller's next turn); the kernel drops it in
	// cleanupGen when that turn is the next one.
	UntilNextTurn bool
	// WhileHost: it also ends when the card that created it (its host,
	// State.g.embO) leaves the battlefield (Duration$
	// UntilHostLeavesPlayOrEOT), and its statics read the host as their
	// source (an EffectSource filter is FSelf).
	WhileHost bool
	// Capture: the target slot (a Ref) whose object the emblem remembers
	// (the Effect's RememberObjects$ Targeted); RefNone when it remembers
	// none. OpCreateEmblem stores the object in State.g.embN (object+1;
	// 0 = none), and a DamageDone trigger of an EOT emblem with Capture set
	// fires only for that object as the damage source (gorge's
	// Creature.IsRemembered over the registration's Remembered set; The Inner
	// Light). The trigger's filter carries no Remembered flag.
	Capture Ref
	// UntilEndOfNextTurn: Duration$ UntilTheEndOfYourNextTurn. gorge keeps
	// the registration through its controller's next turn and drops it at
	// that turn's cleanup (engine/rules/layers_effect.go:91 freezes
	// UntilTurn to nextTurnFor, and :510 drops it when e.G.Turn reaches
	// UntilTurn). The kernel marks the emblem in State.g.embF at its
	// controller's turn start and drops it in cleanupGen when that turn ends.
	UntilEndOfNextTurn bool
	// WhileHostNoEOT: the WhileHost leave check without the cleanup drop
	// (Duration$ AsLongAsInPlay): the emblem ends only when its host leaves
	// the battlefield, and its statics read the host as their source.
	WhileHostNoEOT bool
	// AsLongAsControl: Duration$ AsLongAsControl. It ends when its host
	// leaves the battlefield or is no longer controlled by the emblem's
	// player (CR 611.2b; gorge's continuousLive and expireAsLongAsControl),
	// and its statics read the host as their source.
	AsLongAsControl bool
}

// HostBound reports whether the emblem's lifetime follows its host object:
// it ends when that object leaves the battlefield, and its statics read the
// host as their source.
func (e *Emblem) HostBound() bool {
	return e.WhileHost || e.WhileHostNoEOT || e.AsLongAsControl
}

// Panharmonicon's Static.A: the one trigger mode a ValidMode$ names without
// a cause (0: every mode). Static.B is then the Whose of the event's player
// (ValidPlayer$), read against the static's controller.
const (
	PanhModeLifeGained int16 = 1 // ValidMode$ LifeGained: a trigger of a player gaining life
	PanhModeDrawn      int16 = 2 // ValidMode$ Drawn: a trigger of a player drawing a card
)

// StaticFlags (Static.StFlags).
type StaticFlags uint16

// Gate is an ability's threshold gate: with Ctr != 0 the
// ability exists only while the source permanent's count of Ctr is >= N
// (== N when Eq). A Class's level-N ability is Gate{Ctr: CtrLevel, N: N-1};
// its level-N activator is Gate{CtrLevel, N-2, Eq} (gorge kwClass:
// IsPresent$ Card.Self+classLevel_EQ<N-1>). A gated trigger is checked when
// the event happens and again on resolution (gorge's ClassBand$ rides the
// intervening-if walk, triggerConditionHoldsWithSVars). Only a printed
// battlefield trigger or activated ability carries one (kgen refuses the
// rest); the kernel reads it only behind prep.anyGate.
type Gate struct {
	Ctr CounterKind
	N   uint8
	Eq  bool
}

// StatCanAttackDefender (an explicit high ordinal
// clear of the iota tail in ops.go): creatures matching Filter can attack as
// though they didn't have defender (CR 702.3b; gorge
// combat.AttackAllowedThroughDefender), while Cond holds.
const StatCanAttackDefender StaticKind = 250

// StatGoad (an explicit high ordinal clear of the iota
// tail in ops.go): creatures matching Filter are goaded by the static's
// controller while it applies (a Continuous static's Goad$ True: Koloth,
// Tribble Hunter; CR 701.15b; gorge's static goad route, IsGoaded). With
// two players the goader never changes what a goaded creature must do, so
// the kernel reads it as goaded by anyone (act_goad.go goadedAt).
const StatGoad StaticKind = 249

// StatCantTarget (an explicit high ordinal clear of the iota tail in
// ops.go): a printed S:Mode$ CantTarget. A battlefield permanent matching
// Filter (read with the static's controller as "you" and its source as
// Self) can't be the target of a spell or ability whose source matches
// Filter2 (0 = any source; the spell card, or the permanent that has the
// ability, read from the static controller), whose kind is in A (a mask of
// CantTgtSpell, CantTgtActivated, CantTgtTriggered; 0 = every kind) and
// whose controller StFlags names (StRaiseOpp: the static controller's
// opponents; StRaiseYou: the controller; neither: anyone), while Cond2 holds
// (CR 115.4; gorge cantTargetBlocks). Species 8472, Thrun, Breaker of
// Silence.
const StatCantTarget StaticKind = 235

// StatCantTarget's Static.A bits: the targeting spell or ability kinds a
// ValidSA$ names.
const (
	CantTgtSpell     = 1
	CantTgtActivated = 2
	CantTgtTriggered = 4
)

// StatCantActivate (an explicit high ordinal clear of
// the iota tail in ops.go): activated abilities of battlefield permanents
// matching Filter can't be activated (StRaiseOpp: by the static
// controller's opponents only; StRaiseYou: by its controller only; under
// Cond2; A&3 = 1 spares mana abilities; A&3 = 2 reaches loyalty abilities only;
// A&CantActNamed: only sources whose card is the static source's as-enters
// chosen name) (CR 602.5; gorge abilityRestricted).
const StatCantActivate StaticKind = 236

// CantActNamed is the StatCantActivate A bit for ValidCard$ Card.NamedCard
// (Sorcerous Spyglass): the filter holds only for a source with the static
// source's chosen name (ExtChooseName).
const CantActNamed = 4

// StCasterActive: on a StatCantBeCast only, the
// restriction binds a caster who is the active player (Caster$
// Player.Active; gorge actorMatches). It aliases StTargetsFilter2, whose
// every reader is gated on Kind == StatReduceCost (dirac/statics_gen.go:80,
// legal.go:681, statics_gen.go:332, mana_window.go:904, featview.go:681), so
// the StaticFlags iota tail in table.go stays untouched.
const StCasterActive = StTargetsFilter2

// StSeatActive: on a StatMayPlayFrom only, the permission's beneficiary is
// the active player (MayPlayPlayer$ Player.Active; Weftwalking), not the
// static's controller. It aliases StEmbNum (1<<14), a ReduceCost-emblem bit
// no MayPlayFrom static reads; StaticFlags is full at 16 bits.
const StSeatActive = StEmbNum

// StatDisableETB: S:Mode$ DisableTriggers | ValidMode$
// ChangesZone[,ChangesZoneAll] | Destination$ Battlefield (CR 603.2g; gorge
// disableTriggersExcludes). Filter = ValidCause$, matched against the
// ENTERING object with the static's controller and source as context; no
// Filter2. While it applies, nothing triggers off that object entering the
// battlefield. An explicit ordinal keeps the iota tail of StaticKind
// untouched.
const StatDisableETB StaticKind = 234

// StEmbNum (explicit bit 14, outside the iota block):
// a StatReduceCost static an Effect registered for an emblem whose Amount$
// was `Count$ChosenNumber`. gorge's effects/effect.go evaluates
// SetChosenNumber$ ONCE when the Effect resolves (the frozen
// state.ContinuousEffect.ChosenNumber) and the cost path reads that frozen
// number; a later life-loss or life-gain change in the same turn does not
// raise the discount. The kernel keeps the frozen number per emblem
// (State.g.embN) and this flag says the static's amount is that number, not
// Static.VA (so StDynamic is clear and Static.A stays 0). A byte above any
// generic cost is saturated at 255 (an exact discount).
const StEmbNum StaticFlags = 1 << 14

// StRaiseLife (aliases StPanhETB, a Panharmonicon-only bit) on a
// StatIncreaseCost: Static.A is LIFE the matching spell's caster pays on top
// of its cost, not generic mana (RaiseCost Cost$ PayLife<N>: Terror of the
// Peaks; gorge composeRaiseCost's RaiseLife). With StTargetsFilter2 the life
// is owed once a chosen target matches Filter2.
const StRaiseLife = StPanhETB

// StRaisePerTarget (aliases StOncePerTurn, a MayPlayFrom-only bit) on a
// StatIncreaseCost: the spell's own "costs more for each target beyond the
// first" (Officious Interrogation; gorge composeRaiseCost, Relative$ True with
// Amount$ TargetedObjects$Amount/Minus.1). Static.A is the generic mana per
// charge, Static.B the coloured pips per charge packed as StReduceColor packs
// them; the charge count is the targets chosen so far less one (0 with none),
// read from the build in progress (raiseCostGen).
const StRaisePerTarget = StOncePerTurn

// StatMaxBlockers: creatures matching Filter (battlefield,
// scanned against the attacker with the static's controller) can't be
// blocked by more than one creature while Cond2 holds (CR 509.1b; gorge
// MinMaxBlockerBounds, the literal Max$ 1). An explicit ordinal keeps the
// iota tail of StaticKind untouched.
const StatMaxBlockers StaticKind = 241

// StMayRaise (explicit high bit, outside the iota
// block): a StatMayPlayFrom static whose permission carries a non-mana
// RaiseCost$ ("by paying 3 life in addition", "by discarding a card in
// addition", "by paying 2 life and sacrificing an artifact or creature in
// addition"; gorge rules/mayplay.go mayPlayRaiseCost). Static.B is 1 + the
// index of a latent ability of the static's SOURCE card whose Cost is the
// raise: PayLife<N> (CostPayLife, N), Discard<1/Card> (CostDiscard, 1), or
// PayLife<N> + Sac<1/filter> (CostSac with N = 1, CostPayLife with the life in
// Cost.Ctr: the two counts cannot share Cost.N). Static.A stays the
// StRemove6Counters count.
const StMayRaise StaticFlags = 1 << 15

// StatCantGainLife / StatCantDraw: while the static
// functions (battlefield, Cond2), no player gains life / draws a card
// (gorge replacement_life.go lifeGainForbidden, drawForbidden). kgen
// admits only ValidPlayer$ absent or Player. Explicit ordinals: the
// StaticKind iota tail stays untouched.
const (
	StatCantGainLife StaticKind = 247
	StatCantDraw     StaticKind = 248
)

// StatCantPutCounter (an explicit high ordinal clear of the iota tail in
// ops.go): no counters of Static.A's kind (A < 0: any kind) may be put on a
// battlefield permanent matching Filter while the static functions. The
// object form of gorge PutCounterBlocked (engine/rules/layers_restrict.go):
// checked at the counter-placement choke point, before any AddCounter
// replacement, so a prohibition with no accompanying R:Event$ AddCounter
// line still swallows the placement (CR 122.1; Blossombind, Blightbeetle,
// Tatterkite, Melira's Keepers). The player form (ValidPlayer$) is not
// lowered.
const StatCantPutCounter StaticKind = 252

// StatCantPhaseIn (an explicit ordinal clear of the iota tail): while the
// static's source is on the battlefield no permanent can phase in (S:Mode$
// CantPhaseIn | ValidCard$ Card.phasedOutPermanent, "Permanents can't phase
// in"; CR 702.26; gorge rules/cantphase.go phaseBlocked). Only that
// blanket ValidCard$ is lowered. dirac/kw_phasing.go.
const StatCantPhaseIn StaticKind = 238

// StatCantSacrifice (an explicit ordinal clear of the iota tail): no
// battlefield permanent matching Filter (read with the static's controller
// as "you") can be sacrificed, for a cost or by an effect (S:Mode$
// CantSacrifice without ValidCause$ or ForCost$; CR 701.21a; gorge
// rules/layers_restrict.go sacrificeBlocked). A cost that would sacrifice
// one can't be paid, so a Treasure under Quark, Humble Proprietor is no
// mana source.
const StatCantSacrifice StaticKind = 237

// StatReduceAbility: activated abilities of
// battlefield permanents matching Filter (controlled by the static's
// controller) cost A (or VA when StDynamic) generic less, floored at zero;
// B = 1: Exhaust abilities only; B = kop.ReduceAbPowerUp: Power-up abilities
// only; B = kop.ReduceAbEquip: Equip abilities only, and with StTargetsFilter2
// only an equip whose target matches Filter2, the chosen target priced
// (CR 601.2f, 602.2b, 702.193a; gorge costModifiersCompose,
// abilityScope). It follows its own ability's ReduceCost$. It prices a filter
// land's fee (filtFee); any other mana ability with a mana cost is not
// priced, and kgen's matchup guard keeps the static out of a pool holding one
// it could lower. An explicit ordinal keeps the iota tail of StaticKind
// untouched.
const StatReduceAbility StaticKind = 233

// two StaticFlags bits (explicit, outside the
// iota block; 14 and 15 are StEmbNum and StMayRaise) on a spell's own
// ReduceCost static.
const (
	// StBargained gates a self ReduceCost ("costs {N} less if it's
	// bargained": Hamlet Glutton, Ice Out, Johann's Stopgap; ValidSpell$
	// Spell.Bargain). gorge's spellConstraintMatches reads the cast MODE, so
	// the static prices only the bargained cast (CR 702.166), at the offer
	// and at the charge.
	StBargained StaticFlags = 1 << 12
	// StReduceColor is a ReduceCost whose Color$ names coloured pips as well
	// as generic ones (Brush Off's "Color$ 1 U"). Static.A is the generic
	// part; Static.B packs the coloured part, two bits per colour in W U B R
	// G C order from bit 0 (a count of 0..3). The coloured pips come off the
	// cost first and the shortfall spills to generic (gorge pay CostMods.ApplyTo).
	StReduceColor StaticFlags = 1 << 13
)

// StatStackKw: the spells the static's controller casts that match Filter
// (a card on the stack or still in hand) carry the keyword StackKw* code A
// for the cast-time readers (AffectedZone$ Stack AddKeyword$ Cascade; gorge
// keywordmay.go stackKeywordPossibleH, cascade.go cascadeInstances).
// Zone == ZoneHand is the "cast from your hand" word (wasCastFromYourHand).
// Explicit value: the tail of the iota block is taken.
const StatStackKw StaticKind = 251

// StatCombatDmgToughness: creatures matching Filter
// (battlefield, read with the static's controller and source) assign combat
// damage equal to their toughness rather than their power (CR 510.1;
// gorge statics_assignment.go combatDamageAmount). Explicit ordinal: the
// StaticKind iota tail stays untouched.
const StatCombatDmgToughness StaticKind = 245

// StatCantBeBeamedUp: permanents matching Filter (battlefield, read with the
// static's controller and source) can't be returned to pay beam me up (gorge
// matchesBeamReturn). Explicit ordinal: the StaticKind iota tail stays untouched.
const StatCantBeBeamedUp StaticKind = 244

// TapPowToughness is TapPowerCrew / TapPowerSaddle's sentinel: the card
// crews / saddles using its toughness rather than its power (Value$ Toughness).
const TapPowToughness int8 = -128

// StatUntapOther: permanents matching Filter (battlefield;
// the kgen shape is Card.Self) untap during each other player's untap step,
// after the active player's untap action (gorge rules/turn.go
// finishUntapStep, untap.go untapOtherStaticsMatch).
// Explicit ordinal: the StaticKind iota tail stays untouched.
const StatUntapOther StaticKind = 246

// StatBeamGrant: during the controller's main phase each instant and sorcery card
// in their graveyard has beam me up at its mana cost (Scotty, Miracle Worker). The
// granter's CardDef.FbCost holds the return.
const StatBeamGrant StaticKind = 253

// StBeamGranted on a StatCantBeBeamedUp static: it bars only the return that pays
// for a grant its own permanent makes.
const StBeamGranted StaticFlags = 1 << 0

// StatTapExtra: whenever the static's controller taps a permanent matching
// Filter for mana, they add A-coloured mana, B of it (A is a mana index W U B R
// G C), once per tap (a Mode$ TapsForMana | Static$ True trigger whose body is
// one literal DB$ Mana: Groundchuck & Dirtbag, Badgermole Cub, Forsaken
// Monument; gorge pay.PaymentPlanFoldableTapMana). StTapOnC restricts it to a
// tap that declares {C} (Produced$ C). Explicit ordinal: the StaticKind iota
// tail stays untouched.
const StatTapExtra StaticKind = 254

// StTapOnC on a StatTapExtra static: it fires only for a tap that makes {C}.
const StTapOnC StaticFlags = 1 << 1

// StatAnyTypeCreatures: the static's controller may spend mana as though it
// were mana of any type to cast creature spells (S:Mode$ ManaConvert |
// ValidPlayer$ You | ValidCard$ Creature.YouCtrl | ValidSA$ Spell |
// ManaConversion$ AnyType->AnyType; CR 609.4b; Vizier of the Menagerie;
// gorge rules/mana_convert.go manaConversionParts). No operands: the fact is
// read from the battlefield. Explicit ordinal: the StaticKind iota tail
// stays untouched.
const StatAnyTypeCreatures StaticKind = 239
