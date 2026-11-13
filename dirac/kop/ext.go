package kop

// Extension sub-ops (OpExt): the op space is full (the iota list below
// OpCount and the high range 250-254), so a new family adds an ExtOp here,
// numbered from 1 up and append-only, and lowers its instruction as
// {Op: OpExt, A: Const(sub)} with the rest of the fields its own. The
// kernel dispatches them in a cold file (extop.go):
// one extra switch only on an ext instruction, nothing on the hot path.
type ExtOp int16

const (
	// ExtNone is no sub-op (an OpExt instruction never carries it).
	ExtNone ExtOp = iota
	// ExtPreventColor (Prismatic Strands): the resolving
	// controller chooses a colour (ChGenColor, all five); all damage that
	// sources of that colour would deal this turn is prevented (gorge
	// effChooseColor + effEffect's bodyless DamageDone Prevent$
	// replacement, ValidSource$ Card.ChosenColor). Each effect keeps the
	// colour chosen as it was created. No other field.
	// dirac/cast_altfree.go.
	ExtPreventColor
	// ExtEachDamage (Case of the Gateway Express): each
	// battlefield permanent matching F (seat 0 first, entry order) -- or,
	// with damager target slots in C>>8 (Common Goal: a bit per
	// slot, gorge DefinedDamagers$ ParentTarget), those slots' objects --
	// deals B damage (B = Const(0): its own power) to the object in Ref
	// C&0xff, one simultaneous damage event whose hits each name their own
	// source (gorge effEachDamage: BeginDamageBatch .. EndDamageBatch).
	// dirac/extop.go.
	ExtEachDamage
	// ExtRemoveCounterAny (Glissa Sunslayer): remove up to B.N
	// counters of any kinds from the object in Obj, the controller picking
	// the kind one counter at a time (C 1: UpTo$, a Done option at every
	// step); the picks are removed together once the election ends (gorge
	// removeCounterAny, cr-removecounter-any-upto 461d8b818).
	// dirac/ctr_removeany.go.
	ExtRemoveCounterAny
	// ExtTwoPilesSac (Liliana of the Veil): the resolving
	// controller separates the battlefield permanents matching F that the
	// player in Obj controls into two piles (ChGenPileSplit, a permanent
	// per pick, AkDone-closed; all picked ends it), that player picks one
	// (ChGenPileChoose) and sacrifices every permanent in it (gorge
	// effTwoPiles' battlefield arm then SacrificeAll IsRemembered,
	// cr-twopiles-battlefield-validcards). dirac/act_piles.go.
	ExtTwoPilesSac
	// ExtChooseColorRes (Skrelv, Defector Mite): the resolving
	// controller chooses a colour (ChGenColor, all five); it rides
	// res.chosen (colour index + 1) for the ExtColorGrant that follow,
	// and res.col for the ChosenColor filters of the rest of the
	// resolution (gorge effChooseColor Defined$ You records it on the
	// source, chosenColorMatches reads it). dirac/static_colourgrant.go.
	ExtChooseColorRes
	// ExtColorGrant (Skrelv): the object in Obj gains, until end
	// of turn and for the colour in res.chosen, hexproof from that colour
	// (C bit 0), toxic B.N (B) and "can't be blocked by creatures of that
	// colour" (C bit 1) (gorge definedKWGrant and the CantBlockBy Effect's
	// freezeChosenColor, cr-pump-definedkw-chosencolor); protection from
	// that colour (C bit 2) for Obj, or, with a filter F, for every
	// permanent matching F as it resolves (gorge effProtectionAll Gains$
	// ChosenColor; Akroma's Blessing).
	// dirac/static_colourgrant.go.
	ExtColorGrant
	// ExtMayPlayGrave (Case of the Uneaten Feast): every
	// graveyard card matching F (seat 0's graveyard first, zone order) gets
	// a may-play-from-the-graveyard permission until end of turn for the
	// resolving controller (B: kop.MayPlayNonLand skips lands); gorge
	// effAnimate over Defined$ ValidGraveyard granting a self MayPlay
	// static. dirac/cast_graveyardgrant.go.
	ExtMayPlayGrave
	// ExtHideaway (Collector's Cage; CR 702.75a): C 0 is the
	// Hideaway ETB: the controller looks at the top B cards of their library
	// (B a constant), exiles one face down linked to the source and puts the
	// rest on the bottom in a random order (gorge effHideaway, KChoose
	// "hideaway_pick"). C 1 is the Play of the card exiled with the source
	// (gorge effPlay Defined$ ExiledWith, Optional$, WithoutManaCost$, a land
	// included): the ChGenCastFree ask over it.
	// dirac/kw_hideaway.go.
	ExtHideaway
	// ExtEndure (CR 701.63): the object in Obj endures N,
	// N the PA of override C (read once; N <= 0 does nothing). On the
	// battlefield its controller chooses (ChGenEndure: 0 N +1/+1 counters
	// on it, 1 an N/N white Spirit token, B.N its card); gone, the
	// resolving controller gets the Spirit with no ask (gorge effEndure).
	// dirac/act_endure.go.
	ExtEndure
	// ExtDividedCounters (Quirion Beastcaller): put the dying
	// source's last-known +1/+1 counters (the entry's LKI) on the objects of
	// target slot C, as the controller divided them while putting the
	// trigger on the stack (TargetSpec.Divide == DivLKIP1P1: the slot takes
	// 0..X targets, the total is X; CR 601.2d, 603.10a; gorge cr-dies-trigger-
	// divided-counters-lki). B.N is the counter kind the shares put (0,
	// CtrP1P1, for every form but the literal-N one; Contagion's -2/-1).
	// dirac/ctr_divided.go.
	ExtDividedCounters ExtOp = 30
	// ExtSacDelayed (Mobilize): sacrifice, as one batch, the
	// group program's remembered objects (trigObj and the one packed in
	// trigAmt) still on the battlefield. dirac/kw_mobilize.go.
	ExtSacDelayed ExtOp = 32
	// ExtPlotGrant (Kellan Joins Up): every object in the
	// resolution's remembered set that is in exile becomes plotted (CR
	// 702.170b-d): a DurPlot record stamped with the current turn, then its
	// own BecomesPlotted triggers (gorge effAlterAttribute "Plotted", zone
	// gate ZExile; the fold's PlottedTurn). Obj is the Remembered ref (read
	// by the kernel as the remembered set). dirac/kw_plot.go.
	ExtPlotGrant ExtOp = 31
	// ExtCounterLose (Tishana's Tidebinder): counter the stack
	// ability in target slot Obj (RefTarget0..2); if its source is still the
	// same artifact, creature or planeswalker on the battlefield, that
	// permanent loses all abilities for as long as the resolving source stays
	// on the battlefield (C: the Table.Overrides index of the host-bound
	// LoseAbilities override; gorge cr counter RememberCountered$ remembers
	// the countered ability's source card). dirac/act_counterlose.go.
	ExtCounterLose ExtOp = 41
	// ExtNumTimes (Tranquil Frillback): a printed trigger's own
	// "you may pay {unit} up to N times" announcement (gorge cumulative.go
	// triggeredCostNumTimesAsk): the controller announces 0..m, m the most
	// repetitions (at most N = C>>4&7) that the potential mana covers of
	// the unit (generic C&15, packed pips B.N); the answer rides the
	// resolution's X (0 declines and jumps C>>7 instructions). The IfPaid
	// right after it charges the unit that many times. With no repetition
	// payable nothing is asked and the IfPaid poses its decline-only ask.
	// dirac/cast_announcetimes.go.
	ExtNumTimes ExtOp = 50
	// ExtAegisClone (Assimilation Aegis): the resolution of a
	// TrigAttached trigger. The attachment's controller picks a creature
	// card exiled with the attachment (the exile-until-leaves link of its
	// incarnation; the ask is gorge's clone_choice KChoose, one option
	// whatever the pool) and the bearer (RefTriggerObj) becomes a copy of it
	// for as long as the attachment stays attached to it (gorge Clone
	// Duration$ UntilUnattached, ChoiceZone$ Exile). dirac/zone_exilethenattach.go.
	ExtAegisClone ExtOp = 80
	// ExtAirbend (CR 701.65a): exile the battlefield
	// permanent Obj (or, with Obj RefNone, every battlefield permanent
	// matching F, seat 0 first; C = 1+slot excludes that target slot's
	// object, Defined$ ...NotDefinedTargeted), and give each card exiled
	// that way its owner's DurAirbend record: the owner may cast it from
	// exile for {2}. dirac/act_airbend.go.
	ExtAirbend ExtOp = 95
	// ExtManaChoice (Chandra, Hope's Beacon; CR 605.1a): a resolving
	// Mana effect's colour choice, "add two mana in any combination of
	// colors" on a loyalty ability, or a triggered "add four mana in any
	// combination of colors" (Realm-Scorcher Hellkite; CR 106.3, 605.1a;
	// gorge effects.askManaChoice, Produced$
	// Combo Any with Amount$ 2 / Produced$ Any): B the amount (constant, at
	// most 7), C the colour mask (W U B R G bits 0..4, two or more) with bit 8
	// for the any-one-colour form (one pick for the whole amount; else one
	// pick per mana, ChGenManaChoice). dirac/mana_colourchoice.go.
	ExtManaChoice ExtOp = 100
	// ExtLibSweep (Teferi, Temporal Pilgrim): every battlefield
	// permanent matching F (as the resolving controller) goes to its owner's
	// library, on top in move order (seat 0's battlefield, then seat 1's),
	// then each owner whose cards moved, in order of first placement, shuffles
	// its library (gorge effChangeZoneAll Destination$ Library, Shuffle$
	// True). dirac/zone_sweeptolibrary.go.
	ExtLibSweep ExtOp = 101
	// ExtBreachChoose (Breach the Multiverse): for each player (the
	// resolving controller first, then the other), the controller chooses a
	// creature or planeswalker card in that player's graveyard (a mandatory
	// ChGenChooseCard ask, posed with one candidate too, none when the
	// graveyard holds none) and imprints it: the picks accumulate in the
	// chosen set (RefChosen, OpClearChosen clears it), in pick order (gorge
	// RepeatEach Player over ChooseCard Defined$ You Choices$
	// Creature.RememberedPlayerCtrl,Planeswalker.RememberedPlayerCtrl
	// ChoiceZone$ Graveyard ImprintChosen$ True). No other field.
	// dirac/act_millthenreturn.go.
	ExtBreachChoose ExtOp = 160
	// ExtBreachReturn (Breach the Multiverse): the chosen set's cards
	// still in a graveyard enter the battlefield together under the resolving
	// controller's control, in pick order, one entering event (gorge
	// ChangeZone Defined$ Imprinted GainControl$ True Origin$ Graveyard
	// Destination$ Battlefield). dirac/act_millthenreturn.go.
	ExtBreachReturn ExtOp = 161
	// ExtExileLandChoose (Ensign Pavel Chekov): the resolving controller
	// chooses a land card the other player (the one opponent of a two-seat
	// game) owns among the remembered cards in exile (a mandatory
	// ChGenChooseCard ask, posed with one candidate too, none when there is
	// no candidate) and imprints it: the chosen set (RefChosen, OpClearChosen
	// clears it) (gorge RepeatEach Opponent over ChooseCard Defined$ You
	// Choices$ Land.IsRemembered+RememberedPlayerOwn ChoiceZone$ Exile
	// ImprintChosen$ True). No other field. dirac/act_exilelandchoose.go.
	ExtExileLandChoose ExtOp = 162
	// ExtRecruit (The Hobbit's Recruit action; CR 701.xx,
	// 701.9, 111.1): the resolving controller draws a card, then discards a
	// card (gorge effRecruit: one KChoose Min 1 Max 1 in hand order, no ask
	// when the hand holds at most one), then creates one 1/1 white Human
	// Soldier token only if the discarded card was a nonland. The pick is
	// the kernel's ChRecruitDiscard ask, CanonKind "discard"
	// (dirac/act_recruit.go).
	ExtRecruit ExtOp = 94
	// ExtPhases (Spectral Adversary; api Phases, CR 702.26): the
	// permanent named by Ref Obj (RefSelf, or a target slot RefTarget0..2,
	// run once per object of a multi-object slot) phases out, and with it
	// every permanent attached to it, indirectly (CR 702.26g). A phased-out
	// permanent is on no battlefield walk (dirac.ZonePhased). With Obj RefNone
	// and F set (AllValid$ <filter>; The City on the Edge of Forever) every
	// battlefield permanent matching F phases out, in object order. The toggle
	// form (PhaseInOrOut$), Phaseout$ False, RememberAffected$ and
	// WontPhaseInNormal$ are refused by kgen. dirac/kw_phasing.go.
	ExtPhases ExtOp = 110
	// ExtLockActs (Braided Net): the permanent in Obj (a target),
	// when it is tapped as this resolves, loses the ability to have its
	// activated abilities activated, mana abilities included, for as long as
	// it stays tapped (gorge Effect Mode$ CantBeActivated | ValidCard$
	// Card.IsRemembered | ValidSA$ Activated, Duration$ UntilTargetedUntaps;
	// CR 602.5, 611.2b): its first untap or departure ends the lock for good,
	// and the source leaving does not. dirac/act_lockpermanent.go.
	ExtLockActs ExtOp = 140
	// ExtTokenPT (DB$ Token TokenPower$/TokenToughness$, "create
	// an X/X token"; CR 111.4, 613.4b): right BEFORE an OpCreateTokenEx. Reads
	// override C's P/T through its PA/TA first (gorge resolves both sides once
	// before the mint loop), runs that CreateToken in place and skips it, then
	// sets the token it minted (res.lastTok) to those values as a permanent
	// layer-7b set stamped at the mint (gorge effects/token.go: one SubSet
	// effect sourced to the token). The Ex form counts the mint: no mint does
	// nothing, more than one stalls StallTokenPT.
	// dirac/act_tokenpt.go.
	ExtTokenPT ExtOp = 170
	// ExtExHaste (Agility Bobblehead): the object in Obj (RefTarget runs once
	// per object of a multi-object slot) can't be blocked this turn except by
	// creatures with haste (gorge Effect Mode$ CantBlockBy ValidBlocker$
	// Creature.withoutHaste; CR 509.1b, 702.10a). dirac/static_cantblockexcept.go.
	ExtExHaste ExtOp = 171
	// ExtDistCounters (Vastwood Hydra): "distribute N +1/+1 counters among
	// any number of creatures you control", N the op's B value (the dying
	// source's last-known counters): no targets. The resolving controller
	// picks 1..min(N, eligible) of the F-matching battlefield permanents
	// (seat 0 first, entry order; one DkChoose per pick, Done after the
	// first), then the N counters go round-robin in pick order, the earlier
	// picks taking the remainder (gorge putCounterPickDistribute). With one
	// eligible permanent it takes all N unasked, with none nothing happens.
	// dirac/ctr_distpick.go.
	ExtDistCounters ExtOp = 310
	// NumExtOps is a fixed capacity of the sub-op tables, not last+1: a new
	// sub-op takes the next free value below it (values are pinned by
	// testdata/ordinals.tsv) and never edits this line. Unused slots stay
	// unnamed. Raise it only when the free values run out.
	NumExtOps ExtOp = 320
)

