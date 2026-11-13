package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// The lowered IR (LIR): the generator's own form of a card, one step from
// the kop encoding. Lowering (lower*.go) turns gorge's compiled IR into LIR
// or refuses; emission (emit.go) turns LIR into kop composite literals. The
// LIR names ops, refs and values symbolically so a refusal or a dump can say
// what it means; emission maps each symbol to its kop ordinal and refuses a
// symbol kop lacks.

// refKind names who or what an instruction acts on (kop.Ref).
type refKind uint8

const (
	rNone                 refKind = iota
	rTarget                       // target slot N of this ability
	rSelf                         // the source object
	rYou                          // the source's controller
	rEachOpp                      // each opponent
	rEachPlayer                   // each player
	rTargetController             // controller of the object in target slot N
	rTargetOwner                  // owner of the object in target slot N
	rTriggered                    // the triggering object (as it last existed / now is)
	rTriggeredNew                 // the triggering object in the zone it moved to
	rTriggeredSource              // the source of the triggering damage
	rTriggeredPlayer              // the player the trigger names (activator / controller)
	rTriggeredTgtCtrl             // the triggering referent's controller, read live at resolution (gorge TriggeredTargetController; a dead referent reads its owner)
	rEquipped                     // the object the source is attached to
	rRemembered                   // the resolution's remembered objects
	rDelayedRemembered            // the objects a delayed trigger remembered
	rReplaced                     // the object an ETB replacement is modifying
	rChosen                       // the chosen card(s)
	rEach                         // the RepeatEach loop variable
	rSacrificed                   // the object sacrificed to pay the cost
	rHost                         // the permanent that granted this ability (OriginalHost)
	rDefending                    // the defending player of an attack trigger the controller's attack fires
	rTargetUnion                  // every target of the slots in bit mask N, in slot order (gorge's chain target union)
	rRememberedController         // the remembered object's controller (its RememberLKI$ capture off the battlefield)
	rDiscarded                    // the card a spell's Discard<1/...> cost discarded (Grab the Prize)
	rLibTop                       // the top card of the controller's library (Defined$ TopOfLibrary)
	rOppNonRemCtrl                // the opponent unless a remembered object's controller is that opponent (OppNonRememberedController)
	rSrcOwner                     // the source object's owner (Defined$ CardOwner)
	rTriggerAttackers             // the attacker group of the AttackersDeclared event (Defined$ TriggeredAttackers)
	rRememberedOwner              // the remembered object's owner (Unidentified Hovership's leave trigger)
	rImprinted                    // the resolution's imprinted objects (Dig Imprint$ on a permanent's ability)
	rPromised                     // the opponent the gift was promised to (Defined$ / TokenOwner$ Promised)
	rAttachedCtrl                 // the controller of the object the source is attached to, read live (Contaminated Bond's blocker)
	rLastCreated                  // the token(s) the previous CreateToken/copy op minted (kop.RefLastCreated; Encore's duty on the minted copy)
	rTaxedAttacker                // the creature an attack tax is being priced for (kop.RefTaxedAttacker; Nils's RememberingAttacker$ charge, lower_attacktax.go)
	rCount
)

var refNames = [...]string{"None", "Target", "Self", "You", "EachOpp", "EachPlayer", "TargetController",
	"TargetOwner", "Triggered", "TriggeredNew", "TriggeredSource", "TriggeredPlayer", "TriggeredTgtCtrl", "Equipped", "Remembered",
	"DelayedRemembered", "Replaced", "Chosen", "Each", "Sacrificed", "Host", "Defending", "Targets", "RememberedController", "Discarded", "LibTop", "OppNonRemCtrl", "SrcOwner", "TriggerAttackers", "RememberedOwner", "Imprinted", "Promised", "AttachedCtrl", "LastCreated", "TaxedAttacker"}

type lRef struct {
	Kind refKind
	N    int // target slot for rTarget / rTargetController / rTargetOwner
}

func (r lRef) String() string {
	switch r.Kind {
	case rTarget, rTargetController, rTargetOwner:
		return fmt.Sprintf("%s%d", refNames[r.Kind], r.N)
	case rTargetUnion:
		out := refNames[r.Kind]
		for i := 0; i < 31; i++ {
			if r.N&(1<<i) != 0 {
				out += fmt.Sprint(i)
			}
		}
		return out
	}
	return refNames[r.Kind]
}

// valKind names an amount's source (kop.ValueRef).
type valKind uint8

