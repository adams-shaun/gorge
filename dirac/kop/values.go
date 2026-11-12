package kop

// VKind is how a ValueRef computes its amount. Append-only.
type VKind uint8

const (
	VConst           VKind = iota // N
	VX                            // the spell's X (times N if N>1... N ignored; X)
	VCountFilter                  // the number of objects matching filter N
	VLastDamage                   // damage dealt by the previous DealDamage/Fight in this program
	VTargetPower                  // target 0's power
	VSelfPower                    // the source's power
	VCountersSelf                 // the number of counters of kind N on the source
	VCardsInHand                  // the controller's hand size
	VTriggerAmount                // the amount the trigger event carried (damage, life, counters)
	VNegX                         // -X
	VLifeGainedTurn               // life the controller gained this turn
	VTargetToughness              // target 0's toughness
	VSelfToughness                // the source's toughness
	VCountersTarget               // the number of counters of kind N on target 0
)

// ValueRef is an op amount.
type ValueRef struct {
	K VKind
	N int16
}

// Const is a constant ValueRef.
func Const(n int) ValueRef { return ValueRef{K: VConst, N: int16(n)} }

// VExpr: the amount is Table.Exprs[N].
const VExpr VKind = VCountersTarget + 1

// ExprKind is an Expr's computation. Append-only.
type ExprKind uint8