// ExtDrainMana: the player in Obj loses all unspent mana (gorge
// effDrainMana: one events.ManaClear for that seat; Power Sink's
// unless-payment decline, Mana Short). The pool and its side tallies empty.
// dirac/act_drainmana.go.
const ExtDrainMana ExtOp = 40

func init() { extOpNames[ExtDrainMana] = "DrainMana" }

// extOpNames is filled by per-sub-op init registrations (extOpNames[ExtX] = "X").
var extOpNames [NumExtOps]string

// String names the sub-op (reports and panics only).
func (x ExtOp) String() string {
	if x >= 0 && x < NumExtOps && extOpNames[x] != "" {
		return extOpNames[x]
	}
	return "Ext(?)"
}

// Ext is in's extension sub-op, ExtNone for any other op.
func (in *Instr) Ext() ExtOp {
	if in.Op != OpExt {
		return ExtNone
	}
	return ExtOp(in.A.N)
}

// ExtInstr is an OpExt instruction for sub-op x (its other fields zero).
func ExtInstr(x ExtOp) Instr { return Instr{Op: OpExt, A: Const(int(x))} }

// DB$ AddPhase extra combats, upkeeps and end steps, and the
// FirstAttack$ / FirstCombat$ trigger gates. Numbered explicitly clear of the
// iota lists that sibling families append to.