const (
	vNone valKind = iota
	vConst
	vX                   // the X paid
	vCount               // number of battlefield objects matching F
	vCountGraveyard      // number of graveyard cards matching F
	vSumPower            // total power of battlefield creatures matching F
	vDistinctCMC         // number of distinct mana values among battlefield objects matching F
	vKicked              // Sub[0] if the spell was kicked, else Sub[1]
	vMorbid              // Sub[0] if a creature died this turn, else Sub[1]
	vAttackedThisTurn    // number of creatures the controller attacked with this turn
	vPowerOf             // power of Ref
	vToughnessOf         // toughness of Ref
	vLifeOf              // life total of player Ref
	vLibrarySizeOf       // cards in player Ref's library
	vStartingLife        // the controller's starting life total
	vTriggerAmount       // the triggering event's amount (damage dealt / life gained or lost)
	vLifeGainedThisTurn  // life the controller gained this turn
	vOppLifeLostThisTurn // life the opponents lost this turn
	vCountersOn          // counters of kind S on Ref
	vCreaturesDied       // creatures put into a graveyard from the battlefield this turn
	vRememberedCount     // size of the remembered set
	vExcessDamage        // excess damage dealt by the previous DealDamage
	vLastLifeLost        // life lost by the previous LoseLife
	vTokensCreated       // tokens created by the previous CreateToken (RememberOriginalTokens)
	vCountCast           // spells cast this turn matching F (refused: copy machinery)
	vLifeLostThisTurn    // life the controller lost this turn
	vSum                 // Sub[0] + Sub[1]
	vHandSizeOf          // cards in player Ref's hand
	vCountExile          // exiled cards matching F
	vOppsLostLife        // opponents who lost life this turn
	vMVOf                // mana value of object Ref
	vCountAtLeast        // Sub[0] if battlefield objects matching F number at least N, else Sub[1]
	vPromisedGift        // Sub[0] if the gift was promised, else Sub[1] (Count$PromisedGift)
	vResolvedThisTurn    // times the resolving triggered ability resolved this turn, this one included
	vThisTurnCast        // spells player Ref cast this turn: N=0 every spell, N=1 instants and sorceries
	vRollResult          // the result of the ability's die roll (RollDice ResultSVar$)
	vCastManaSpent       // the mana spent to cast the spell (Count$CastTotalManaSpent)
	vCompare             // Count$Compare: Sub[0] compared (Op) with N picks Sub[1]'s then/else arm
	vBranches            // vCompare's arms: Sub[0] then, Sub[1] else
	vDevotion            // the controller's devotion to the colours of mask N (Count$Devotion)
	vParty               // the controller's party size (Count$Party)
	vDrawnThisTurn       // cards the controller drew this turn (Count$YouDrewThisTurn)
	vGraveTypes          // card types among the cards in the controller's graveyard (delirium)
	vValidAgg            // aggregate N (kop.Agg*) over the battlefield objects matching F
	vGraveAgg            // mana-value aggregate N (kop.Agg*) over the graveyard cards matching F
	vYourSpeed           // the controller's speed (Count$YourSpeed; CR 702.179)
	vHighestOppPoison    // the highest poison count among the controller's living opponents
	vThisTurnEntered     // per-ADD zone entries this turn (Count$ThisTurnEntered_...)
	vThisTurnCastSpec    // spells cast this turn matching F where they are now: N the kop.CastSpec* bits
	vCommittedCrime      // Sub[0] if the controller committed a crime this turn, else Sub[1] (Count$CommittedCrimeThisTurn)
	vThisTurnActGrave    // activations this turn by the controller whose source now sits in a graveyard
	vSelfPrepared        // 1 if the source is prepared, else 0; N=1 negates (Count$ValidSelf Card.[!]IsPrepared)
	vSneaked             // Sub[0] if the resolving spell was cast for its sneak cost, else Sub[1]
	vTriggerSpellMVLKI   // the mana value of the spell that triggered a SpellCast ability, as gorge's LKI
	vPoisonOf            // poison counters of player Ref
	vProduct             // Sub[0] * Sub[1]
	vAttackersAll        // attackers declared this turn by every player
	vRefCount            // the objects Ref names matching F where they now are (TriggeredSource$Valid <spec>)
	vCountHand           // hand cards matching F, every hand (Count$ValidHand <spec>)
	vYourTurns           // N 0: the controller's turns begun (Count$YourTurns); N 1: 1 on the controller's turn (PlayerCountPropertyYou$HasPropertyActive)
	vLowestLife          // the lowest CURRENT life total among the living players (PlayerCountPlayers$LowestLifeTotal)
	vRedNCDmg            // the noncombat damage dealt this turn by sources you controlled that were red (Count$NonCombatDamageThisTurn Card.Red+YouCtrl Any)
	vVoid                // Sub[0] if a nonland permanent left the battlefield this turn or a spell was cast for its warp cost this turn, else Sub[1] (Count$Void.<yes>.<no>)
	vRevolt              // 1 if a permanent that left the battlefield this turn is controlled by the controller now (Revolt$ True)
	vLeftZone            // distinct objects whose zone-add entry left Zone this turn and match F where they now live (Count$LeftGraveyardThisTurn)
	vCastFromGrave       // Sub[0] if the resolving spell was cast from a graveyard (a graveyard-origin cast flag or its cast zone), else Sub[1] (ConditionPresent$ Card.wasCastFromGraveyard; Count$wasCastFromGraveyard.<yes>.<no>)
	vCtrAdded            // 1 if the controller put a matching counter this turn; N packs kop.CtrAddedKind* | CtrAddedForm*<<2 (Count$CountersAddedThisTurn <Kind> You <Spec>, read >= 1 only)
	vExiledWithN         // the cards exiled with the reading source, every seat's exile zone (ExiledWith$Amount)
	vConverge            // the colours of mana spent to cast the spell (Count$Converge)
	vExileTypes          // the distinct card types among the exiled cards matching F, every seat's exile (Count$ValidExile <spec>$CardTypes)
	vSacrificedThisTurn  // the permanents the controller sacrificed this turn; N 0 every one, N 1 artifact cards (PlayerCountPropertyYou$SacrificedThisTurn Permanent|Artifact)
	vDiscardedThisTurn   // the cards the controller discarded this turn (PlayerCountPropertyYou$CardsDiscardedThisTurn)
	vNumColorsOf         // the colour count of object Ref (TriggeredCard$CardNumColors in a SpellCast trigger)
	vScriedThisTurn      // the scry (N 0) or surveil (N 1) instructions the controller performed this turn (Count$YouScryThisTurn / Count$YouSurveilThisTurn)
	vRemLKIPT            // RememberedLKI$CardPower (N 0) / $CardToughness (N 1): the sum over the creatures a Sacrifice RememberSacrificed$ took, read from before they left
	vCastBattlefield     // the battlefield permanents matching the card's CastBF as the spell was cast (Count$LastStateBattlefieldWithFallback <spec>)
	vUnlockedDoors       // Count$UnlockedDoors: unlocked Room doors the controller has
	vChosenMode          // the source permanent's chosen mode index, 255 when unanswered / off the battlefield (ChosenMode<X>)
	vMod                 // Sub[0] % N (a /Mod.N value modifier)
	vPow                 // Sub[0] raised to Sub[1], saturating (a /Pow.<operand> modifier)
	vAttackedTurn        // the objects declared as attackers this turn that match F where each now is (Count$CreaturesAttackedThisTurn <spec>)
	vPhaseCount          // N 0: the combats begun this turn; N 1: the finished end steps (Count$FinishedEndOfTurnsThisTurn)
	vAdamant             // Sub[0] if at least n mana of a colour was spent to cast the source, else Sub[1]; N packs colour | n<<3 (CastSA>Count$Adamant_<n>.<Colour>.<yes>.<no>, a self-ETB trigger's CheckSVar$ only)
	vDilemmasFaced       // the times the controller chose modes this turn (Count$YouFaceDilemmaThisTurn)
	vChromaOf            // the symbols of colour N in the printed cost of object Ref (TriggeredCard$ChromaSource.<Colour> in a SpellCast trigger)
	vOppsAttacked        // opponents the controller attacked this turn (PlayerCountPropertyYou$OpponentsAttackedThisTurn)
	vChosenNumber        // the number the source permanent chose as it entered, 0 when none (a filter bound "Chosen": cmcEQChosen)
	vFirstCast           // Sub[0] if the resolving spell was its controller's first cast this game, else Sub[1] (the static SpellCast trigger's StoreSVar Count$YouCastThisGame read by ConditionSVarCompare$ EQ1)
	vYourExtraTurns      // 1 if the controller has begun an extra turn this game, else 0 (Count$YourExtraTurns)
	vNotOwnLibMV         // the mana values of the remembered library cards the repeated player does not own (Count$ValidLibrary Card.IsRemembered+!RememberedPlayerOwn$CardManaCost, a RepeatEach body)
	vCounterKinds        // the distinct counter kinds among the battlefield objects matching F (Count$DifferentCounterKinds_<spec>)
	vTargetPlayers       // 1 when target slot Ref holds a player, else 0 (TargetedPlayer$Amount of an opponents-only slot)
	vIfCastOwnMain       // Sub[0] if the controller is the active player in a main phase and the source spell was cast, else Sub[1] (Count$IfCastInOwnMainPhase.<yes>.<no>)
	vPWTypes             // the distinct planeswalker subtypes printed on the battlefield planeswalkers matching F (Count$Valid <spec>$PlaneswalkerType)
	vTapPower            // the tap-power total of Ref (a Station's amount: power plus every scoped StatTapPower bonus)
	vStepIn              // 1 while the current step is one of the kop.Step bits in N (ConditionPhases$)
	vTgtSpent            // the mana spent to cast the spell the resolving spell targets (Targeted$CastTotalManaSpent)
	vOppExtreme          // a figure over the controller's living opponents, kop.OppExt* in N, F for OppExtValid (PlayerCountOpponents$)
	vThisTurnActExhaust  // 1 if the controller activated an Exhaust ability this turn (Count$ThisTurnActivated_Activated.Exhaust+YouCtrl)
	vActivations         // the resolving activated ability's source's non-mana activations this turn, this one included (ConditionActivationLimit$)
	vLifeLostLastTurn    // life the controller lost during the previous turn (PlayerCountPropertyYou$LifeLostLastTurn)
	vLastDamage          // damage the spell's previous DealDamage landed (kop.VLastDamage; value_totaldamage.go)
	vDmgAmount           // the positive damage the source received this turn (Count$TotalDamageReceivedThisTurn)
	vTimesKicked         // CR 702.43's multikicker payment count (Count$TimesKicked; kop.ExKicked N 1)
	vMonarch             // Sub[0] if the controller (N 0) or an opponent (N 1) is the monarch, else Sub[1] (Count$Monarch.<yes>.<no>, CheckDefinedPlayer$ You|Opponent.isMonarch; lower_monarch.go)
	vEnergyOf            // energy counters of player Ref (Count$YourCountersEnergy)
	vPlExtreme           // the highest, over a player group, of the member's filtered count in the named zones (PlayerCountPlayers/Opponents$HighestValid<zones>)
	vJoinForces          // the resolution's join-forces total (kop.ExJoinForces; lower_storesvar.go)
	vRemEachOwnNonLand   // the remembered cards that are nonland and owned by the repeated player (Remembered$Valid Card.nonLand+RememberedPlayerOwn, a RepeatEach body's ConditionCheckSVar$ gate)
	vCount_
)

var valNames = [...]string{"None", "Const", "X", "Count", "CountGraveyard", "SumPower", "DistinctCMC", "Kicked",
	"Morbid", "AttackedThisTurn", "PowerOf", "ToughnessOf", "LifeOf", "LibrarySizeOf", "StartingLife",
	"TriggerAmount", "LifeGainedThisTurn", "OppLifeLostThisTurn", "CountersOn", "CreaturesDied",
	"RememberedCount", "ExcessDamage", "LastLifeLost", "TokensCreated", "CountCast", "LifeLostThisTurn", "Sum",
	"HandSizeOf", "CountExile", "OppsLostLife", "MVOf", "CountAtLeast", "PromisedGift", "ResolvedThisTurn", "ThisTurnCast", "RollResult", "CastManaSpent", "Compare", "Branches", "Devotion", "Party", "DrawnThisTurn", "GraveTypes", "ValidAgg", "GraveAgg", "YourSpeed", "HighestOppPoison", "ThisTurnEntered", "ThisTurnCastSpec", "CommittedCrime", "ThisTurnActGrave", "SelfPrepared", "Sneaked", "TriggerSpellMVLKI", "PoisonOf", "Product", "AttackersAll", "RefCount", "CountHand", "YourTurns", "LowestLife", "RedNCDmg", "Void", "Revolt", "LeftZone", "CastFromGrave", "CtrAdded", "ExiledWithN", "Converge", "ExileTypes", "SacrificedThisTurn", "DiscardedThisTurn", "NumColorsOf", "ScriedThisTurn", "RemLKIPT", "CastBattlefield", "UnlockedDoors", "ChosenMode", "Mod", "Pow", "AttackedTurn", "PhaseCount", "Adamant", "DilemmasFaced", "ChromaOf", "OppsAttacked", "ChosenNumber", "FirstCast", "YourExtraTurns", "NotOwnLibMV", "CounterKinds", "TargetPlayers", "IfCastOwnMain", "PWTypes", "TapPower", "StepIn", "TgtSpent", "OppExtreme", "ThisTurnActExhaust", "Activations", "LifeLostLastTurn", "LastDamage", "DmgAmount", "TimesKicked", "Monarch", "EnergyOf", "PlExtreme", "JoinForces", "RemEachOwnNonLand"}

// lValue is an amount: Kind with its operand, then Mul/Div/Add applied in
// that order, then negated when Neg.
type lValue struct {
	Kind valKind
	N    int // vConst
	F    *lFilter
	F2   *lFilter // vThisTurnEntered: the Creature-base alternatives rewritten onto Card
	F3   *lFilter // vThisTurnEntered: the spec's other alternatives
	F4   *lFilter // vThisTurnEntered: the card form for a dest=Battlefield entry off the battlefield
	Ref  lRef
	S    string // counter kind for vCountersOn
	Sub  []lValue
	Mul  int // 0 = 1
	Div  int // 0 = 1 (floor)
	Add  int
	Neg  bool
	Op   uint8 // vCompare: opGE..opLT
	Zone uint8 // vThisTurnEntered: the destination kop.Zone
	From int8  // vThisTurnEntered: -1 = no origin
	Prop uint8 // vThisTurnEntered: kop.PropNone or a PropCard* sum
}

func cst(n int) lValue { return lValue{Kind: vConst, N: n} }

func (v lValue) isConst() bool {
	return v.Kind == vConst && v.Mul == 0 && v.Div == 0 && v.Add == 0 && !v.Neg
}