const (
	ExNone                ExprKind = iota
	ExConst                        // N
	ExX                            // the X paid
	ExCount                        // battlefield objects matching F
	ExCountGraveyard               // graveyard cards matching F (all graveyards; F.Owner narrows); N 1 also counts tokens
	ExSumPower                     // total power of battlefield creatures matching F
	ExDistinctCMC                  // distinct mana values among battlefield objects matching F
	ExKicked                       // Sub[0] if the spell was kicked, else Sub[1]
	ExMorbid                       // Sub[0] if a creature died this turn, else Sub[1]
	ExAttackedThisTurn             // creatures the controller attacked with this turn
	ExPowerOf                      // power of Ref (Slot: target slot for RefTarget0)
	ExToughnessOf                  // toughness of Ref
	ExLifeOf                       // life total of player Ref
	ExLibrarySizeOf                // cards in player Ref's library
	ExStartingLife                 // the controller's starting life total
	ExTriggerAmount                // the triggering event's amount
	ExLifeGainedThisTurn           // life the controller gained this turn
	ExOppLifeLostThisTurn          // life the controller's opponents lost this turn
	ExCountersOn                   // counters of kind Ctr on Ref
	ExCreaturesDied                // creatures put into a graveyard from the battlefield this turn
	ExRememberedCount              // size of the remembered set
	ExExcessDamage                 // excess damage the previous DealDamageEx dealt (DmgTrackExcess)
	ExLastLifeLost                 // life the previous LoseLife took
	ExTokensCreated                // tokens the previous CreateTokenEx created (TokRemember)
	ExLifeLostThisTurn             // life the controller lost this turn
	ExSum                          // Sub[0] + Sub[1] (an SVar-named modifier: /Plus.Y, /Minus.Y via a negated Sub[1])
	ExHandSizeOf                   // cards in player Ref's hand
	ExCountExile                   // exiled cards matching F (all exile zones; F.Owner narrows)
	ExOppsLostLife                 // the number of the controller's opponents who lost life this turn
	ExMVOf                         // the mana value of object Ref's card (printed; X counts 0)
	ExCountAtLeast                 // Sub[0] if battlefield objects matching F number at least N, else Sub[1] (Count$Compare)
	ExPromisedGift                 // Sub[0] if the gift was promised (the spell, or the permanent's spell as it enters), else Sub[1] (Count$PromisedGift)
	ExResolvedThisTurn             // times the resolving ability (source object, ability) resolved this turn, this one included (AfResolveCount)
	ExThisTurnCast                 // spells player Ref cast this turn: N=0 every spell (the history bank), N=1 instants and sorceries (Count$ThisTurnCast_<spec>)
	ExRollResult                   // the result (die + modifier) of the resolution's last OpRollDie (gorge RollDice ResultSVar$)
	ExCastManaSpent                // the mana spent to cast the resolving (or entering) spell (Count$CastTotalManaSpent)
	ExCompare                      // Count$Compare: Sub[0] compared (Op) with N picks Sub[1]'s ExBranches then (true) or else arm
	ExBranches                     // ExCompare's arms: Sub[0] then, Sub[1] else (never evaluated alone)
	ExDevotion                     // the controller's devotion to the colours of ColorMask N (CR 700.5)
	ExParty                        // the controller's party size (CR 700.8)
	ExDrawnThisTurn                // cards the controller drew this turn (Count$YouDrewThisTurn)
	ExGraveTypes                   // card types among the cards in the controller's graveyard (delirium, CR 207.2c)
	ExValidAgg                     // aggregate N (kop.Agg*) over the battlefield objects matching F (Count$Valid <spec>$<prop>)
	ExGraveAgg                     // mana-value aggregate N (AggGreatestMV, AggSumMV, AggDistinctMV) over the graveyard cards matching F (Count$ValidGraveyard <spec>$<prop>)
	ExYourSpeed                    // the controller's speed (Count$YourSpeed; CR 702.179)
	ExHighestOppPoison             // the highest poison count among the controller's living opponents (PlayerCountOpponents$HighestCounters.Poison; value_highestpoison.go)
	ExThisTurnEntered              // per-ADD zone entries this turn: N=destination zone, F/F2/F3 filters, From, Prop
	ExThisTurnCastSpec             // spells cast this turn whose object matches F where it is now: N holds the CastSpec* bits (Count$ThisTurnCast_<spec>)
	ExCommittedCrime               // Sub[0] if the controller committed a crime this turn, else Sub[1] (Count$CommittedCrimeThisTurn; CR 700.13)
	// ExSelfPrepared: 1 if the evaluating source is prepared (CR 722.3a), else 0;
	// N = 1 negates (Count$ValidSelf Card.IsPrepared / Card.!IsPrepared)
	ExSelfPrepared
	ExThisTurnActGrave  // activations this turn by the controller whose source now sits in a graveyard (Count$ThisTurnActivated_Activated.YouCtrl+inZoneGraveyard)
	ExSneaked           // Sub[0] if the resolving spell was cast for its sneak cost, else Sub[1] (Count$ValidStack Card.Self+sneaked)
	ExTriggerSpellMVLKI // the mana value of the spell that triggered this SpellCast ability, as gorge's last known information (TriggeredSpellAbility$CardManaCostLKI)
	ExPoisonOf          // poison counters of player Ref (TargetedController$Counters.Poison)
	ExProduct           // Sub[0] * Sub[1] (an SVar-named /Times.Y modifier)
	ExAttackersAll      // attackers declared this turn by every player (PlayerCountPlayers$AttackersDeclared)
	ExRefCount          // the objects Ref names that match F where they now are (gorge evalRefProperty's Valid <spec>: TriggeredSource$Valid ...)
	// ExNumColorsOf: the colour count of object Ref (gorge
	// count_ref_property.go:238-249 CardNumColors, len(h.ObjectColors(o)):
	// the face's colours off the battlefield, so a SpellCast trigger's
	// spell on the stack reads its printed colours)
	ExNumColorsOf
	// ExTgtSpent: the mana spent to cast the spell the resolving spell's
	// target slot names, snapshotted as the resolution starts (0 with no
	// snapshot). The counter it targets removes the spell's entry before a
	// later condition can read the spend (gorge targetManaSpentLKI; EOE
	// Unravel).
	ExTgtSpent
	// ExThisTurnActExhaust: 1 if the evaluating controller has activated an
	// Exhaust ability this turn, else 0 (Count$ThisTurnActivated_
	// Activated.Exhaust+YouCtrl; the kernel keeps one bit per player, so a
	// second activation still reads 1). dirac/static_turnbits.go.
	ExThisTurnActExhaust
	// ExActivations: the resolving activated ability's source's non-mana
	// activations this turn, the resolving one included (gorge
	// ConditionActivationLimit$ GE<n>, Ctx.ActivationsThisTurn: the ability's
	// own count). It reads the object's per-turn census (object.acts), which
	// counts every non-mana activation of the object, so the lowering admits
	// only a card whose one non-mana activated ability is the counted one.
	// Dragon Whelp.
	ExActivations
	// ExLifeLostLastTurn: the life the evaluating controller lost during the
	// previous turn (gorge PlayerCountPropertyYou$LifeLostLastTurn, the
	// LifeLostThisTurn tally one turn back; 0 on the first turn).
	// dirac/turn.go latches it as the turn changes.
	ExLifeLostLastTurn
	ExCount_
)