// ExtAddPhase queues extra phases (gorge effAddPhase). A holds the ext
// selector like every OpExt; B is the count (NumPhases$, a constant or
// TriggerCount$DamageAmount); C is the kind | the splice point << 4. Obeka,
// Splitter of Seconds, Aurelia the Warleader, Y'shtola Rhul.
const ExtAddPhase ExtOp = 193

func init() {
	extOpNames[ExtAddPhase] = "AddPhase"
}

// ExtSkipPhase: the player Obj skips every combat phase of their next turn
// (gorge effSkipPhase, DB$ SkipPhase with Phase$ BeginCombat, Start$ FromNext,
// Duration$ UntilTheEndOfTargetedNextTurn; CR 614.10). False Peace, Empty City
// Ruse. dirac/act_skipphase.go.
const ExtSkipPhase ExtOp = 313

func init() {
	extOpNames[ExtSkipPhase] = "SkipPhase"
}

// DB$ Clone on the battlefield (CR 707.2, 707.9b, 613.2a).

// ExtCloneCopy: a battlefield permanent (or every one matching F) becomes a
// copy of another permanent, a layer-1 copy (gorge effClone; Fleeting
// Reflection, Shuri, Deepfathom Echo, Oko the Ringleader, Silent Hallcreeper,
// Naga Fleshcrafter). Obj is the becomer (RefSelf or a target slot), or
// RefNone with F the Valid set of becomers. B.N packs the flags (CloneEOT ..)
// and the template's target slot (CloneTmplShift); C is the copy exceptions'
// override index+1 (0 none). With ClonePick set the op is only the Choices$
// pick: the controller picks one battlefield permanent matching F (the
// template) and the choice rides the resolution's chosen[0]; the copy op that
// follows (behind an OpMayAsk MayGuardCloneCopy when Optional$) reads it.
// dirac/copy_clone.go.
const ExtCloneCopy ExtOp = 81