func (v lValue) String() string {
	if v.Kind == vNone {
		return "-"
	}
	var b strings.Builder
	if v.Neg {
		b.WriteString("-")
	}
	switch v.Kind {
	case vConst:
		fmt.Fprintf(&b, "%d", v.N)
	default:
		b.WriteString(valNames[v.Kind])
		var args []string
		if v.F != nil {
			args = append(args, v.F.String())
		}
		switch v.Kind {
		case vValidAgg:
			args = append(args, fmt.Sprintf("agg=%d", v.N))
		case vCompare:
			args = append(args, fmt.Sprintf("op=%d,n=%d", v.Op, v.N))
		case vSacrificedThisTurn, vScriedThisTurn, vDilemmasFaced, vRemLKIPT, vPhaseCount, vAdamant, vMonarch:
			args = append(args, fmt.Sprintf("n=%d", v.N))
		case vCountGraveyard, vHandSizeOf, vOppsLostLife:
			if v.N != 0 {
				args = append(args, fmt.Sprintf("n=%d", v.N)) // N 1: the cards, not the raw zone length (a hand token is no card); vOppsLostLife N 1 legendary hit, N 2 any combat hit
			}
		}
		if v.Ref.Kind != rNone {
			args = append(args, v.Ref.String())
		}
		if v.S != "" {
			args = append(args, v.S)
		}
		for _, s := range v.Sub {
			args = append(args, s.String())
		}
		if len(args) > 0 {
			b.WriteString("(" + strings.Join(args, ",") + ")")
		}
	}
	if v.Mul != 0 {
		fmt.Fprintf(&b, "*%d", v.Mul)
	}
	if v.Div != 0 {
		fmt.Fprintf(&b, "/%d", v.Div)
	}
	if v.Add != 0 {
		fmt.Fprintf(&b, "%+d", v.Add)
	}
	return b.String()
}

// condKind is a branch condition (kop.Cond).
type condKind uint8

const (
	cNone          condKind = iota
	cPresent                // count of battlefield objects matching F compares (Op,N) — default GE 1
	cValue                  // value A compares (Op) with value B
	cKicked                 // the spell was kicked
	cThreshold              // the controller has seven or more cards in their graveyard
	cYourTurn               // it is the controller's turn
	cRefMatches             // the object(s) Ref match F (any of them); Op/N compare the matching count
	cCostPaid               // the optional cost of this ability was paid ("if you do")
	cOpponentsTurn          // it is not the controller's turn
	cPromisedGift           // the gift was promised (Card.Self+PromisedGift on the stack, or the entering permanent)
	cAnd                    // both Sub conditions hold
	cEchoOwed               // the source came under its controller's control since the beginning of their most recent upkeep (echo's gate)
	cSourceZone             // the source object is in Zone (source-identity IsPresent$ PresentZone$)
	cCount_
)

var condNames = [...]string{"None", "Present", "Value", "Kicked", "Threshold", "YourTurn", "RefMatches",
	"CostPaid", "OpponentsTurn", "PromisedGift", "And", "EchoOwed", "SourceZone"}

type lCond struct {
	Kind condKind
	F    *lFilter
	Ref  lRef
	Op   uint8
	A, B lValue
	Sub  []lCond
	Neg  bool
	Zone uint8 // cSourceZone: the source's zone
}

func (c lCond) String() string {
	if c.Kind == cNone {
		return "true"
	}
	s := condNames[c.Kind]
	var args []string
	for _, sub := range c.Sub {
		args = append(args, sub.String())
	}
	if c.F != nil {
		args = append(args, c.F.String())
	}
	if c.Ref.Kind != rNone {
		args = append(args, c.Ref.String())
	}
	if c.Op != 0 {
		args = append(args, fmt.Sprintf("op%d", c.Op))
	}
	if c.Kind == cSourceZone {
		args = append(args, zoneLabel[c.Zone])
	}
	if c.A.Kind != vNone {
		args = append(args, c.A.String())
	}
	if c.B.Kind != vNone {
		args = append(args, c.B.String())
	}
	if c.Neg {
		s = "!" + s
	}
	return s + "(" + strings.Join(args, ",") + ")"
}

// andCond composes a and b into a conjunction (cNone is the identity).
func andCond(a, b lCond) lCond {
	if a.Kind == cNone {
		return b
	}
	if b.Kind == cNone {
		return a
	}
	return lCond{Kind: cAnd, Sub: []lCond{a, b}}
}

// lInstr is one instruction: Op on Obj with amounts A, B, the op-specific
// constant C (symbolic in CS until emission), filter F, a condition (for
// the branch ops) and Skip (the branch length, patched after lowering the
// guarded block).
type lInstr struct {
	Op   string
	Obj  lRef
	A, B lValue
	C    int
	CS   string // symbolic constant: keyword set, counter kind, token key, zone, ...
	Obj2 lRef   // a second object (AttachObj: the object attached)
	// W: a player-valued side input. OpTapOrUntap/OpTap: the Tapper$
	// provenance (gorge effTapOrUntap's Tapper$ read, the tap's trigger
	// player); OpAttach: the player DESTINATION of an Enchant:Player aura
	// (gorge effAttach's "attach to player" event, defined() player arms).
	W    lRef
	F    *lFilter
	Cond lCond
	Skip int
	Note string // semantic note carried into the dump (not emitted)
}

func (in lInstr) String() string {
	var parts []string
	parts = append(parts, in.Op)
	if in.Obj.Kind != rNone {
		parts = append(parts, "obj="+in.Obj.String())
	}
	if in.A.Kind != vNone {
		parts = append(parts, "A="+in.A.String())
	}
	if in.B.Kind != vNone {
		parts = append(parts, "B="+in.B.String())
	}
	if in.C != 0 {
		parts = append(parts, fmt.Sprintf("C=%d", in.C))
	}
	if in.CS != "" {
		parts = append(parts, "CS="+in.CS)
	}
	if in.Obj2.Kind != rNone {
		parts = append(parts, "obj2="+in.Obj2.String())
	}
	if in.W.Kind != rNone {
		parts = append(parts, "W="+in.W.String())
	}
	if in.F != nil {
		parts = append(parts, "F="+in.F.String())
	}
	if in.Cond.Kind != cNone {
		parts = append(parts, "if="+in.Cond.String())
	}
	if in.Skip != 0 {
		parts = append(parts, fmt.Sprintf("skip=%d", in.Skip))
	}
	if in.Note != "" {
		parts = append(parts, "// "+in.Note)
	}
	return strings.Join(parts, " ")
}

// Zones.
const (
	zNone = iota
	zBattlefield
	zHand
	zGraveyard
	zLibrary
	zExile
	zStack
	zCommand
	zAny
)

var zoneNames = map[string]int{"Battlefield": zBattlefield, "Hand": zHand, "Graveyard": zGraveyard,
	"Library": zLibrary, "Exile": zExile, "Stack": zStack, "Command": zCommand, "Any": zAny, "All": zAny}

var zoneLabel = [...]string{"-", "Battlefield", "Hand", "Graveyard", "Library", "Exile", "Stack", "Command", "Any"}

// lTarget is one target slot.
type lTarget struct {
	F            *lFilter
	Zone         int
	Min, Max     lValue
	Mode         int  // the charm mode it belongs to (-1: every mode)
	Unique       bool // TargetUnique: distinct from every earlier slot's choices
	SameCtrl     bool // all chosen objects share a controller/owner
	DiffCtrl     bool // the chosen objects have different controllers
	SharedCrType bool // TargetsWithSameCreatureType$: the chosen objects share a creature type
	DistinctName bool // TargetsWithDifferentNames$: no two chosen cards share a name
	MaxMV        int  // MaxTotalTargetCMC$ N + 1 (0 = no cap): the chosen cards' total mana value is at most N
	Divided      int  // damage divided as the controller chooses among the chosen

	// DividedV: with Divided == kop.DivValue, the divided total, evaluated
	// where the spell or ability is put on the stack (gorge divisionTotalFor).
	DividedV lValue

	LKIDiv   bool // 0..X targets and X divided counters, X the triggering source's last-known +1/+1 counters (kop.DivLKIP1P1)
	Spell    bool // a spell on the stack
	StackAb  bool // with Spell: an activated or triggered ability on the stack too (TgtStackObj)
	AbOnly   bool // with StackAb: abilities only, no spell (TgtStackAb)
	ActOnly  bool // with AbOnly: activated abilities only (TgfActOnly)
	TrigOnly bool // with StackAb: triggered abilities only beside the spells (TgtStackObjTrig)
	// VTgt: with Spell, TargetValidTargeting$: a spell whose own object targets
	// include a battlefield object matching it (TgtSpellTgting)
	VTgt *lFilter
	// VSpellF: with VTgt or VYou, the spell side's ValidTgts$ filter (nil = any
	// spell); VYou: the targeting clause is "You" (a target of the spell is
	// the chooser), VTgt is nil
	VSpellF *lFilter
	VYou    bool
	// CopyAb: with AbOnly, an ability the chooser controls whose source's types
	// match F: 1 activated or triggered (TgtCopyAb), 2 triggered only
	// (TgtCopyTrig)
	CopyAb uint8
	// SingleTgt: with a spell or StackAb slot, only a spell or ability whose targets are exactly one distinct target (TgfSingleTgt)
	SingleTgt bool
	// PermOrSpell: with Origin$ Battlefield,Stack, a permanent matching F or
	// any spell (TgtPermOrSpell)
	PermOrSpell bool

	// PermOrGraveCard: with Origin$ Battlefield,Graveyard, a permanent or a
	// graveyard card matching F (TgtPermOrGraveCard)
	PermOrGraveCard bool

	// GraveExileCard: with Origin$ Graveyard,Exile, a card in a graveyard or
	// exile matching F, clause i in ClauseZones[i] (TgtGraveExileCard)
	GraveExileCard bool
	ClauseZones    []int

	// TrigChooser: the slot's ask is answered by the queue-time trigger
	// player, not the ability's controller (gorge TargetingPlayer$
	// TriggeredAttackingPlayer on a triggered ability's target,
	// targetChooserFromSpec: the trigger context's attacking player; only
	// the DECISION's Player moves — target legality stays judged from the
	// controller). TgfTrigChooser; the kernel reads it in the cast stage's
	// DkTarget ask (cast.go).
	TrigChooser bool

	AtRes   bool   // asked mid-resolution (gorge's deferred chain-link targets), at its ChooseTargets
	ChgSub  bool   // declared by a ChangeZone chain link (not the root): gorge announces its targets on cast (castSubChangeZoneAnnounceable)
	Decl    bool   // a spell's copy target declaration (its root node or a charm mode's head)
	SubDecl bool   // a SubAbility$ link's copy target declaration (gorge copyChainDeclarations)
	SubCond string // with SubDecl: the link's lowercased Condition$ ("kicked", "optionalcost" excuse the announcement)
	// ParentPlus1 is the 1-based slot of the player target whose graveyard
	// this card slot is restricted to (TargetsWithDefinedController$
	// ParentTarget); 0 = unrestricted.
	ParentPlus1 int
}