// ExValidAgg's aggregate (Expr.N): gorge count_body_zone.go's $<prop>
// folds over the battlefield matches.
const (
	AggGreatestPower     = 1  // max derived power (0 when nothing matches)
	AggGreatestToughness = 2  // max derived toughness
	AggGreatestMV        = 3  // max mana value
	AggSumToughness      = 4  // total derived toughness
	AggSumMV             = 5  // total mana value
	AggDistinctPower     = 6  // distinct derived powers
	AggColors            = 7  // colours among the matches (live layer-5 colours)
	AggDistinctMV        = 8  // distinct mana values (ExGraveAgg only)
	AggDistinctNames     = 9  // distinct card names among the matches (CR 201.2)
	AggCardTypes         = 10 // distinct card types on the matches' faces (CR 205.2a)
	AggSumCounters       = 11 // total of counter kind Expr.Ctr over the matches (CR 122.1)
	AggPermTypes         = 12 // distinct permanent card types (artifact, battle, creature, enchantment, land, planeswalker) on the matches' faces (CR 110.4; ExGraveAgg only)
	AggCreatureTypes     = 13 // distinct current creature types among the matches (CR 205.3m), every one for a Changeling
)

// Expr is a computed amount: Kind over its operands, then *Mul (0 = 1),
// /Div (floor; 0 = 1), +Add, then negated when Neg.
type Expr struct {
	Kind ExprKind
	N    int16
	F    FilterID
	Ref  Ref
	Ctr  CounterKind
	Sub  [2]ValueRef
	Mul  int8
	Div  int8
	Add  int8
	Neg  bool
	Op   CmpOp // ExCompare's comparison

	// Appended by ExThisTurnEntered's operands.
	F2   FilterID // creature-base alternatives rewritten onto Card (gated by WasCreature)
	F3   FilterID // the spec's non-Creature alternatives
	F4   FilterID // dest=Battlefield, object off it: the card-base form of F
	From int8     // -1 = no from_ origin; else the pre-move Zone
	Prop uint8    // 0 = count; PropCardPower/Toughness/ManaCost sum the face value
}

const (
	PropNone          uint8 = iota
	PropCardPower           // gorge objectPropertyCardPower
	PropCardToughness       // gorge objectPropertyCardToughness
	PropCardManaCost        // gorge objectPropertyCardManaCost
)

// ExThisTurnCastSpec's Expr.N bits (gorge spellsCastThisTurnMatching).
const (
	CastSpecYou      = 1 << iota // the spec says "You" (gorge's substring test): only the controller's casts count
	CastSpecExclude              // !CastSaSource, and always for the count-level $CardTypes aggregate: every cast of the source object is excluded
	CastSpecHand                 // wasCastFromYourHand[ByYou]: only casts from the caster's hand count
	CastSpecNotHand              // !wasCastFromYourHand[ByYou]: only casts from anywhere but the caster's hand count
	CastSpecTreasure             // CastSa Spell.ManaFromTreasure: only casts that spent mana from a Treasure source
	CastSpecGrave                // bare wasCastFromGraveyard: only objects whose current cast flags say flashback (gorge CastFlags, read NOW)
	CastSpecExile                // wasCastFromExile: only pushes from exile (gorge PutOnStack.From == ZExile)
	// Appended by.
	CastSpecCtrlTrig // +ControlledBy TriggeredActivator: the object's current controller is the trigger's activating player
	CastSpecTypes    // count-level $CardTypes: the distinct card types among the matching objects, not their count
	CastSpecAsCast   // OnlyFirstSpell$ (gorge onlyFirstSpellUsed): a push of an Adventure half is read as that half, not as the creature face the card shows now; no push is dropped
)