func init() {
	extOpNames[ExtCloneCopy] = "CloneCopy"
}

// ExtCopyAbility: gorge's CopySpellAbility with no Defined$
// and TargetType$ Activated.YouCtrl,Triggered.YouCtrl (or Triggered.YouCtrl),
// "copy target activated or triggered ability you control; you may choose new
// targets for the copy" (Peter Parker's Camera, Echo Perceptive Prodigy,
// Scientist Supreme of A.I.M., Kirol Attentive First-Year), B times (Amount$:
// Gogo's X, Mister Fantastic's 2; a missing one is Const 1). C>>1 is the target
// slot (a TgtCopyAb or TgtCopyTrig one); C bit 0 is MayChooseTarget$ True: each
// copy's controller may choose new targets as the copy is about to resolve
// (CR 707.10c). The copy is an ordinary stack entry that is neither cast nor
// activated (CR 707.10), so no cost is paid and no activation tally moves. An
// ability with Ability.CantCopy is not copied (gorge SpellCopyAllowed).
const ExtCopyAbility ExtOp = 191

func init() {
	extOpNames[ExtCopyAbility] = "CopyAbility"
}

// ExtCopyTokenCtr (Obj source, B amount, C kind): the token copies of Obj
// enter with B counters of kind C (gorge copypermanent.go's
// WithCountersType$/WithCountersAmount$; CR 122.6, 614.1c). The amount is read
// once for the whole resolution; a token doubler's extra copies each get it.
// No copy exceptions or CopyFlags: kgen refuses those combinations.
// dirac/copy_counters.go.
const ExtCopyTokenCtr ExtOp = 172

func init() {
	extOpNames[ExtCopyTokenCtr] = "CopyTokenCtr"
}

// CopyPermanent and Animate over Defined$ Valid <filter>.

// ExtCopyAllEach: for each battlefield permanent matching F (the OpGoad sweep
// shape: seat 0 first, entry order, the set read once), create token copies
// for the resolving controller: every source's count through the CreateToken
// replacements first, then the mints source by source (gorge CopyPermanent's
// Defined arm). C is the copy exceptions' override index+1, 0 for none
// (dirac/copy_definedvalid.go).
const ExtCopyAllEach ExtOp = 224

func init() {
	extOpNames[ExtCopyAllEach] = "CopyAllEach"
}

// ExtEffEtbCtr: a SpellCast trigger's Effect that gives
// the spell that was cast "N additional +1/+1 counters as it enters" (Torgal,
// A Fine Hound; Wildgrowth Archaic: DB$ Effect | RememberObjects$
// TriggeredCard | SetChosenNumber$ N | ExileOnMoved$ Stack | ReplacementEffects$
// <Event$ Moved ... Destination$ Battlefield ... PutCounter ETB$ True>). B is
// N, evaluated once as the Effect resolves (gorge's frozen ChosenNumber); the
// spell's stack entry gains N extra P1P1 counters (stackEntry.spc bits 11-15)
// that enterGen puts on the permanent as it enters. The effect ends when the
// spell leaves the stack (ExileOnMoved$ Stack), so a spell already gone gets
// nothing.
const ExtEffEtbCtr ExtOp = 197

func init() {
	extOpNames[ExtEffEtbCtr] = "EffEtbCtr"
}

// ExtExileExMark: "if that creature would die this
// turn, exile it instead" on a Pump or Fight (gorge effects/damage.go
// registerReplaceDying over the Targeted objects; ReplaceDyingDefined$
// Targeted; CR 614.1a, 614.9). The object in Obj (RefTarget runs once per
// object of a multi-object slot) that is still on the battlefield takes the
// kernel's turn-scoped exile-instead mark (markExileEx). C is a Fight's
// fighter ref (RefNone for a Pump): the mark lands only while the fighter and
// the object are both creatures on the battlefield (gorge fightPairs). Emitted right after
// the pump's or the fight's own events, before any state-based action.
// dirac/repl_exilemark.go.
const ExtExileExMark ExtOp = 51

func init() { extOpNames[ExtExileExMark] = "ExileExMark" }

// "CARDNAME must be blocked if able." granted until end
// of turn (CR 509.1c; gorge hidden keyword MustBlock, read layer-derived per
// attacker by combat.DerivedHiddenFlags). The printed form is
// CardDef.MustBeBlocked. Numbered clear of the siblings' ordinals.