// lCost is an ability's cost.
type lCost struct {
	Mana           manaCost
	Tap            bool
	Untap          bool // {Q}: untap this permanent (an activated battlefield ability's cost)
	SacSelf        bool
	Sac            *lFilter // sacrifice SacN permanents matching
	SacN           int
	SacX           bool     // Sac<X/Spec>: sacrifice the announced X permanents matching Sac (kop.CostSac with N 0; SacN stays 0)
	XMin           int      // XMin<N> / XMin$ N: the announced X's floor (kop.Cost.XMin)
	DiscardN       int      // discard N cards (any)
	DiscardSpec    string   // Discard<N/Spec>: "" (Card, any card), "Random" or "Hand"
	DiscardHand    bool     // Discard<N/Hand> as an activated ability's cost: the whole hand (kop.Cost.DiscardHand)
	DiscardSelf    bool     // discard this card (a hand ability: cycling)
	DiscardF       *lFilter // Discard<N/Spec>: the discard candidates' filter (CostDiscardFiltered)
	Loyalty        int      // +N / -N loyalty
	LoyaltyX       bool     // [-X]: SubCounter<X/LOYALTY>, X announced up to the loyalty
	Exert          bool     // Exert<1/NICKNAME>: exert the source as the cost (CR 702.100a)
	SubCtr         int      // remove N counters of kind SubCtrKind from the source
	SubCtrX        bool     // SubCounter<X/Kind>: X announced up to the counters of SubCtrXKnd on the source
	SubCtrXKnd     string
	SubAmongN      int // RemoveAnyCounter<N/Kind/Filter>: remove N counters of kind SubAmongKnd from among SubAmongF you control (kop.CostSubAmong)
	SubAmongKnd    string
	SubAmongF      *lFilter
	SubCtrKnd      string
	AddCtr         int // put N counters of kind AddCtrKnd on the source (AddCounter<N/KIND>; Wall of Roots' M0M1)
	AddCtrKnd      string
	AddCtrRef      lRef // the UnlessCost$ AddCounter recipient: rSelf for CARDNAME/NICKNAME, rTarget for Card.targetedBy
	PayLife        int
	LifeX          bool    // PayLife<X>: pay the announced X life, SVar X Count$xPaid (kop.CostPayLife with N 0)
	LifeBy         *lValue // PayLife<X> of a printed trigger's own cost, X read as the cost is paid (SVar X TriggeredCard$CardPower; payInstr)
	TapN           int     // tap N untapped creatures you control matching TapF
	TapF           *lFilter
	TapHost        bool // tap the granting host (tapXType<1/OriginalHost>)
	SacHost        bool // sacrifice the granting host (Sac<1/OriginalHost>), or exile it when HostExile (Exile<1/OriginalHost>)
	HostExile      bool // SacHost's part exiles the granting host (kop.CostHostExile; an activated ability's cost only)
	SacExile       bool // Sac/SacN's part is Exile<N/Spec> from the battlefield (kop.Cost.ExileBf; an activated ability's or a spell's additional cost only)
	TapPowN        int  // tap any number of untapped creatures you control matching TapPowF with total power >= TapPowN (Crew, Saddle; lower_tappower.go)
	TapPowF        *lFilter
	TapPowCrew     bool     // the tap-power cost is Crew (graveexile: candidates' TapPowerCrew applies)
	TapPowSaddle   bool     // the tap-power cost is Saddle (graveexile: candidates' TapPowerSaddle applies)
	Forage         *lFilter // forage: exile three cards from your graveyard or sacrifice a Food matching (CR 701.61a)
	ExileG         *lFilter // ExileFromGrave<N/Spec>: exile ExileGN cards matching from your graveyard (CARDNAME: Self)
	ExileGN        int
	ExileGSelf     bool     // ExileFromGrave<1/CARDNAME> first, then ExileFromGrave<N/Other spec> (kop.Cost.ExileGSelf; a graveyard activated ability's cost)
	exileGCard     bool     // ExileG is the CARDNAME singleton part (a second ExileFromGrave part may follow)
	ExileHandN     int      // ExileFromHand<N/Card>: exile N cards from your hand (a trigger's own cost only; cost_exilefromhand.go)
	ExileHand      bool     // ExileFromHand<1/Card>: the ExileG cards come from the payer's hand (kop.Cost.ExileHand; a granted exile ability's cost only)
	ExileGBf       bool     // ExileCtrlOrGrave<N/Spec>: the battlefield (the payer's permanents) as well (kop.CostExileCtrlOrGrave, Craft)
	ExileStackSelf bool     // ExileFromStack<1/Card.Self>: the resolving spell exiles itself from the stack (an unless cost beside ExileG)
	Energy         int      // PayEnergy<N>: pay N energy (an activated ability's cost only)
	EnergyBy       *lValue  // PayEnergy<X>/<SVar>: the amount read as the cost is offered (a resolution unless cost)
	Waterbend      int      // Waterbend<N>: N generic payable by tapping artifacts/creatures (an activated ability's cost only)
	WaterbendX     bool     // Waterbend<X>: the cost's one {X} payable by tapping artifacts/creatures (kop.CostWaterbend with N 0; an activated ability's cost only)
	BlightN        int      // Blight<N>: put N -1/-1 counters on a creature you control (CR 701.68; shares Cost.N/Filter)
	ReturnSelf     bool     // Return<1/CARDNAME>: return this permanent to its owner's hand (an activated ability's cost only)
	ExileSelf      bool     // Exile<1/CARDNAME>: exile this permanent (an activated ability's cost only)
	Return         *lFilter // Return<N/Spec>: return ReturnN permanents you control matching (a resolution cost only)
	ReturnN        int
	DrawN          int      // Draw<N/You>: draw N cards (a trigger's own cost only)
	EvidenceN      int      // CollectEvidence<N>: exile graveyard cards with total mana value >= N (CR 701.59a)
	EvidenceX      bool     // CollectEvidence<X>: the announced X (a trigger's own cost only)
	TapXVar        bool     // tapXType<X/Spec>: tap any number, X the count tapped (a trigger's own cost only; shares TapF)
	ExileTrigNew   bool     // ExileAnyGrave<1/Card.TriggeredNewCard>: exile the triggered card from any graveyard (a dies trigger's own cost)
	PutLibSelf     bool     // PutCardToLibFromGrave<1/-1/CARDNAME>: the source card goes from its graveyard to the bottom of its owner's library (a graveyard activated ability's cost)
	BeholdN        int      // Behold<N/Type>: reveal or choose N permanents or hand cards matching BeholdF (CR 701.4a; a spell's additional, optional, may-flash or flashback cost part)
	BeholdF        *lFilter // the Behold<N/Type> candidates' filter
}

// pureMana reports whether the cost is only its mana part. Power-up's
// reduction (CR 702.193b) applies to the mana cost, so a PowerUp$ ability
// with any other cost is outside the modelled shape.
func (c lCost) pureMana() bool {
	return !c.Tap && !c.Untap && !c.SacSelf && c.Sac == nil && c.SacN == 0 &&
		c.DiscardN == 0 && c.DiscardSpec == "" && !c.DiscardHand && !c.DiscardSelf && c.DiscardF == nil && c.Loyalty == 0 && !c.LoyaltyX &&
		c.SubCtr == 0 && !c.SubCtrX && c.SubAmongN == 0 && c.AddCtr == 0 && c.PayLife == 0 && !c.LifeX && c.LifeBy == nil && !c.SacX && c.XMin == 0 && c.TapN == 0 && !c.TapHost && !c.SacHost && !c.HostExile && !c.SacExile && c.TapPowN == 0 && !c.Exert &&
		c.Forage == nil && c.ExileG == nil && !c.ExileGSelf && c.ExileHandN == 0 && c.Energy == 0 && c.EnergyBy == nil && !c.ReturnSelf &&
		!c.ExileSelf && !c.ExileStackSelf && c.Return == nil && c.DrawN == 0 && c.BlightN == 0 && c.EvidenceN == 0 && !c.EvidenceX && !c.TapXVar && !c.ExileTrigNew &&
		!c.PutLibSelf && c.BeholdN == 0 && c.BeholdF == nil
}

// manaCost is a printed mana cost.
type manaCost struct {
	Generic int
	Pips    [5]int // W U B R G
	Hybrid  []string
	Twobrid []byte // monocoloured hybrid pips ({2/W}) as colour letters, in printed order (lower_twobrid.go)
	// Phyrexian pips ({W/P}) as colour letters, in printed order
	// (lower_twobrid.go)
	Phyrexian []byte
	X         bool
	XX        bool // a second {X} symbol (X set too)
	XXX       bool // a third {X} symbol (X and XX set too)
	None      bool // "no cost": cannot be cast
	C         int  // colorless-specific pips
	// XColor is Forge's XColor$ as one WUBRG letter: every announced {X} is
	// that colour's pip, not generic (CR 107.3, 601.2b; Consume Spirit).
	// Zero when absent. Only a one-colour value is lowered; the multi-colour
	// Soul Burn shape is refused by name.
	XColor byte
}

func (m manaCost) String() string {
	if m.None {
		return "none"
	}
	s := fmt.Sprintf("{%d", m.Generic)
	for i, c := range "WUBRG" {
		s += strings.Repeat(string(c), m.Pips[i])
	}
	if m.C > 0 {
		s += strings.Repeat("C", m.C)
	}
	for _, h := range m.Hybrid {
		s += "(" + h + ")"
	}
	for _, c := range m.Twobrid {
		s += "(2" + string(c) + ")"
	}
	for _, c := range m.Phyrexian {
		s += "(" + string(c) + "P)"
	}
	if m.X {
		s += "X"
	}
	if m.XX {
		s += "X"
	}
	if m.XXX {
		s += "X"
	}
	return s + "}"
}