// CondID indexes Table.Conds; 0 is "always".
type CondID uint16

// CondKind is a Cond's test. Append-only.
type CondKind uint8

const (
	CondAlways     CondKind = iota
	CondPresent             // the number of battlefield objects matching F compares (Op) with A
	CondValue               // A compares (Op) with B
	CondKicked              // the spell was kicked
	CondThreshold           // seven or more cards in the controller's graveyard
	CondYourTurn            // it is the controller's turn
	CondRefMatches          // the number of objects in Ref matching F compares (Op) with A
	CondPromised            // the gift was promised (the spell, or the permanent's spell as it enters; Card.Self+PromisedGift)
	CondAnd                 // both Sub[0] and Sub[1] hold
	CondCount_
)

// CmpOp is a comparison.
type CmpOp uint8

const (
	CmpNone CmpOp = iota
	CmpGE
	CmpLE
	CmpEQ
	CmpGT
	CmpLT
	CmpNE
)

// Cond is a condition; Neg inverts it.
type Cond struct {
	Kind CondKind
	F    FilterID
	Ref  Ref
	Op   CmpOp
	A, B ValueRef
	Neg  bool
	Sub  [2]CondID // CondAnd's operands (zero for every other kind)
}

// ExOppExtreme reads a figure over the controller's living opponents, walked
// in seat order (gorge effects/count_player.go opponentGroup,
// playerGroupCount, lifeExtreme and playerCountExtreme). N's low byte names
// the figure: OppExtAmount the opponents themselves (one each), OppExtLife a
// life total, OppExtHand the cards in hand, OppExtGrave the cards in the
// graveyard, OppExtValid the battlefield permanents matching F read from
// that opponent's side ("YouCtrl" is the opponent's). OppExtLowest in N
// takes the lowest, else the highest. An empty group reads 0. Explicit
// ordinal, the iota tail stays untouched.
const ExOppExtreme ExprKind = 220

// ExOppExtreme's N.
const (
	OppExtAmount = 1
	OppExtLife   = 2
	OppExtHand   = 3
	OppExtGrave  = 4
	OppExtValid  = 5
	OppExtLowest = 0x100
)

// ExStepIn is 1 while the game's current step is one of the kop.Step bits
// in N, else 0 (gorge ConditionPhases$ <list>; the step set is the ONE
// phase-name parser, engine/state.ParsePhases, mapped onto this numbering).
// The kernel numbers first-strike and regular combat damage as separate
// steps where gorge has one combat-damage step, so a gorge combat-damage
// step sets both bits. Explicit ordinal, the iota tail stays untouched.
const ExStepIn ExprKind = 221

// ExPhaseCount reads the turn's phase ledger (gorge Game.CombatsThisTurn,
// EndStepsThisTurn): N 0 the combat phases begun this turn (FirstCombat$ is
// == 1); N 1 the finished end steps (Count$FinishedEndOfTurnsThisTurn, the
// end steps begun, less the one being run while in the end step).
const ExPhaseCount ExprKind = 232

// PhaseCountCombats and PhaseCountEndSteps are the N of an ExPhaseCount.
const (
	PhaseCountCombats  = 0
	PhaseCountEndSteps = 1
)

// ExCastBattlefield is
// Count$LastStateBattlefieldWithFallback: the number of battlefield
// permanents matching CardDef.CastBF as the spell was cast (CR 601.2a: the
// battlefield is frozen the moment the spell reaches the stack, before
// targets, divisions and costs). beginCast freezes the count into the
// spell entry's trigMV (dirac/value_castbattlefield.go) and the expression
// reads it at resolution; an uncast source counts the live battlefield
// (gorge engine/effects/count_cast_battlefield.go).
const ExCastBattlefield ExprKind = 237

// ExCastFromGrave (high expr kind): Sub[0] if the resolving
// spell was cast with a graveyard-origin cast flag (gorge
// state.ObjectWasCastFromGraveyard: flashback, harmonize, jumpstart, escape),
// else Sub[1]. Both the ConditionPresent$ Card.wasCastFromGraveyard condition
// and the Count$wasCastFromGraveyard.<yes>.<no> branch head lower to it. It
// is a cast-flag read, not a zone read, and a stack copy reads false (CR
// 707.10): a copy is put on the stack, never cast.
const ExCastFromGrave ExprKind = 247