// ExtMustBeBlocked: the battlefield objects named by Obj (RefSelf, a target
// slot, ...), or with Obj RefNone every battlefield permanent matching F
// (PumpAll's ValidCards$, seat 0 first, entry order), are marked must be
// blocked this turn if able. The mark is an arena slot bit (mechMustBlocked,
// no State byte): cleared as the next turn begins and when the permanent
// leaves the battlefield. Pump KW$ HIDDEN CARDNAME must be blocked if able.,
// PumpAll's, an Animate's HiddenKeywords$. dirac/combat_mustbeblocked.go.
//
// C = MustBeBlockedLure marks the permanent a lure instead ("All creatures able
// to block CARDNAME do so.", CR 509.1c; Pump KW$ HIDDEN: Alluring Scent):
// the arena bit mechLure, same lifetime. Only a single Obj. dirac/combat_lure.go.
const ExtMustBeBlocked ExtOp = 194

// MustBeBlockedLure is ExtMustBeBlocked's C for the lure mark.
const MustBeBlockedLure = 1

func init() {
	extOpNames[ExtMustBeBlocked] = "MustBeBlocked"
}

// ExtEnterUntapped (CR 614.1d, 701.26b): the body of "lands
// you control enter untapped" (The Wandering Minstrel, Spelunking; gorge
// R:Event$ Moved ... ReplaceWith$ ETBUntapped, DB$ Untap | ETB$ True |
// Defined$ ReplacedCard). Run on the entering permanent (the replaced
// object) it clears the tapped bit. It is not OpUntap: that op removes a
// stun counter instead of untapping (CR 122.1d), and an entering permanent
// carries no counters, so the replacement is the plain untap event (gorge
// effects/untap.go). Lowered only as the whole program of a
// ReplOthersEnterProgram; competing with an enters-tapped body it is
// ordered by the CR 616.1 entry-order ask (dirac/repl_entryorder.go).
const ExtEnterUntapped ExtOp = 228

func init() { extOpNames[ExtEnterUntapped] = "EnterUntapped" }

// ExtEnterAttacking (CR 508.4, 614.12): the second half of "creatures you
// control enter tapped and attacking" (Kira Nerys, Firebrand; gorge R:Event$
// Moved ... ReplaceWith$ DB$ ChangeZone | Defined$ ReplacedCard | Tapped$ True
// | Attacking$ True), after OpEnterTapped. Run on the entering permanent (the
// replaced object) it marks it attacking the controller's opponent (the one
// CR 508.4 defender; two or more stall), declared nothing, so no Attacks
// trigger fires. Only in declare attackers..end of combat of the controller's
// own turn. No other field. dirac/repl_enterattacking.go.
const ExtEnterAttacking ExtOp = 250

func init() { extOpNames[ExtEnterAttacking] = "EnterAttacking" }

// ExtExchangeControl (CR 701.12; gorge effects/exchange_control.go
// effExchangeControl): C is a bit set of the resolving entry's target slots.
// The battlefield objects they name that are still legal, in slot then pick
// order, are the pair: when exactly two differently controlled ones remain,
// each goes to the other's controller for no duration. Anything else does
// nothing. dirac/act_exchangecontrol.go.
const ExtExchangeControl ExtOp = 244

func init() { extOpNames[ExtExchangeControl] = "ExchangeControl" }

// operations on stack entries that already exist.
const (
	// ExtChangeTargets (CR 115.7; gorge effChangeTargets, choose_control.go):
	// the stack entry named by Obj (a TgtStackObj target slot, or
	// RefTriggerObj, the spell whose cast fired this trigger) has its one
	// target changed. The controller picks the new target among the entry's
	// legal targets as its own controller would choose them; C bit 0:
	// Optional$ True (a Done option keeps the old target). An entry with no
	// target is left alone. dirac/act_stackrewrite.go.
	ExtChangeTargets ExtOp = 225
	// ExtCounterAll (CR 701.5; gorge effCounter, Defined$ ValidStack): counter
	// every spell and ability on the stack that an opponent of the resolving
	// controller controls (an uncounterable spell is skipped, an ability is
	// removed), and count the entries countered (the Count$RememberedSize
	// that follows). dirac/act_stackrewrite.go.
	ExtCounterAll ExtOp = 226
)

func init() {
	extOpNames[ExtChangeTargets] = "ChangeTargets"
	extOpNames[ExtCounterAll] = "CounterAll"
}

// ExtFinalityCtr: a SpellCast trigger's Effect that
// gives the cast spell "enters with a FINALITY counter" (Intrepid
// Paleontologist, Leonardo, Sewer Samurai, Osteomancer Adept — the same
// DB$ Effect | ReplacementEffects$ <Moved -> PutCounter ETB$ True>
// | RememberObjects$ TriggeredCard shape the P1P1 riders use, with a
// constant CounterNum$ 1). The spell's stack entry gains one (a spell's
// stackEntry.sid is its rider count: several grants can cover one cast —
// Leonardo beside active Osteomancer effects — and gorge runs each body's
// PutCounter separately); enterGen puts each through the counter
// replacements, after the printed enters-with counters and before the
// entry's triggers.
const ExtFinalityCtr ExtOp = 198

func init() {
	extOpNames[ExtFinalityCtr] = "FinalityCtr"
}

// the suspected designation (CR 701.60; gorge effAlterAttribute
// Attributes$ Suspected, Object.Suspected). Numbered clear of the siblings'
// ordinals (filters.go, ext.go).

// ExtSuspect: the objects named by Obj (RefSelf, a target slot, the trigger
// object, ...), or with Obj RefNone every battlefield permanent matching F
// (Defined$ Valid <filter>, the OpGoad sweep shape: seat 0 first, entry
// order), become suspected (C bit0 clear) or no longer suspected (C bit0 set,
// Activate$ False). The designation is an arena slot bit (mechSuspected): a
// suspected creature has menace and can't block, and the designation ends
// when the permanent leaves or when any control change moves it (CR 701.60b-d;
// dirac/act_suspect.go). The Optional$ election rides a preceding
// OpMayAsk (MayGuardSuspect).
const ExtSuspect ExtOp = 190