// Ability kinds.
const (
	akSpell = iota
	akActivated
	akMana
	akTriggered
	akLatent // a program reached only through an op (delayed/reflexive trigger, granted ability, mode body)
)

var abilityKindNames = [...]string{"Spell", "Activated", "Mana", "Triggered", "Latent"}

type lTrigger struct {
	Event string
	F     *lFilter // the object filter (ValidCard / ValidSource / ...)
	F2    *lFilter // the second filter (ValidTarget of damage / attackers)
	// F2Zones is the zone each F2 clause is matched in (TargetsValid$): a
	// battlefield alternative lowers as ZoneNone, a zone-free one as
	// kop.ZoneAny, the marker kernel castTargetsAll reads for a target off
	// the battlefield.
	F2Zones       []kop.Zone
	Whose         uint8  // whoAny / whoYou / whoOpp (ValidPlayer / ValidActivatingPlayer / AttackingPlayer)
	Zone          int    // the zone the source must be in (TriggerZones)
	Step          string // Phase trigger step
	Origin        int
	Dest          int
	From          int  // Leaves: the zone left (graveyard leaves)
	Combat        int8 // 1 combat damage only, -1 noncombat only
	Optional      bool
	Cond          lCond // intervening-if
	NthDraw       int   // Drawn: Number$
	First         bool  // FirstTime$: first time each turn
	OncePer       bool  // ActivationLimit$ 1
	OnlyYourTurn  bool
	OnlyOppTurn   bool
	MinAttackers  int
	AtkCmp        uint8 // ValidAttackersAmount operator as a kop.DmgCmp (0 = GE, the plain literal; kernel trigCmpOK)
	CrossAt       int   // CounterAdded: CounterAmount$ EQn, the counter total the put must reach (a crossing, CR 122.6); 0 = none
	PerCounter    bool  // CounterAdded, a named CounterType$, no CounterAmount$ or ActivationLimit$: one trigger per counter of the batch, each carrying amount 1 (CR 603.2c; gorge counterAddedMatches; kop.Tf4PerCounter)
	CastSnap      bool  // SpellCast: the amount reads the instants and sorceries cast this turn, snapshotted (TriggerObjectsCurrentCastSpells)
	CastSpent     bool  // SpellCast: the amount is the triggering spell's pool mana spent, snapshotted as the trigger fires (opus; gorge TriggeredCard$CastTotalManaSpent)
	Increment     bool  // SpellCast: Increment$ True -- the source is a creature and the cast's spend exceeds its P/T, read as the trigger fires and rechecked on resolution (gorge kwIncrement's marker, trigmatch.IncrementHolds; kernel Tf2Increment)
	SpentGate     int16 // SpellCast: ValidSA$/ValidSAonCard$ Spell.ManaSpent <op><n|X>, a kop.DmgGate over the cast's pool mana spent (X = 0xff, the printed mana value); gorge spellCastMatches, kernel spentGateOK
	NthSpell      int   // SpellCast: the caster's spell count this turn against ActivatorThisTurnCast$ <op>N
	NthCmp        uint8 // the NthSpell operator as a kop.DmgCmp (0 = EQ, the plain literal; kernel trigCmpOK)
	NthFiltered   bool  // NthSpell counts the spells F matches: ValidCard$ turns some away (gorge activatorCastCount)
	Amount        int   // ManaExpend: expend N
	BlockedBySelf bool  // AttackerBlocked: ValidBlocker$ Card.Self
	TargetsAll    bool  // SpellCast: every target of the spell matches F2 (TargetsValid$)
	Alone         bool  // Attacks: only when exactly one creature attacks (Alone$ True)
	Attacked      uint8 // Attacks: Attacked$ as atkYou|atkYourPW|atkOpp bits (0 = any defender)
	Batch         bool  // a "one or more" mode (ChangesZoneAll, DamageDoneOnce): once per simultaneous batch
	NotToGrave    bool  // Leaves: only to a zone other than a graveyard (ChangesZoneAll Destination$ without Graveyard)
	AlsoGrave     bool  // Leaves: the line also fires for a card leaving a graveyard to a zone in N (ChangesZoneAll Origin$ Battlefield,Graveyard; kop.Tf2LeavesGrave)
	Training      bool  // Attacks: Training$ True, the source attacks with a greater-power attacker (kop.TrigNTraining)
	Valiant       bool  // BecomesTarget: Valiant$ True with ValidTarget$ Card.Self, ValidSource$ SpellAbility.YouCtrl
	TeamworkTap   bool  // Taps: Teamwork$ True, only a teamwork-cost tap (kop.TrigNTeamworkTap)
	FirstAttack   bool  // FirstAttack$ True: the attacker has not been declared earlier this turn (kop.Tf3FirstAttack)
	GameOnce      bool  // GameActivationLimit$ 1: once per source object (kop.Tf3GameOnce)
	SingleTarget  bool  // SpellCast: the cast spell's root target list has exactly one target (kop.Tf4SingleTarget)
	CounterCell   bool  // CounterAddedOnce from Mode$ CounterPlayerAddedAll: one fire per put action per recipient (kop.Tf4CounterCell)
	Outstanding   bool  // Mode$ Always, the counter-threshold state trigger: one instance at a time, the leave re-checks (kop.Tf4StateCtr)
	OptTrigPlayer bool  // Optional: the trigger's player (the Phase trigger's active player) answers, not the controller (kop.Tf4OptTrigPlayer)
	NamedCast     bool  // SpellCast lowered from Mode$ SpellAbilityCast over the source's chosen card name: also fires on an activation of a source with that name (kop.Tf4NamedCast; lower_choosename.go)
	TapPlan       uint8 // kop.TapPlan; Taps, non-Self ValidCard$: the payment-planner class of the watcher body (lower_tapsclass.go)
	TapPainN      uint8 // TapPlanPain: the vetted literal damage amount
	CauseTarget   bool  // DamageDone: ValidCause$ Spell.numTargets EQ1 + TargetRelativeToCause$ Creature.targetedBy (kop.Tf4SingleCauseTarget)
	// Watch: BecomesTarget watching ValidTarget$ objects (F) targeted by an
	// opponent's spell or ability (ValidSource$ SpellAbility.OppCtrl);
	// SpellToo: a creature spell its controller controls counts too.
	Watch, SpellToo bool
	// ByCreature: AttackerBlocked lowered from Mode$ AttackerBlockedByCreature,
	// once per (attacker, blocker) pair; F reads the attacker, F2 the
	// blocker, exactly one of them Card.Self; the trigger object is the
	// pair's other member. AnchorBlocker: the source is the blocker.
	ByCreature, AnchorBlocker bool
	Cycled                    bool  // Discarded lowered from Mode$ Cycled ValidCard$ Card.Self: fires from the graveyard when the card is cycled
	Unblocked                 bool  // AttackerBlocked lowered from Mode$ AttackerUnblocked: once per unblocked attacker matching F
	OrCopy                    bool  // SpellCast lowered from Mode$ SpellCastOrCopy: also a spell copy
	ConvergeBound             bool  // the body reads Count$Converge: the entering spell's colours spent, bound as it triggers (self ETB only; kernel Tf3ConvergeBound)
	CastConverge              bool  // SpellCast: the amount is the triggering spell's colours spent as the trigger fires (TriggeredCard$Converge; kernel Tf3CastConverge)
	SpentBound                bool  // the body reads Count$CastTotalManaSpent: the entering spell's mana spent, bound as it triggers (self ETB only; kernel Tf2SpentBound)
	XBound                    bool  // the body reads Count$xPaid without an {X} cost of its own: the trigger object's spell's X, bound as it triggers (CR 107.3m)
	DefPlayer                 int   // AttackersDeclared per defender, the player only: 1 any player, 2 an opponent of the controller, 3 the controller, 4 enchanted by this Aura (AttackedTarget$ Player.EnchantedBy; kernel Tf4DefEnch)
	PerRef                    bool  // DamageDone lowered from Mode$ DamageDoneOnce: once per damaged referent per batch (kernel Tf2PerReferent)
	SrcInPlay                 bool  // DamageDone ValidSource$ carries inZoneBattlefield: a source that is not a permanent never matches (kernel Tf2SrcInPlay)
	SelfPlayerRef             bool  // PerRef from a plain self-source DamageDone to any player that reads the damaged player (lower_damagedoneonce.go selfPlayerRefShape)
	DmgGate                   int16 // PerRef: DamageAmount$ gate (kop.DmgGate), 0 = none
	ResolvedLimit             bool  // ResolvedLimit$ 1: the source's limited lines resolve at most once each turn
	Convoked                  bool  // ETB TfSelf: the trigger amount is Convoked$Amount, bound as it triggers (kernel Tf2Convoked)
	CastHand                  bool  // ValidCard$ ...+wasCastFromYourHandByYou: ETB on the entering spell's cast, SpellCast on the spell (kernel Tf2CastFromHand)
	ManaFromSelf              bool  // SpellCast ValidSA$ Spell.ManaFromCard.StrictlySelf: the spell was paid with mana this source produced (kernel Tf2ManaFromSelf)
	MayPlaySrc                bool  // SpellCast ValidSA$ Spell.MayPlaySource: the cast went through a may-play permission (gorge FlagMayPlay; kernel Tf3MayPlaySrc)
	Static                    bool  // SpellCast Static$ True: resolved off the stack by the trigger drain, before the ordering ask (gorge static_trigger.go; kernel Tf3Static)
	ManaFromTreasure          bool  // SpellCast ValidSA$ Spell.ManaFromTreasure: the cast spent Treasure mana (kernel Tf3ManaFromTreasure)
	OrTargets                 bool  // SpellCast ValidSA$ "Spell.X,Spell.IsTargeting Valid Y": F matches the spell OR F2 matches some target (kernel Tf3OrTargets)
	N                         int16 // LandPlayed: the Origin$ zone mask, 1<<kop.Zone (0 = any)
	AbClass                   int16 // AbilityCast: the ValidSA$ class (kop.TrigAbCast*)
	Evolve                    bool  // ETB: Condition$ Evolve, the entering creature's power or toughness greater than the source's, read as it fires

	// Drawn: FirstCardInDrawStep$ gates the card's position in the drawer's
	// own draw step (gorge trigmatch.firstCardInDrawStep, CR 504.1).
	FirstInDrawStep    bool // FirstCardInDrawStep$ True (kop.Tf5FirstCardInDrawStep)
	NotFirstInDrawStep bool // FirstCardInDrawStep$ False (kop.Tf5NotFirstCardInDrawStep)
}