// ExConverge (high expr kind): the number of distinct colours
// (W U B R G) of mana spent to cast a spell (gorge Object.ConvergeColours,
// Count$Converge; the pay-time FlagConverged CastInfo). It reads the
// resolving spell's entry (its spell chain, and the permanent it becomes as
// it enters: K:etbCounter), or a self-ETB trigger's value bound as it
// triggers (Tf3ConvergeBound). A stack copy and an object that entered
// uncast read 0.
const ExConverge ExprKind = 245

// value heads that read an object or a number the kernel
// already tracks. The new ordinals are numbered explicitly, clear of the iota
// lists other files append to.

// ExTapPower: the tap-power total of the object Ref names: its layer-derived
// power plus every StatTapPower static that scopes the action (CR
// 702.184a's Station, Crew, Saddle) and matches it. Ref is the tapped
// creature (RefTriggerObj for Station, where the picked creature rides the
// ability entry), and the host the action's own permanent. This is the one
// read Station's PutCounter amount (vTapPower) and the Crew/Saddle payment
// share, so a Tapestry Warden contributes toughness and Sulu twice his power
// to both. Explicit ordinal clear of the iota tail. Kernel value_tappower.go.
const ExTapPower ExprKind = 222

// ExPow: Count$<base>/Pow.<operand>, the saturating integer power of the
// running value (gorge engine/effects/count_operand.go applyCountOp `Pow.`:
// logarithmic exponentiation, clamped at each multiply). Sub[0] is the base,
// Sub[1] the exponent. A negative exponent fails closed to 0 (gorge
// applyCountOpOperandOK returns not-evaluated for a negative Pow operand,
// and a Number$ literal carrying it fails closed). Kernel
// value_countheads.go. Mathemagics' Number$2/Pow.X.
const ExPow ExprKind = 230

// SacSumMV adds the sacrificed permanent's printed mana value to the
// resolution's sacrifice accumulator (kop.ExRemLKIPT N 3; gorge
// engine/effects/count_ref_property.go CardManaCost over the Sacrificed LKI
// capture, state.SacrificedInfo.ManaValue, CR 608.2g/h). It is a Sacrifice /
// trigger-cost B.N bit beside SacSumPower (2) and SacSumToughness (4);
// bit 1 is the remember marker.
const SacSumMV = 8

// ExUnlockedDoors: explicit ordinal, the iota tail stays untouched.
const ExUnlockedDoors ExprKind = 236

// CmpPrime: ExCompare's "Sub[0] is prime" (gorge count_body_simple.go isPrime).
const CmpPrime CmpOp = 64

// ExMod: explicit ordinal, the iota tail stays untouched.
// Sub[0] % N, the remainder with the dividend's sign (gorge count_operand.go
// applyCountOp Mod.N).
const ExMod ExprKind = 234

// ExCtrAdded: gorge Count$CountersAddedThisTurn <Kind> You
// <Spec> read as the gate every carrier uses (CheckSVar$ X, default GE1).
// It returns 1 when the reading controller put a matching counter this
// turn, else 0. X.N packs the kind class (CtrAddedKind*, bits 0-1) and the
// object form (CtrAddedForm*, bits 2-3; Card.EffectSource is not lowered, see dirac/kgen/value_countersadded.go). The layout keeps no amounts, so
// dirac/kgen lowers it only under a >= 1 comparison.
const ExCtrAdded ExprKind = 243

// ExDiscardedThisTurn: gorge Count$ via
// PlayerCountPropertyYou$CardsDiscardedThisTurn (engine/rules/host_turn_ledger.go
// CardsDiscardedThisTurn). The cards the reading controller discarded this
// turn, kept as one per-player tally in State.
const ExDiscardedThisTurn ExprKind = 241