func init() {
	extOpNames[ExtSuspect] = "Suspect"
}

// ExtSweepFrom (CR 120.3, 120.4): DamageAll with
// DamageSource$ ParentTarget over "<spec>.NotDefinedTargeted" ("target
// creature you control deals damage equal to its power to each other
// creature", Waltz of Rage, Nibelheim Aflame; gorge effects/damage.go
// effDamageAll). Obj the source's target slot, B the amount (read once),
// F the battlefield creature filter, C bit 0 set: the source's own slot is
// not hit. Every matching creature is hit in one damage batch whose hits all
// name the source (its deathtouch, lifelink, infect and wither apply).
// dirac/act_sweepdamage.go.
const ExtSweepFrom ExtOp = 97

func init() { extOpNames[ExtSweepFrom] = "SweepFrom" }

// ExtToGraveyard: move the object Obj names to its
// owner's graveyard. gorge's ChangeZone Defined$ TopOfLibrary Origin$ Library
// Destination$ Graveyard Optional$ True (Gathering Stone, Traveling Botanist),
// the else branch after an optional PeekAndReveal: a ChangeZone move, not the
// mill keyword action (CR 701.17a), so it shares nothing with OpMill. Obj is
// RefLibTop there, under OpMayAsk MayGuardLibraryTop.
const ExtToGraveyard ExtOp = 192

func init() {
	extOpNames[ExtToGraveyard] = "ToGraveyard"
}

// ExtUnlockDoor (CR 709.5f-g; gorge
// effects/unlockdoor.go effUnlockDoor): DB$ UnlockDoor over one Room target
// slot (Obj). C 0 unlocks a locked door; C 1 first asks the controller
// whether to unlock (option 0) or lock, then picks the door. The kernel
// swaps the Room permanent's door state without paying (this is not the
// unlock special action) and fires the unlocked door's triggers and the
// fully-unlock watchers; a lock clears the designation and fires nothing
// (gorge DoorLock). dirac/face_unlockdoor.go.
const ExtUnlockDoor ExtOp = 227

func init() { extOpNames[ExtUnlockDoor] = "UnlockDoor" }

// Spectral Adversary's sub-op name (dirac/kw_phasing.go).
func init() {
	extOpNames[ExtPhases] = "Phases"
}

// Breach the Multiverse's sub-op names (dirac/act_millthenreturn.go).
func init() {
	extOpNames[ExtBreachChoose] = "BreachChoose"
	extOpNames[ExtBreachReturn] = "BreachReturn"
	extOpNames[ExtExileLandChoose] = "ExileLandChoose"
}

// Braided Net's sub-op name (dirac/act_lockpermanent.go).
func init() {
	extOpNames[ExtLockActs] = "LockActs"
}

// Ext sub-op names, registered per sub-op. A new sub-op registers its own
// name from its own cold file: func init() { extOpNames[ExtX] = "X" }.
func init() {
	extOpNames[ExtNone] = "None"
	extOpNames[ExtPreventColor] = "PreventColor"
	extOpNames[ExtEachDamage] = "EachDamage"
	extOpNames[ExtRemoveCounterAny] = "RemoveCounterAny"
	extOpNames[ExtTwoPilesSac] = "TwoPilesSac"
	extOpNames[ExtChooseColorRes] = "ChooseColorRes"
	extOpNames[ExtColorGrant] = "ColorGrant"
	extOpNames[ExtMayPlayGrave] = "MayPlayGrave"
	extOpNames[ExtHideaway] = "Hideaway"
	extOpNames[ExtEndure] = "Endure"
	extOpNames[ExtDividedCounters] = "DividedCounters"
	extOpNames[ExtPlotGrant] = "PlotGrant"
	extOpNames[ExtSacDelayed] = "SacDelayed"
	extOpNames[ExtCounterLose] = "CounterLose"
	extOpNames[ExtNumTimes] = "NumTimes"
	extOpNames[ExtAegisClone] = "AegisClone"
	extOpNames[ExtAirbend] = "Airbend"
	extOpNames[ExtManaChoice] = "ManaChoice"
	extOpNames[ExtLibSweep] = "LibSweep"
	extOpNames[ExtRecruit] = "Recruit"
	extOpNames[ExtExHaste] = "ExHaste"
	extOpNames[ExtDistCounters] = "DistCounters"
}

func init() { extOpNames[ExtTokenPT] = "TokenPT" }

// ExtPreventDamageScope: all damage that would be dealt this turn to the
// resolving controller (C bit PrevScopePlayer) and/or the creatures they
// control (PrevScopeCreatures), optionally only by creature sources on the
// battlefield (PrevScopeSrcCreature), is prevented (gorge effEffect's
// bodyless DamageDone Prevent$ replacement in the Command zone). No other
// field. dirac/repl_preventscope.go.
const ExtPreventDamageScope ExtOp = 246

// The C bits of ExtPreventDamageScope.
const (
	PrevScopePlayer = 1 << iota
	PrevScopeCreatures
	PrevScopeSrcCreature
)

func init() { extOpNames[ExtPreventDamageScope] = "PreventDamageScope" }

// ExtCounterPlusEOT: until end of turn, if one or more counters would be put
// on a permanent the resolving controller controls, that many plus one of
// each of those kinds are put on it instead (Brad Boimler, Eager Ensign's
// tap-triggered Effect; gorge effEffect's AddCounter replacement with no
// ValidCounterType$). A per-player count in genState.plusCtr, cleared at
// cleanup. No other field. dirac/ctr_pluseot.go.
const ExtCounterPlusEOT ExtOp = 247

func init() { extOpNames[ExtCounterPlusEOT] = "CounterPlusEOT" }