// lTrigger.Attacked bits.
const (
	atkYou uint8 = 1 << iota
	atkYourPW
	atkOpp
)

type lAbility struct {
	Kind            int
	Latent          bool   // not active on the card itself: reached through an op or a grant
	TrigRem         string // a reflexive trigger's TriggerRemembered$ rest (lower_trigremembered.go)
	RemTgt          bool   // a Pump RememberObjects$ ThisTargetedCard (lower_remembered.go)
	ReflexObj       bool   // a reflexive latent whose ImmediateTrigger captured a target (lower_remembered.go)
	DelayedObjOnly  bool   // a delayed latent whose RememberObjects$ set holds objects only (lower_delayedobjects.go)
	DelayedPlayer   bool   // a delayed latent registered with a remembered PLAYER (the countered spell's controller): Defined$ DelayTriggerRemembered reads the trigger player (lower_delayremplayer.go)
	DelayedSelf     bool   // the delayed program remembered the source's own card (Transformed$ return; lower_delayedobjects.go)
	Origin          int    // gorge's Face.Abilities index, or 0x80|Face.Triggers index; 0 is ambiguous with A[0], so Origin is valid only when OriginOK
	OriginOK        bool
	Index           int // position in the card's ability list (latent abilities included)
	Name            string
	Cost            lCost
	Optional        bool // the cost is optional ("you may pay ... if you do")
	Sorcery         bool
	Zone            int // activation zone (battlefield unless graveyard/hand)
	Loyalty         bool
	Exhaust         bool    // Exhaust$ True (CR 702.177a): the AbilityCast class test (kop.Ability.Exhaust)
	CantCopy        bool    // CantCopy$ True: "this ability can't be copied" (kop.Ability.CantCopy)
	Limit           int     // activations per game (0 = unlimited)
	Once            int     // the ordinal among the card's activate-only-once abilities (1..3; 0 = none)
	EvidenceTargets bool    // CollectEvidence<X> with SVar:X AllTargeted$CardManaCost (a spell's mandatory additional cost; kop.EvidenceTargetsMV)
	PowerUp         bool    // PowerUp$ True (CR 702.193): once per object stint, cost reduced on its entry turn
	TurnOnce        bool    // ActivationLimit$ 1: once each turn
	Reduce          *lValue // ReduceCost$: the generic reduction of an activated ability's own cost (lower_ownreduce.go)
	ActCond         lCond   // CheckSVar$/SVarCompare$: activate only if (CR 602.5b)
	Targets         []lTarget
	Modes           int // charm: number of modes to choose from
	// charm: CharmNum$ (the most picks) and MinCharmNum$ (default: the most)
	ModesPick, ModesMin int
	RepMax, RepCost     [4]uint8 // charm CanRepeatModes$ (lower_charmrepeat.go): the most each mode is chosen, its pawprint price
	RepBudget           uint8    // charm Pawprint$
	ModeRestrict        uint8    // charm ChoiceRestriction$: 0 none, 1 ThisTurn, 2 ThisGame (lower_charmrestrict.go)
	Trigger             *lTrigger
	Prog                []lInstr
	Mana                *lMana
	Unless              *lUnless // the resolving ability is countered/skipped unless a player pays
	// ResolveCount: the ability reads its own resolutions this turn
	// (Count$ResolvedThisTurn)
	ResolveCount bool
	Gate         *lGate // a Class level gate
	// Cycling: a cycling ability (Keyword$ Cycling / TypeCycling): its
	// discard-this cost is a cycle (gorge cyclingKeyword, Mode$ Cycled)
	Cycling bool
	// Ninjutsu: the K:Ninjutsu expansion (CR 702.49; lower_ninjutsu.go)
	Ninjutsu bool
	// PutSelf: "{cost}: Put this card onto the battlefield from your hand"
	// (a mana-only hand ability that keeps its source in the hand; the
	// ChangeZone of lower_ninjutsu.go putSelfAbility)
	PutSelf bool
	// Station: the Station ability (K:Station, CR 702.184a;
	// lower_station.go)
	Station bool
}

// lGate is a threshold gate: the ability exists only while the source
// permanent's count of the gate's counter kind is >= Level (== Level when
// Eq). Ctr "" is a Class's level; "LORE" is a Saga chapter
// (CR 714.2b), whose trigger fires only when the count crosses Level.
type lGate struct {
	Ctr   string
	Level int
	Eq    bool
	Speed bool // a max-speed gate (kop.CtrMaxSpeed; lower_speed.go)
}

// lMana is a mana ability's production.
type lMana struct {
	Colors     string // the colours one of which is produced ("C", "G", "BR", "WUBRG", "Chosen")
	Choice     bool   // Colors is a choice among them (Produced$ Combo ...), not simultaneous production
	Each       bool   // Amount of each of Colors at once (Produced$ "W U": simultaneous production; kop.Ability.ManaEach)
	Amount     lValue
	Restrict   *lFilter // spend only on spells matching
	RestrictAb *lFilter // ... or on activated abilities of objects matching
	NonSpell   bool     // ... or on any payment that is not a spell (CantCastNonArtifactSpells)
	Special    string
	SpecialF   *lFilter
	// Produce: the reflected read is what the matching permanents' mana
	// abilities could PRODUCE, not their colours (ReflectProperty$ Produce,
	// every Valid$ alternative OppCtrl: Exotic Orchard, Fellwar Stone).
	Produce    bool
	Deferred   bool     // a TriggersWhenSpent$ source gorge's planner defers (kop.Ability.ManaDeferred; lower_deferredmana.go)
	Split      *lSplit  // a Condition-split production's arm (kop.Ability.SplitGate; lower_condsplit.go)
	HasteValid *lFilter // AddsKeywords$ Haste: a spell matching it, paid with this mana, gains haste until end of turn (kop.Ability.ManaHasteValid)
	// Rider is the rider the ability runs with its mana (kop.Ability.ManaRider;
	// lower_manarider.go): none, deplete or each-opponent life.
	Rider    int
	RiderCtr string // the source counter kind a deplete rider reads
	RiderN   int    // the life count
}

type lUnless struct {
	Cost  lCost
	Payer lRef
}

// Static kinds (lStatic.Kind).
type lStatic struct {
	Kind   string // Continuous, ReduceCost, CantBlock, CantBlockBy, AlternativeCost, CastWithFlash, MustAttack, Override, ...
	F      *lFilter
	F2     *lFilter
	Cond   lCond
	Power  lValue
	Tough  lValue
	Kw     []string
	AddSub []string
	// AddChosen: AddType$ ChosenType, the static's host's chosen creature
	// type (kop.Static.AddChosen)
	AddChosen bool
	// OutlivesLoss: the pump beside an override static with a part in layers
	// 1 to 5 (kop.Static.OutlivesLoss)
	OutlivesLoss bool
	Amount       lValue
	Cost         lCost
	Zone         int
	Override     *lOverride
	// ProtFrom: a Continuous grant's protection qualities (kop.Static.ProtFrom)
	ProtFrom *lFilter
	// Landwalk: a Continuous grant's landwalk land filter (kop.Static.Landwalk)
	Landwalk *lFilter
	Grant    []int    // latent abilities the affected objects gain
	Flags    []string // kind-specific flags
	Note     string
	Ward     int // a Continuous grant's "Ward:N" (Kw then holds "Ward")
	// WardLife: a Continuous grant's "Ward:PayLife<N>" (kop.Static.WardLife)
	WardLife int
	// AbilityRed: a ReduceCost static of activated abilities (Type$ Ability)
	// rather than spells: 1 any, 2 Exhaust abilities only, 3 a Room unlock,
	// 4 Power-up abilities only, 5 Equip abilities only (F2: their target;
	// kop.StatReduceAbility)
	AbilityRed uint8
	// MinMana: that static's MinMana$ floor, 1..15 (kop.ReduceAbMinShift)
	MinMana uint8
	// PwVsSrc: CantBlockBy ValidBlocker$ powerLTX / powerGTX with X = Count$CardPower
	// (kop.Static.PwVsSrc)
	PwVsSrc int8
	// TapCol, TapAmt: a TapExtra static's mana index and amount (kop.StatTapExtra)
	TapCol, TapAmt uint8
	// RaiseAb: a MayPlayFrom static's RaiseCost$ (flag "MayRaise"): 1 + the
	// latent ability whose Cost is the raise (lower_mayraise.go)
	RaiseAb int
	// RedCol: a ReduceCost's Color$ counts, two bits per colour W U B R G C
	// (flag "ReduceColor"; lower_reducespell.go)
	RedCol uint16
	// RaiseCol: a per-target RaiseCost's Cost$ pips per charge, packed like
	// RedCol (flag "RaisePerTarget"; lower_raisecost.go)
	RaiseCol uint16
	// Ctr: a CantPutCounter's CounterType$ kind, -1 for every kind
	// (kop.Static.A; Blossombind, Blightbeetle)
	Ctr int16
	// OvGate: the bit of lOverride.GrantStatics that switches this static on
	// (kop.Static.OvGate); 0 = a printed static
	OvGate uint8
}