// ExAdamant (explicit high expr kind): Count$Adamant_<n>.
// <Colour>.<yes>.<no> read as CastSA, "at least n mana of <Colour> was
// spent to cast this spell" (CR 702.5; gorge effects/count_body_heads.go
// evalAdamantBranch). N packs the colour index (bits 0-2, W U B R G = 0-4)
// and the threshold n (bits 3-4, 0-3: the spend counts saturate at 3);
// Sub[0] is the value when it holds and Sub[1] when it does not. It is
// lowered only in the header (CheckSVar$) of a self-ETB trigger. As the
// object enters it reads the resolving spell's spend (stackEntry.spc); once
// queued the colour's count is bound into the pending trigger's x, which the
// resolution recheck reads (kernel kw_evoke.go).
const ExAdamant ExprKind = 244

// ExExiledWithN: the cards exiled with the reading source, counted over every
// seat's exile zone (gorge exiledWithSet; Count$ExiledWith$Amount).
const ExExiledWithN ExprKind = 246

// High expr kinds, numbered explicitly clear of the iota
// tail in table.go. Both read the per-turn zone-add ring for entries
// that LEFT a zone.
const (
	// ExRevolt: 1 iff a permanent that left the battlefield this turn is
	// controlled by the expression's "you" now (gorge revoltThisTurn reads
	// Object.Controller after the move, which gorge resets to the owner off
	// the battlefield; tokens count; CR 603.4 intervening-if, CR 400.7).
	ExRevolt ExprKind = 248
	// ExLeftZone: the count of DISTINCT objects among this turn's zone-add
	// entries whose origin is N (the kop.Zone, Graveyard) that match F where
	// they now live (gorge Count$LeftGraveyardThisTurn,
	// evalCov3HeadLeftZoneThisTurn).
	ExLeftZone ExprKind = 249
)

// ExLowestLife (high expr kind): the lowest CURRENT life
// total among the living players (gorge PlayerCountPlayers$LowestLifeTotal,
// lifeExtreme over AliveFrom; in two players min(life[0], life[1]), read as
// the replacement applies). The DSK "enters tapped unless a player has 13 or
// less life" lands' ConditionCheckSVar$ X with ConditionSVarCompare$ GT13.
const ExLowestLife ExprKind = 252

// ExRemLKIPT: RememberedLKI$CardPower / $CardToughness over a
// Sacrifice RememberSacrificed$ set (gorge count_ref_property.go, CR 608.2h):
// the sum of the power (N 0) or toughness (N 1) the creatures this
// resolution sacrificed had just before they left. The Sacrifice's B.N bit 1
// (power sum) or bit 2 (toughness sum) makes the kernel accumulate it
// (resCtx.remLK); kgen admits a program that reads exactly one of the two.
const ExRemLKIPT ExprKind = 240

// Sacrifice B.N bits beside the remember bit 1.
const (
	SacSumPower     = 2
	SacSumToughness = 4
)

// ExSacrificedThisTurn: gorge Count$ via
// PlayerCountPropertyYou$SacrificedThisTurn <Permanent|Artifact>
// (engine/effects/count_cov3.go sacrificedThisTurn). The permanents the
// reading controller sacrificed this turn: X.N 0 every one, X.N 1 the
// artifact cards among them. Kept as two per-player tallies in State.
const ExSacrificedThisTurn ExprKind = 242

// ExScriedThisTurn: gorge Count$YouScryThisTurn /
// Count$YouSurveilThisTurn (engine/effects/count_body_paid.go,
// rules/scry_surveil_this_turn.go ScriedThisTurn / SurveilledThisTurn). The
// scry (X.N 0) or surveil (X.N 1) instructions the reading controller
// performed this turn. Kept as two per-player tallies in State.
const ExScriedThisTurn ExprKind = 238

// ExDilemmasFaced: gorge Count$YouFaceDilemmaThisTurn (Player.DilemmasFacedThisTurn):
// the times the reading controller chose modes this turn.
const ExDilemmasFaced ExprKind = 239

// ExChosenMode: the evaluating source permanent's chosen
// mode index (gorge Card.ChosenMode<X>, effects/filter.go: the permanent's
// ChosenModes holds X), 255 when the source is off the battlefield or has
// answered no mode. Explicit ordinal, the iota tail stays untouched.
const ExChosenMode ExprKind = 233