// ExtGrantPanharm (Lwaxana Troi, Charming Guest): the object in Obj (a target
// slot) gains "if a triggered ability of this permanent triggers, that
// ability triggers an additional time" for as long as the resolving ability's
// source stays tapped on the battlefield and the object stays on the
// battlefield (gorge Effect Mode$ Panharmonicon | ValidCard$ Card.IsRemembered,
// Duration$ UntilUntaps, ExileOnMoved$ Battlefield; CR 603.2d, 611.2b). Nothing
// is granted when the source is not a tapped permanent as this resolves. One
// record per grant in genState.pgT / pgH, counted by the echo walk beside the
// printed StatPanharmonicon statics. dirac/act_grantpanharm.go.
const ExtGrantPanharm ExtOp = 248

func init() { extOpNames[ExtGrantPanharm] = "GrantPanharm" }

// ExtSearchCounters (Captain Jean-Luc Picard): the WithCountersType$ /
// WithCountersAmount$ rider of a library search onto the battlefield. B is
// the amount (a positive constant) and C the counter kind: the found card
// that enters the battlefield consumes genState.enterCtr and enters with
// that many counters (CR 122.6), exactly as a graveyard pick's rider does.
// B <= 0 clears the pending entry counters, so kgen emits the op again
// right after the OpSearchEx. dirac/ctr_search.go.
const ExtSearchCounters ExtOp = 252

func init() { extOpNames[ExtSearchCounters] = "SearchCounters" }

// ExtPutCounterExisting (Dathon, His Heart Open): "choose a counter on target
// enchantment you control, put another of that kind on it" (gorge
// putCounterExisting, CounterType$ ExistingCounter). Obj is the recipient and B
// the amount (a constant of at least 1). The kinds on the recipient are listed
// in kernel kind order: none does nothing, one is put unasked, two or more
// ask the controller (the Glissa Sunslayer kind ask, ChRemCtrKind). The put is
// an ordinary OpPutCounter of the pick, so its replacements, the CR 616.1
// order ask and the counter-added triggers all run. dirac/ctr_putexisting.go.
const ExtPutCounterExisting ExtOp = 253

func init() { extOpNames[ExtPutCounterExisting] = "PutCounterExisting" }

// ExtChooseNumber: an as-enters number choice (CR 614.12; gorge's ETB
// ChooseNumber replacement, etbOptions' "number" arm through
// effects.ETBNumberChoices): the entering permanent's controller chooses a
// number from B.N to C inclusive (a ChGenNumber decision, mode = the number),
// kept in the permanent's chosen byte and read by ExChosenNumber. Only the
// first instruction of an enters program. dirac/turn_gen.go.
const ExtChooseNumber ExtOp = 230

func init() { extOpNames[ExtChooseNumber] = "ChooseNumber" }

// ExtChooseNumberMid: a number chosen while an ability or spell resolves
// (gorge effects.ChooseNumber's non-ETB path, the "pay any amount" family:
// Well of Lost Dreams, Yusri, Fortune's Flame, Tibalt's Trickery, the join
// forces cards, and the energy cards). Obj is the chooser; C&0xff is the
// literal Min$ floor (the corpus has only 0..2), C&0x100 says Max$ is
// present and B is its value (a literal, SVar or Count$ expression), C&0x200
// says Random$ True. The chooser picks from Min..Max (gorge
// effects.chooseNumberAsk); a bound the context cannot honour falls back to
// the deterministic 0, exactly as gorge does, and a present Max$ above the
// chosen byte's 254 (gorge's own offer reaches 1000) is a declared capacity
// stall. The answer is stored on the
// source permanent's chosen byte (or the resolving card object when the
// source is a spell), and read by ExChosenNumber. A single candidate is
// recorded without an ask; a Random$ choice draws from the kernel rng.
// dirac/repl_choosenumber.go.
const ExtChooseNumberMid ExtOp = 303

func init() { extOpNames[ExtChooseNumberMid] = "ChooseNumberMid" }

// ExtGrantExile (Q, Stacking the Deck): every remembered object that is in
// exile gains the activated ability C of the resolving ability's card, an
// ability whose ActZone is Exile (gorge Animate Abilities$ with Duration$
// Permanent: a layer-6 AddAbilities effect that ends when the object leaves
// exile). The grant is a record (host = the resolving ability's source, the
// object and its version) in genState.qgO / qgV / qgH; the object is offered
// the ability while the record is live (dirac/cast_exilegrant.go).
const ExtGrantExile ExtOp = 249

func init() { extOpNames[ExtGrantExile] = "GrantExile" }

// ExtChooseName: an as-enters nonland card-name choice (CR 614.12, 201.4;
// gorge's ETB NameCard replacement, ValidCards$ Card.nonLand, Defined$ You):
// the entering permanent's controller chooses the name of one nonland card
// of the matchup (a ChGenName decision, mode = the card's CardID), kept in
// the permanent's chosen byte as that CardID (Kai Winn, Faithless Leader).
// Only the first instruction of an enters program. dirac/repl_choosename.go.
// 248, 249 and 250 were taken by ExtGrantPanharm, ExtGrantExile and
// ExtEnterAttacking on land after this branch cut, so the Kai Winn sub-op
// takes 251 and Picard's search-counter rider takes 252.
const ExtChooseName ExtOp = 251

func init() { extOpNames[ExtChooseName] = "ChooseName" }

// ExtTurnFaceUp (CR 708.6; gorge effSetState Mode$ TurnFaceUp): an effect
// turns a face-down battlefield permanent face up, no cost paid; its
// turned-face-up triggers fire. A permanent that is not face down, or whose
// printed face is an instant or sorcery, is left alone. C 0: the object in
// Obj (a target; Expose the Culprit). C 1: the resolving controller picks
// up to one battlefield permanent matching F that would turn up, seat 0's
// first in battlefield order, or Done (gorge setStateChoose's
// "setstate_pick", Min 0 Max 1; Hustle // Bustle). dirac/face_turnupeffect.go.
const ExtTurnFaceUp ExtOp = 254