// lOverride is the override record a hard static/effect applies.
type lOverride struct {
	SetP, SetT       int
	PVal, TVal       lValue // non-None: Power$/Toughness$ values (ValPT)
	HasSetPT         bool
	LivePT           bool // PVal/TVal are read live, every check (a granted static's SetPower$/SetToughness$, for the affected permanent; an override static's, from its source)
	SetTypes         uint16
	HasSetTypes      bool
	AddTypes         uint16
	SetSubtypes      []string
	HasSetSubtypes   bool
	AddSubtypes      []string
	Kept             effects.SubtypesKept // what RemoveCardTypes$ leaves (keepSubtypes)
	AllCreatureTypes bool
	// RemoveCreatureSub: RemoveCreatureTypes$ without RemoveCardTypes$
	RemoveCreatureSub bool
	SetColors         string
	HasSetColors      bool
	LoseAbilities     bool
	AddKw             []string
	GrantAbility      []int    // latent ability indexes granted
	Name              string   // SetName: the layer-3 name (a name card, lowerPool)
	KeepP, KeepT      bool     // a set of base toughness / power only (Maha)
	RemoveKw          []string // RemoveKeyword$: keywords removed (layer 6)
	Dedup             bool     // share a byte-identical pool record across cards (Earthbend)
	WhileHost         bool     // Duration$ UntilHostLeavesPlay / AsLongAsInPlay
	WhileTapped       bool     // Duration$ UntilUntaps: WhileHost, and the host's untap ends it too
	CantAttack        bool     // HiddenKeywords$ CARDNAME can't attack or block. (with AddKw CantBlock)
	MustBeBlocked     bool     // HiddenKeywords$ CARDNAME must be blocked if able.: emitOverride follows the record with ExtMustBeBlocked on the same ref (not a record field)
	NoCost            bool     // a copy exception's "with no mana cost" (RemoveCost$)
	AddColors         string   // a copy exception's AddColors$: colour letters added after any set
	RemoveLegendary   bool     // a copy exception's NonLegendary$ (CR 707.9b)
	PTDelta           bool     // PVal/TVal are a +N/+M pump (layer 7c), not a base P/T: PumpAll Duration$ UntilYourNextTurn NumAtt$/NumDef$ (kop.Override.PTDelta)
	GrantStatics      uint8    // an Animate staticAbilities$ grant of card statics gated by lStatic.OvGate (kop.Override.GrantStatics)
}

// Replacement-lite kinds.
type lRepl struct {
	Kind string // EntersTapped, EntersWithCounters, CantBeCountered, DoesntUntap, MayNotUntap, EntersAsChoice, CantLoseGame, OthersEnterTapped, OthersEnterWithCounters
	F    *lFilter
	Cond lCond
	N    lValue
	S    string
	Ctr  string // AddCountersOn and DoubleCountersOn: the counter kind ("": every kind, lower_counterrepl.go)
	// Putter: AddCountersOn of every kind only counts counters its controller
	// puts on the permanent (ValidSource$ You; Doc Samson)
	Putter bool
	Prog   []lInstr
	Bits   int // DamagePlus: the recipients (kop.ReplDmg* bits); DrawMore: kop.ReplDraw* flags
	// AnyZone: ExileOnLeave applies to a move to a graveyard from every zone
	// (Repl.C = 1), not only a permanent dying.
	AnyZone bool
}