// ExChosenNumber: the number the evaluating source permanent chose as it
// entered (gorge "Chosen" in a filter bound, Object.ChosenNumber through
// specCtx's Resolve; Nyota Uhura, Communications' cmcEQChosen), 0 when it
// chose none or is off the battlefield (gorge's zero value). Explicit
// ordinal, the iota tail stays untouched.
const ExChosenNumber ExprKind = 228

// ExCountHand (high expr kind): the cards in every hand matching
// F (gorge Count$ValidHand <spec>, a zone count over each seat's hand; YouOwn
// narrows it to the controller's). Land Grant's "no land cards in hand".
const ExCountHand ExprKind = 250

// ExYourTurns (high expr kind): N 0, the turns begun with the
// controller as the active player, the current one included (gorge
// Count$YourTurns, Host.TurnsTaken); N 1, 1 while it is the controller's
// turn, else 0 (gorge PlayerCountPropertyYou$HasPropertyActive). Starting
// Town's "unless it's your first, second, or third turn of the game".
const ExYourTurns ExprKind = 251

// ExRedNCDmg (high expr kind): the noncombat damage dealt this
// turn by sources the controller controlled that were red when it was dealt,
// to any recipient (gorge Count$NonCombatDamageThisTurn Card.Red+YouCtrl Any,
// the DamageDealtThisTurn ledger read at the colour and controller recorded
// with each hit; prevented damage never lands, so it never counts). Ojer
// Axonil's Temple of Power.
const ExRedNCDmg ExprKind = 253

// ExAttackedTurn: the objects declared as attackers this turn that match F
// where each now is (gorge Count$CreaturesAttackedThisTurn <spec>, count_cov3.go
// evalCov3HeadCreaturesAttackedThisTurn). A battlefield object is matched as a
// permanent, one in another zone as a card. Avengers Assemble!, Thaumaton
// Torpedo.
const ExAttackedTurn ExprKind = 235

// ExSacrificedThisTurn N 2 is the Food cards (printed subtype) sacrificed
// this turn; ExOppsLostLife N 1 is the opponent dealt combat damage this turn
// by a legendary creature (gorge PlayerCountRegisteredOpponents$
// HasPropertywasDealtCombatDamageThisTurnBy Creature.Legendary GE1).
const (
	SacrificedFood   = 2
	OppsHitLegendary = 1
)

// ExChromaOf: the mana symbols of colour N (W U B R G = 0..4) in the printed
// mana cost of object Ref (gorge TriggeredCard$ChromaSource.<Colour>, Forge's
// getTotalChroma: a hybrid or Phyrexian symbol counts once for either of its
// colours); a face-down object has no mana cost (CR 708.2) and counts none.
// A SpellCast trigger's spell on the stack is the one reader.
const ExChromaOf ExprKind = 231

// ExOppsAttacked (high expr kind): the number of opponents the controller
// attacked this turn (gorge Engine.OpponentsAttackedThisTurn,
// PlayerCountPropertyYou$OpponentsAttackedThisTurn): the distinct players a
// declared attack named as its defender, so an attacker aimed at a
// planeswalker or battle does not count its controller. Only the active
// player attacks, so any other controller reads 0. With one opponent the
// value is 0 or 1.
const ExOppsAttacked ExprKind = 229

// ExIfCastOwnMain (high expr kind): Sub[0] if the resolving or announcing
// spell was cast, is read in a main phase and its controller is the active
// player, else Sub[1] (gorge Count$IfCastInOwnMainPhase.<yes>.<no>). The read
// is live, not a stamp of the cast's phase: a spell cast in a main phase that
// resolves in another step reads Sub[1]. A stack copy was never cast.
const ExIfCastOwnMain ExprKind = 224