func init() { extOpNames[ExtTurnFaceUp] = "TurnFaceUp" }

// ExtAttacksIfAble: the battlefield object named by Obj (a target slot)
// attacks this turn if able (CR 508.1d; gorge Effect StaticAbilities$
// Mode$ MustAttack | ValidCreature$ Card.IsRemembered over RememberObjects$
// Targeted, ExileOnMoved$ Battlefield): an attack requirement read beside
// the MustAttack static. The mark is an arena slot bit (mechAttacksIfAble,
// no State byte): cleared as the next turn begins and when the permanent
// leaves the battlefield. Heckling Fiends, Incite, Courtly Provocateur.
// dirac/combat_attacksifable.go.
const ExtAttacksIfAble ExtOp = 255

func init() { extOpNames[ExtAttacksIfAble] = "AttacksIfAble" }

// ExtBlocksIfAble: the battlefield object named by Obj (a target slot)
// blocks this turn if able (CR 509.1c; gorge Effect StaticAbilities$
// Mode$ MustBlock | ValidCreature$ Card.IsRemembered over RememberObjects$
// Targeted, ExileOnMoved$ Battlefield, read by combat.MustBlockCandidates):
// a blocker requirement. The mark is an arena slot bit (mechBlocksIfAble, no
// State byte): cleared as the next turn begins and when the permanent leaves
// the battlefield. Culvert Ambusher, Courtly Provocateur, Hustle // Bustle.
// dirac/combat_blocksifable.go.
const ExtBlocksIfAble ExtOp = 256

func init() { extOpNames[ExtBlocksIfAble] = "BlocksIfAble" }

// ExtSpellToLib: the resolving spell moves from the stack to the top of its
// owner's library, then that library is shuffled (CR 608.2n: the spell's own
// instruction replaces the default move to the graveyard; gorge effChangeZone
// Origin$ Stack, Destination$ Library, Shuffle$ True over Defined$ Parent,
// the spell itself). The spell has left the stack when the resolution
// finishes, so it goes to no graveyard. A spell copy (CR 707.10) stays for
// the finish to cease it, and the shuffle still happens. Blue Sun's Zenith
// and the other Zenith cycle. dirac/act_spelltolib.go.
const ExtSpellToLib ExtOp = 308

func init() { extOpNames[ExtSpellToLib] = "SpellToLib" }

// ExtCopyExiledWith (F, B CopyFlags, C copy exceptions override+1): for each
// card in any seat's exile zone exiled with the resolving entry's source
// (gorge CopyPermanent's Defined$ ExiledWith, exiledWithSet: seat 0 first,
// exile order), create a token that's a copy of it for the resolving
// controller. The set is read once at resolution; every source's copy count
// is sized through the CreateToken replacements first, then the copies are
// minted source by source and enter together (CR 603.6a). B and C are the
// copy riders and exceptions of OpCopyAsTokenEx; F is the exiled-with filter
// the lowering read (PredExiledWith, PredN 1). A copy whose as-enters choice
// or Aura host parks is resumed by the ordinary copy ask when no later copy
// remains, and stalls StallCopyAllAsk otherwise (dirac/copy_exiledwith.go).
const ExtCopyExiledWith ExtOp = 258

func init() { extOpNames[ExtCopyExiledWith] = "CopyExiledWith" }

// ExtExtort (CR 702.101a): an Extort trigger's body. The resolving controller
// may pay {W/B} (one mana of either colour from their pool); on a paid answer
// each opponent still in the game loses 1 life and the controller gains that
// much. The trigger itself is the printed K:Extort expansion's SpellCast line
// (or the latent trigger a layer-6 AddKeyword$ Extort grant carries), lowered
// by kgen as this op; the election is posed as the generated ChGenPayMode ask
// (mode 1 = pay, mode 0 = decline), exactly gorge effExtort's two KModes
// options. dirac/act_extort.go. Blind Obedience, Crypt Ghast, Pontiff of Blight.
// 303: the merge with land kept land's ExtRememberImprinted at 307 and took the
// next free ordinal below NumExtOps.
const ExtExtort ExtOp = 302

func init() { extOpNames[ExtExtort] = "Extort" }

// ExtRememberImprinted: each object in the resolution's imprinted set joins
// the remembered set (gorge Pump RememberObjects$ Imprinted, a body that
// rebinds the previously exiled card before the next dig; Tainted Pact).
// dirac/act_remember.go.
// 307: the merge with land kept land's ExtDrawUpto at 309 and took the free
// ordinal below it.
const ExtRememberImprinted ExtOp = 307

func init() { extOpNames[ExtRememberImprinted] = "RememberImprinted" }

// ExtPreventNext (CR 615.7): prevent the next B damage that would be dealt
// this turn to the player or permanent Obj (gorge effects/prevent_damage.go
// effPreventDamage: one this-turn DamageDone shield per recipient that
// prevents only the next N damage, depletes across events and lets the rest
// through). B is the amount, read once as the effect resolves; C 1: the shield
// matches combat damage only (CombatDamage$ True). A recipient that is not on
// the battlefield (or a player who has lost) takes no shield, and an amount of
// zero or less installs nothing. A shield is a genState.prevCol record (three a
// turn of any kind; a fourth stalls StallPrevCol). A shield that has to be
// ordered against another damage replacement (CR 616.1: a colour or scope
// prevention, a doubler, a SHIELD counter) stalls StallPrevNext: the order
// changes what is prevented; two or more shields on one recipient are asked in
// order. dirac/repl_prevnext.go.
const ExtPreventNext ExtOp = 305

func init() { extOpNames[ExtPreventNext] = "PreventNext" }