// lCard is a lowered card.
type lCard struct {
	Name           string
	Token          bool
	Cost           manaCost
	CostEmpty      bool // the printed ManaCost text is empty (not "no cost")
	Types          uint16
	Subtypes       []string
	Colors         string
	P, T           int
	PTStar         bool
	NoPT           bool     // the face prints no power or toughness (CardDef.NoPT)
	TwoSeat        bool     // the lowering holds only for 2 players (CardDef.TwoSeat, set by lowerCtx.twoSeat)
	TwoSeatWhy     []string // the reasons that set TwoSeat, in first-seen order (TestTwoSeatReasons)
	Loyalty        int
	Defense        int // a Battle's printed Defense (lower_battle.go)
	Kw             []string
	Flash          bool
	Kicker         *manaCost
	Multikicker    *manaCost   // K:Multikicker:<cost> (CR 702.43): the per-payment additional cost, paid any number of times
	KickerCost     *lCost      // K:Kicker with one Sac<1/..> or Return<1/..> part: Kicker is then zero mana (lower_kickercost.go)
	Spree          bool        // K:Spree (CR 702.172) or K:Tiered (CR 702.183): ModeCost holds each mode's additional mana cost
	ModeCost       [4]manaCost // Spree/Tiered: mode i's ModeCost$
	MorphCost      *manaCost   // K:Morph / K:Megamorph / K:Disguise turn-face-up cost
	MorphFamily    uint8       // kop.Morph*: which face-down keyword the cost belongs to
	MorphReduce    lValue      // the turn-up cost's printed generic reduction (lower_morphreduce.go)
	Flashback      *manaCost
	RemPersist     bool          // the enters/leaves remembered pair, or a Saga's chapter pair (lower_rempersist.go)
	RemSeed        uint32        // RemPersist: the abilities that start with the kept set (CardDef.RemSeed)
	RemKeep        uint32        // RemPersist: the abilities whose finished set is kept (CardDef.RemKeep)
	ExwWord        bool          // the card's text names ExiledWithSource: gorge records the reverse ExiledWith scalar for what it exiles (CardDef.ExiledWithRev)
	ExiledWith     bool          // a resolution reads the exiled-with source link: the card records what it exiles (CardDef.ExiledWith)
	CombatHitRec   bool          // a filter reads wasDealtCombatDamageThisTurnBySource against the card itself: its objects' combat hits on players are recorded (CardDef.CombatHitRec)
	SpendRead      bool          // an SVar body reads Count$CastTotalManaSpent: gorge's planner declines the cast
	Identity       kop.ColorMask // the card's colour identity (CardDef.Identity; a commander-identity mana ability's read)
	ProduceCol     kop.ColorMask // the face's printed mana abilities' produced colours ({C} dropped; CardDef.ProduceCol; a Produce-mode reflected ability's read)
	FbCost         *lCost        // the flashback cost's non-mana part (Sac<N/Spec> or tapXType<N/Spec>)
	Madness        *manaCost     // K:Madness:<cost> (CR 702.35a): the exile-cast cost
	Miracle        *manaCost     // K:Miracle:<cost> (CR 702.94a): the hand-cast cost of the revealed first draw
	Warp           *manaCost     // K:Warp:<cost> (CR 702.185a): the hand's alternate cost and the end-step exile marker
	NoAbilities    bool          // the printed face has no ability (effects.FaceHasNoAbilities): CardDef.NoAbilities, the NoAbilities filter word's card read
	HasManaAbility bool          // the printed face has a mana ability (effects.FaceHasManaAbility): CardDef.HasManaAbility, PredHasManaAbility's card read
	Impending      *manaCost     // K:Impending:<N>:<cost> (CR 702.176a): the hand's alternate cost; ImpendingN time counters
	ImpendingN     int
	Plot           *manaCost // K:Plot:<cost> (CR 702.170a): the sorcery-speed special action's cost
	Suspend        *manaCost // K:Suspend:<N>:<cost> (CR 702.62a): the hand's special action cost; SuspendN time counters
	SuspendN       int
	Foretell       *manaCost // K:Foretell:<cost> (CR 702.143a): the later cast's cost (the {2} special action is fixed; CardDef.ForetellCost)
	Paradigm       bool      // K:Paradigm (SOS): resolved -> exile, a free cast from exile each first main phase
	Sneak          *manaCost // K:Sneak:<cost> (CR 702.190a): the declare-blockers alternative cost (plus the return)
	WebSling       *manaCost // K:Web-slinging:<cost> (CR 702.188a): the hand's alternative cost (plus returning a tapped creature)
	MayFlash       *manaCost // K:MayFlashCost:<cost> (CR 702.8): the extra mana that buys an instant-timing cast from the hand
	MayFlashBehold *lFilter  // K:MayFlashCost:Behold<1/Type>: the extra is beholding one of these, no mana (MayFlash is empty)
	FbBeholdN      int       // K:Flashback:<mana> Behold<N/Type>: the flashback cast also beholds N of FbBeholdF (lower_fbcost.go, CardDef.FbBehold)
	FbBeholdF      *lFilter
	Dash           *manaCost // K:Dash:<cost> (CR 702.109a): the hand's alternative cost; the permanent gains haste and returns to its owner's hand at the next end step
	Evoke          *manaCost // K:Evoke:<cost> (CR 702.74a): the hand's alternative cost; the permanent is sacrificed as it enters
	Overload       *manaCost // K:Overload:<cost> (CR 702.96a): the hand's alternative cost; the spell reads "target" as "each"
	// OverloadProg is the overloaded cast's program: the printed spell with
	// each targeted instruction rewritten to its "each" sweep (CR 702.96a).
	OverloadProg   []lInstr
	Bestow         *manaCost // K:Bestow:<cost> (CR 702.103a): the hand's alternative cost of the Aura cast
	BestowT        *lTarget  // the bestowed Aura's enchant creature target (gorge bestowedAttachSA)
	Convoke        bool      // K:Convoke (CR 702.51a): the cast may tap creatures for its mana cost
	Improvise      bool      // K:Improvise (CR 702.126a): the cast may tap artifacts for the generic part of its mana cost
	Delve          bool      // K:Delve (CR 702.66a): the cast exiles graveyard cards toward its generic cost (dirac/kw_delve.go)
	Rebound        *lAbility // K:Rebound (CR 702.88a): the latent free-cast-from-exile body of the upkeep recast (dirac/kw_rebound.go)
	Mayhem         *manaCost // K:Mayhem:<cost> (CR 702.187): a graveyard cast for this cost if the card was discarded this turn (dirac/kw_mayhem.go)
	Aftermath      bool      // K:Aftermath on a split card's right half (CR 702.127a): CardDef.Aftermath (dirac/face_aftermath.go)
	BeamMeUp       *manaCost // K:Beam me up:<cost>: a graveyard cast for this cost that also returns a creature you control (FbCost)
	Escape         *manaCost // K:Escape:<cost> (CR 702.138a): a graveyard cast for this cost while exiling the printed exiles (EscCost; dirac/kw_escape.go)
	EscCost        *lCost    // the escape cost's ExileFromGrave<N/Card.Other> part (lower_escape.go; a battlefield-exile part beside it refuses)
	Harmonize      *manaCost // K:Harmonize:<cost> (CR 702.180a): a graveyard cast for this cost, one creature tapped for its power off the generic part
	Retrace        bool      // K:Retrace (CR 702.81a): a graveyard cast paying the PRINTED cost plus the additional discard-a-land cost (dirac/kw_retrace.go); never a substitution
	RetraceCost    *lCost    // the retrace cast's additional cost: Discard<1/Land> (CardDef.RetraceCost)
	AltAddCost     *lCost    // AlternateAdditionalCost: pay this instead of the printed additional cost
	SpellCost      *lCost    // additional costs on the spell (Sac/Discard)
	AltAddOne      bool      // SpellCost is a one-part AlternateAdditionalCost (gorge's planner: shape:optional_cost)
	Leyline        bool
	World          bool      // the printed World supertype (CR 205.4a, 704.5k): CardDef.World
	SelfFlash      bool      // a CastWithFlash EffectZone$ All self grant (Illusion Spinners; CardDef.SelfFlash)
	Changeling     bool      // K:Changeling: every creature type (CR 702.73a), in every zone
	Offspring      *manaCost // K:Offspring:<cost> (CR 702.175): the kernel's kicked cast and Kicker cost, CardDef.Offspring
	Buyback        *manaCost // K:Buyback:<cost> (CR 702.27a): the kernel's kicked cast and Kicker cost, CardDef.Buyback; a Sac<1/..> cost is zero mana plus KickerCost
	Casualty       int       // printed K:Casualty:<N> (CR 702.153a, fixed N >= 1), CardDef.Casualty
	Bargain        bool      // printed K:Bargain (CR 702.166), CardDef.Bargain
	Storm          bool      // printed K:Storm (CR 702.40a), CardDef.Storm: the kernel's own native cast trigger (dirac/kw_storm.go)
	BeholdN        int       // RaiseCost Cost$ BeholdExile<1/Type> (the Lorwyn Champions): behold one permanent or hand card of BeholdF and exile it (lower_beholdexile.go)
	BeholdF        *lFilter
	BeholdExile    bool
	SpellWaterbend int      // RaiseCost Cost$ Waterbend<N> on the spell itself (lower_raisewaterbend.go): N generic payable by tapping artifacts/creatures, CardDef.SpellWaterbend
	OptCost        *lCost   // S:Mode$ OptionalCost's Blight<N> or Behold<1/Type> part (CR 601.2b), CardDef.OptCost/OptBehold (lower_optcost.go)
	Teamwork       int      // printed K:Teamwork:<N> (CR 702.194, N 1..15), CardDef.Teamwork
	ModesPickTW    int      // a Teamwork charm's CharmNum$ X over Count$Teamwork.<a>.<b>: <a> picks teamworked, <b> otherwise (CardDef.ModesPickTW)
	ModesPickV     lValue   // a spell or triggered charm's CharmNum$ over Count$Compare or a branch head (constant arms): the pick count, evaluated as the modes are chosen, or a general count beside MinCharmNum$ 0 (CardDef.ModesPickV; lower_charmcmp.go)
	ModesMinCount  int      // a charm with no MinCharmNum$ over a two-arm count: its smaller arm, the offer gate's minimum (CardDef.ModesMinCount; lower_charmcmp.go)
	TrigCharmFixed bool     // a triggered charm with a fixed pick count, which a ModesPickV would also reach in the kernel
	DungeonNext    []uint8  // a dungeon entry's room arrows (CardDef.DungeonNext; lower_initiative.go)
	TypeValid      []uint16 // an as-enters ChooseType's ValidTypes$ vocabulary indexes (CardDef.TypeValid; lower_choosetype.go)
	SpentSelf      bool     // the card reads Count$CastTotalManaSpent in a spell chain or a self-ETB trigger: gorge's planner declines its cast (CardDef.SpentSelf)
	DmgAmt         bool     // the card reads Count$TotalDamageReceivedThisTurn (CardDef.DmgAmt)
	CastBF         *lFilter // the card reads Count$LastStateBattlefieldWithFallback <spec> in a spell's own program (CardDef.CastBF)
	Gift           bool     // K:Gift (CR 702.174): the GiftAbility body is spliced at the head of the spell, or is the permanent's ETB gift trigger
	Landwalk       *lFilter // K:Landwalk:<land filter> (CR 702.14)
	ProtColors     string   // printed "Protection from <colour>" (CR 702.16): WUBRG letters
	HexColors      string   // printed K:Hexproof:<Colour> (CR 702.11d): WUBRG letters
	ProtFrom       *lFilter // printed K:Protection:<spec> (CR 702.16): the sources it protects from
	Infect         bool     // printed K:Infect (CR 702.90)
	Toxic          int      // printed K:Toxic:<N> instances, summed (CR 702.164c)
	Cascade        int      // printed K:Cascade instances (CR 702.85a/c): CardDef.Cascade
	PreventDmgTo   bool     // printed "Prevent all damage that would be dealt to CARDNAME." (CardDef.PreventDmgTo)
	PrevCmbTo      bool     // printed combat-scoped "dealt to CARDNAME" (CardDef.PreventCombatTo)
	PrevCmbBy      bool     // printed combat-scoped "dealt by CARDNAME" (CardDef.PreventCombatBy)
	Evasion        uint8    // printed Horsemanship / Intimidate (kop.Ev* bits)
	Wither         bool     // printed K:Wither (CR 702.80)
	AloneAtk       bool     // the printed can't-attack-alone keyword (CR 506.5)
	AloneBlk       bool     // the printed can't-block-alone keyword, and the compound's block half (CR 506.5)
	Engines        bool     // printed K:Start your engines (CR 702.179a)
	AtkFace        bool     // the printed face's lines may observe an attack declaration (CardDef.AtkFace)
	SagaN          int      // K:Chapter:<count>: the final chapter number (CR 714.1)
	TapPowerCrew   int      // S:Mode$ TapPowerValue for Crew (graveexile)
	TapPowerSaddle int      // S:Mode$ TapPowerValue for Saddle (graveexile)
	MinBlockers    int      // S:Mode$ MinMaxBlocker | ValidCard$ Creature.Self | Min$ N (CR 509.1b)
	ExtraBlocks    int      // S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ 1 (CR 509.1a)
	Lure           bool     // the all-must-block keyword: every able creature blocks it (CR 509.1c)
	MustBeBlocked  bool     // the must-be-blocked keyword (CR 509.1c)
	Daybound       bool     // K:Daybound (CR 702.145a-d)
	Nightbound     bool     // K:Nightbound (CR 702.145e-g)
	Abilities      []*lAbility
	Statics        []lStatic
	// EquipGrants: the OvGate-gated equip cost reductions a copy exception of
	// this card grants its token (CopyPermanent AddStaticAbilities$, Firion);
	// lowerPool adds each to every pool card with an Equip ability
	// (lower_copygrants.go)
	EquipGrants []lStatic
	Repls       []lRepl
	Tokens      []string // token script keys this card creates
	// Faces is a multi-face card's faces: the
	// pool names of every face, Faces[0] the front (the card itself), the
	// others face keys lowered as extra pool entries beside the tokens;
	// Face is this entry's index and FaceMode the card's kop.Face* layout.
	Faces []string
	Face  int
	// FuseKey is a fuse card's fused split spell entry (lower_fuse.go), one
	// of its Tokens: CardDef.FuseDef. FusedSpell marks that entry.
	FuseKey    string
	FusedSpell bool
	// printed is a token or face entry's printed name (Name is its pool
	// key); a named<X> filter resolves X against it
	printed  string
	FaceMode uint8
	// RoomSplit is a Room state def's count of leading Abilities lowered
	// from door Face (lower_rooms.go); the rest are the other door's.
	RoomSplit  int
	Emblems    []lEmblem
	Refusals   []refusal
	Unreadable []refusal // every-blocker census only: params a refused node left unread

	overrides []*lOverride // override records the Override ops index
}

// lEmblem is an emblem a planeswalker ultimate creates: statics, or one
// latent triggered ability (by index).
type lEmblem struct {
	Statics       []lStatic
	Trigger       int
	UntilEOT      bool // an Effect without a Duration$: it ends at cleanup
	UntilNextTurn bool // Duration$ UntilYourNextTurn: it ends at the next turn's cleanup
	WhileHost     bool // Duration$ UntilHostLeavesPlayOrEOT: it ends when its host leaves too
	// Capture: RememberObjects$ Targeted over the chain's target slot
	// (CaptureSet); the trigger's ValidSource$ ...IsRemembered is then the
	// emblem's own capture, lowered without the Remembered flag
	Capture    lRef
	CaptureSet bool
	// CaptureIsPlayer: the capture names the chain's PLAYER target (an
	// Effect's RememberObjects$ TargetedPlayer), so build_ab sends it to
	// kop.Emblem.CapturePlayer rather than Capture.
	CaptureIsPlayer bool
	// CaptureLapse: the capture ends when the object leaves the battlefield
	// (ForgetOnMoved$ / ExileOnMoved$ Battlefield; kop.Emblem.CaptureLapse)
	CaptureLapse bool
	// Capture2: the second target slot an Effect remembers beside Capture
	// (T'Pring: RememberObjects$ Targeted over both fighters; kop.Emblem.Capture2)
	Capture2    lRef
	Capture2Set bool
	// CaptureRem: the emblem remembers the resolution's remembered set
	// (RememberObjects$ RememberedLKI; kop.Emblem.CaptureRem)
	CaptureRem bool
	// Source: the trigger fires for the emblem's own creator (ValidCard$
	// Card.EffectSource; kop.Emblem.Source)
	Source bool
	// NotHere: made only while the host is off the battlefield (kop.Emblem.NotHere)
	NotHere bool
	// NumAmount and Once: the frozen number rides the trigger as its amount,
	// and the emblem ends after it fires once (OneOff$ True)
	NumAmount, Once bool
}

type refusal struct {
	Card   string
	Node   string // IR node kind: "A[0] SP$ Charm", "T[1] Mode=Phase", "S[0]", "K:Hexproof", ...
	Reason string
}