// ExPlExtreme (low expr kind, the first free ordinal past ExCount_): the
// highest, over a living player group walked in seat order, of the count of
// the group member's objects in the named zones that match F, each member
// read from their OWN side (gorge effects/count_player.go
// playerCountExtremeZones: the "YouCtrl"/"YouOwn" qualifiers of the spec
// name the counted member, c.SpecContext(p); F.Ctrl = the member).
// PlayerCountPlayers$HighestValidGraveyard Card.YouOwn (Jace, the Perfected
// Mind's Y, Visions of Beyond), PlayerCountPlayers$HighestValidGraveyard,
// Library,Exile Card.IsRemembered+YouOwn (Windfall, Jace's Archivist) and
// PlayerCountOpponents$HighestValidGraveyard Creature.YouOwn (Nautiloid
// Ship). An empty group (only at the game's end) reads 0: a count always
// has one answer. Only the Highest form is lowered; gorge's Lowest extreme
// stays refused. F spans the named zones (kop.ZoneNone, no zone default).
const ExPlExtreme ExprKind = 88

// ExPlExtreme's N: PlExtPlayers or PlExtOpponents in bits 0..1, the named
// zones above them. Explicit ordinals, the iota tail stays untouched.
const (
	PlExtPlayers   = 0 // every living player, the evaluating controller included
	PlExtOpponents = 1 // the evaluating controller's living opponents
	PlExtGrave     = 1 << 8
	PlExtLibrary   = 1 << 9
	PlExtExile     = 1 << 10
)

// ExVoid (high expr kind): Sub[0] if the Void of Edge of
// Eternities holds, else Sub[1] (gorge Count$Void.<yes>.<no>,
// cov3BranchHoldsVoid). Void holds when a nonland permanent left the
// battlefield this turn (any player's, to any zone; CR 400.7: the object is
// read where it now lives) or a spell was cast for its warp cost this turn
// (any player's; CR 702.185). It has no filter: gorge strips the
// "CastSa Spell.Warp" token per cast from that cast's own pay-time flags.
const ExVoid ExprKind = 254

// ExFirstCast (high expr kind): Sub[0] if the resolving spell was the first
// spell its controller cast this game, else Sub[1]. Forge's static SpellCast
// trigger writes Count$YouCastThisGame into an SVar the moment the spell is
// cast (before a response can add casts), and the chain compares it with
// EQ1; the flag is stamped at cast (dirac/cast_firstcast.go), never counted
// at resolution. A stack copy keeps the original's stamp, as gorge's copy
// reads the original object's stored SVar.
const ExFirstCast ExprKind = 255

// ExYourExtraTurns: 1 if the controller has begun an extra turn this game,
// else 0 (gorge Count$YourExtraTurns, Host.ExtraTurnsBegun: one per extra
// turn actually begun with the player active, CR 500.7, not per grant).
// genState.extraTurnBegun is the per-seat bit set when an extra turn's
// TurnChange fires. H.M.S. Bounty, Unlikely Timeship.
const ExYourExtraTurns ExprKind = 227

// ExCounterKinds: the number of distinct counter kinds with a positive
// count among the battlefield objects matching F (gorge
// Count$DifferentCounterKinds_<spec>); gorge's own status markers (the
// regeneration shield, the deathtouch mark) are no counter kinds here, and
// a Class level is no counter. Explicit ordinal, the iota tail stays
// untouched.
const ExCounterKinds ExprKind = 225

// ExTargetPlayers: 1 when target slot Ref holds a player, else 0 (an unfilled
// optional slot, or one whose player is no longer a legal target on
// resolution, CR 608.2b). gorge TargetedPlayer$Amount over a slot of an
// opponents-only ValidTgts$ in a two-player game, where the slot holds at
// most the one opponent. Explicit ordinal, the iota tail stays untouched.
const ExTargetPlayers ExprKind = 223

// ExNotOwnLibMV: the summed mana value of the library cards in the
// resolution's remembered set that the repeated player (an OpRepeatEachPlayer
// in progress) does not own: Parker Luck's Count$ValidLibrary
// Card.IsRemembered+!RememberedPlayerOwn$CardManaCost. No parameters.
// Explicit ordinal, the iota tail stays untouched.
const ExNotOwnLibMV ExprKind = 226

// ExDmgAmountReceived: the positive damage the source object received this
// turn, saturating at 255 (gorge Object.DamageReceivedThisTurn,
// Count$TotalDamageReceivedThisTurn). A source with no record reads 0.
// Explicit ordinal, the iota tail stays untouched. Kernel value_dmgamount.go.
const ExDmgAmountReceived ExprKind = 219
