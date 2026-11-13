package kop

// TrigFlags3 are more trigger qualifiers (Trigger.Flags3). It
// takes a padding byte of Trigger, which stays 24 bytes.
type TrigFlags3 uint8

const (
	// Tf3ConvergeBound: ETB of the card itself: the body reads Count$Converge,
	// the entering spell's colour count, copied into the pending trigger's x
	// as it triggers and read back by ExConverge while the source is still
	// that object on the battlefield (gorge reads c.Source live, and the
	// count is cleared when it leaves). Never with Tf2XBound/Tf2SpentBound
	// (they share x).
	Tf3ConvergeBound TrigFlags3 = 1 << 0
	// Tf3CastConverge: SpellCast (battlefield listener, not the spell
	// itself, not OrCopy): the trigger amount (ExTriggerAmount) is the
	// triggering spell's colour count as the trigger fires (gorge
	// TriggerConverge, TriggeredCard$Converge). Never with Tf2CastSpent or
	// Tf2CastSnapshot (they share the amount).
	Tf3CastConverge TrigFlags3 = 1 << 1
)

// TrigFlags4 are the single-target trigger qualifiers (Trigger.Flags4;
// ). Flags is full (8 bits), Flags2 bits 0-31, and Flags3
// bits 0-7, so this family appends a byte after Flags3.
type TrigFlags4 uint8

const (
	// Tf4SingleTarget: SpellCast, the triggering spell's ROOT target list has
	// exactly one target (IsSingleTarget$ True; gorge
	// rules/trigmatch/cast.go TargetShapeMatches reads obj.Targets, which
	// excludes a chained SubAbility$ link's SubTargets). A chained spell with
	// one root target and one link target still matches gorge; a copy's
	// election is not a cast and never reaches this check. Leyline of
	// Resonance.
	Tf4SingleTarget TrigFlags4 = 1 << 0
	// Tf4SingleCauseTarget: DamageDone caused by a resolving spell carrying
	// exactly one target in total (Targets plus SubTargets) that includes the
	// damaged creature (ValidCause$ Spell.numTargets EQ1 +
	// TargetRelativeToCause$ Creature.targetedBy; gorge
	// rules/trigmatch/cause_qualifiers.go damageCauseMatches,
	// stackCauseAdmits and targetCountAdmits). Outside a resolution, from an
	// ability, or from a spell with any other target count it does not
	// match. Imodane, the Pyrohammer.
	Tf4SingleCauseTarget TrigFlags4 = 1 << 1
	// Tf4OptTrigPlayer: with TfOptional, the "may" belongs to the trigger's
	// player (OptionalDecider$ TriggeredPlayer on a Phase trigger: the active
	// player, Peace in Our Time), not the controller; the resolution ask is
	// posed to that seat (gorge deciderFromSpec). Bit 4 is Tf4NamedCast
	// (Kai Winn), so this family's next free bit is 5.
	Tf4OptTrigPlayer TrigFlags4 = 1 << 5
	// Tf4PerCounter: a counter-added line (TrigCounterAddedOnce, a named
	// kind in N) lowered from a plain Mode$ CounterAdded: one trigger per
	// counter of the put batch, each carrying amount 1 (CR 603.2c; gorge
	// counterAddedMatches queues one instance per counter). Fathom Mage.
	// Bit 6 is Tf4StateCtr.
	Tf4PerCounter TrigFlags4 = 1 << 7
)

// Tf4CauseSpellAb (Trigger.Flags4, bit 3, the word's last free bit beside
// Tf4SingleCauseTarget): LifeLost / LifeLostAll, the line's ValidCause$
// names a spell or ability (gorge trigmatch/life.go lifeLossCauseMatches:
// the loss's cause is the spell or ability resolving right now, gorge's
// Facts.Damaging set; combat damage and any loss with no resolution in
// flight -- a cost payment, a mana ability -- match none). Trigger.N, when
// nonzero, carries the cause's controller qualifier:
// TrigNCauseSpellYouCtrl (the cause is controlled by the listener's
// controller, ValidCause$ SpellAbility.YouCtrl) or
// TrigNCauseSpellOppCtrl (by someone else, SpellAbility.OppCtrl); N 0 is
// any spell or ability (a bare SpellAbility$ alternative). Sahir, Visitor
// in Darkness.
const Tf4CauseSpellAb TrigFlags4 = 1 << 3

// Trigger.N of a TrigLifeLost trigger with Tf4CauseSpellAb: the ValidCause$
// controller qualifier (gorge lifeLossCauseMatches's YouCtrl / OppCtrl).
const (
	TrigNCauseSpellYouCtrl int16 = 1
	TrigNCauseSpellOppCtrl int16 = 2
)

// TrigFlags5 are the Drawn trigger's draw-step qualifiers (Trigger.Flags5).
// Flags through Flags4 carry their documented bits, so this family appends a
// byte after Flags4.
type TrigFlags5 uint8

const (
	// Tf5FirstCardInDrawStep: FirstCardInDrawStep$ True -- the draw is the
	// first card its player draws in that player's own draw step (gorge
	// trigmatch.firstCardInDrawStep; Wiretapping). CR 504.1: only the active
	// player's turn-based draw is the draw step's first card.
	Tf5FirstCardInDrawStep TrigFlags5 = 1 << 0
	// Tf5NotFirstCardInDrawStep: FirstCardInDrawStep$ False -- the draw is
	// any card except that first one (Orcish Bowmasters, Xyris, Leela).
	Tf5NotFirstCardInDrawStep TrigFlags5 = 1 << 1
)

// TrigEvent is a triggered ability's subscription: the event kind it listens
// for. The kernel keeps, per event, a bitmask of battlefield slots whose card
// has a trigger on it, so an event only scans the subscribers. Append-only.
type TrigEvent uint8

const (
	TrigNone              TrigEvent = iota
	TrigETB                         // an object matching the trigger filter enters the battlefield
	TrigDies                        // a creature matching the filter is put into a graveyard from the battlefield
	TrigLeaves                      // a permanent matching the filter leaves the battlefield (any destination)
	TrigToGraveyard                 // a card matching the filter is put into a graveyard from anywhere
	TrigAttacks                     // a creature matching the filter attacks
	TrigAttackersDeclared           // the trigger controller's attackers are declared (one or more attacked)
	TrigPhase                       // the start of step Trigger.Step (whose: Trigger.Whose)
	TrigSpellCast                   // a spell matching the filter is cast (Whose: you/any/opponent)
	TrigLifeGained                  // a player (Whose) gains life
	TrigLifeLost                    // a player (Whose) loses life
	TrigDamageDone                  // a source matching the filter deals damage (Trigger.Flags: combat / to-player)
	TrigDamageDealtOnce             // a batch of combat damage is dealt by creatures matching the filter (once per batch)
	TrigBecomesTarget               // a permanent matching the filter becomes the target of a spell/ability (Ward)
	TrigDrawn                       // a player (Whose) draws a card (Trigger.Flags: second-card-this-turn)
	TrigDiscarded                   // a card matching the filter is discarded
	TrigCounterAddedOnce            // one or more counters are put on a permanent matching the filter (once per batch)
	TrigUntaps                      // a permanent matching the filter becomes untapped
	TrigBlocks                      // a creature matching the filter blocks
	TrigBecomesBlocked              // a creature matching the filter becomes blocked
	TrigSacrificed                  // a permanent matching the filter is sacrificed (cost or effect; fired as it leaves)
	TrigManaExpend                  // a player (Whose) spends their Trigger.N-th total mana on spells this turn (expend N, CR 700.14)
	TrigTaps                        // a permanent matching the filter becomes tapped (not as it enters tapped; trigger player: the tapper)
	TrigGiveGift                    // a player (Whose) gives a gift (CR 702.174b; no slot subscription: the kernel scans the battlefield when a gift is given)
	TrigProliferate                 // a player (Whose) proliferates (CR 701.34; gorge's events.Proliferate marker, once per completed proliferate action; no slot subscription)
	TrigTokenCreated                // a token matching the filter is created (gorge Mode$ TokenCreatedOnce, events.TokenCreate; with Tf2Batch once per resolution; no slot subscription)
	TrigBecomeMonstrous             // a permanent matching the filter becomes monstrous (gorge Mode$ BecomeMonstrous; no slot subscription: the kernel scans the battlefield)
	TrigTurnFaceUp                  // a permanent matching the filter is turned face up (gorge Mode$ TurnFaceUp; no slot subscription: the kernel scans the battlefield)
	TrigTransformed                 // a permanent matching the filter transforms (gorge Mode$ Transformed; no slot subscription: the kernel scans the battlefield)
	TrigCommitCrime                 // a player (Whose) commits a crime (CR 700.13; gorge Mode$ CommitCrime, trigmatch commitCrimeMatches); no slot subscription: the battlefield, then graveyard cards, are scanned (act_commitcrime.go)
	TrigUnlockDoor                  // a Room door of this permanent is unlocked, or the cast door as the Room spell enters (CR 709.5h; gorge Mode$ UnlockDoor, rules face_rooms.go); no slot subscription: fired on the Room itself (face_rooms.go)
	TrigBecomesPlotted              // this card becomes plotted (CR 702.170a; gorge Mode$ BecomesPlotted, ValidCard$ Card.Self from exile): no slot subscription, fired by the plot action itself (kw_plot.go)
	TrigBecomesSaddled              // this Mount becomes saddled (CR 702.171b; gorge Mode$ BecomesSaddled, ValidSaddled$ Card.Self; Tf2FirstTime = FirstTimeSaddled$): no slot subscription, fired by the saddle op (trig_saddled.go)
	TrigAttached                    // this Aura/Equipment becomes attached to a creature (CR 701.3a; gorge Mode$ Attached, ValidSource$ Card.Self, ValidTarget$ Creature): no slot subscription, fired by attachTo on the attachment's own abilities; RefTriggerObj is the bearer (zone_exilethenattach.go, Assimilation Aegis)
	TrigFullyUnlock                 // a Room becomes fully unlocked (CR 709.5i; gorge Mode$ FullyUnlock, trigmatch fullyUnlockMatches): no slot subscription, scanned by the unlock action (face_fullyunlock.go)
	TrigDiscardedAll                // one or more cards matching the filter are discarded by a player (Whose), once per discard action (CR 603.2c; gorge Mode$ DiscardedAll, trigger_match.go discard batch); amt = cards in the batch; no slot subscription (trig_discardedall.go)
	TrigAbilityCast                 // an activated non-mana ability of class Trigger.N (TrigAbCastExhaust / Loyalty / LoyaltyOpp) is activated by Whose (CR 602.2, 603.2; gorge Mode$ AbilityCast, trigmatch abilityCastMatches); no slot subscription: the battlefield, then graveyard cards, are scanned as the activation completes (trig_abilitycast.go)
	TrigScry                        // a player (Whose) scries (CR 701.22; gorge Mode$ Scry, trigmatch scryMatches on the completed events.Scry record, also for scry 0 / an empty library); no slot subscription: the battlefield is scanned as for TrigProliferate (trig_scrysurveil.go)
	TrigSurveil                     // a player (Whose) surveils (CR 701.25; gorge Mode$ Surveil, trigmatch surveilMatches on the events.Surveil marker emitted before the look, also for surveil 0); no slot subscription: the battlefield is scanned as for TrigProliferate (trig_scrysurveil.go)
	TrigExplores                    // a creature matching Filter explores (CR 701.44a; gorge Mode$ Explores, trigmatch exploresMatches on the completed events.Explore record: Filter = the explorer, Filter2 = the revealed card, type predicates only); no slot subscription: the battlefield is scanned after the outcome (trig_exploreevidence.go)
	TrigCollectEvidence             // a player (Whose) collects evidence (CR 701.59a; gorge Mode$ CollectEvidence, trigmatch collectEvidenceMatches on events.CollectEvidenceAction, emitted by a cost payment that exiled a card, not by a ward payment); no slot subscription: the battlefield is scanned (trig_exploreevidence.go)
	TrigLandPlayed                  // a land matching the filter is played (CR 305.1; gorge Mode$ LandPlayed, trigmatch landPlayedMatches: the land's move to the battlefield while it is a land play, a Play effect's too); Trigger.N is the origin zone mask; no slot subscription: the entry walk scans the battlefield (trig_modes.go)
	TrigSearchedLibrary             // a player (Whose) searches their own library (gorge Mode$ SearchedLibrary, SearchOwnLibrary$ True, trigmatch cards.go: one marker per hidden library search, also when nothing is found); no slot subscription (trig_modes.go)
	TrigForage                      // a player (Whose) forages (gorge Mode$ Forage, trigmatch ForageAction: once per paid forage, after its moves); no slot subscription (trig_modes.go)
	TrigDiscover                    // a player (Whose) discovers (gorge Mode$ Discover, trigmatch cards.go: once per discover, at completion); the trigger amount is the discover value; no slot subscription (trig_modes.go)
	TrigManifestDread               // a player (Whose) manifests dread (gorge Mode$ ManifestDread, trigmatch keyword_action.go on the completed events.ManifestDreadAction marker, emitted after each window's moves; the trigger object is the card the window put into its owner's graveyard, or none on the one-card shortcut); no slot subscription (trig_modes.go)
	TrigFacesDilemma                // a player (Whose) chooses one or more modes for a spell or ability (gorge Mode$ FacesDilemma, trigmatch facesDilemmaMatches on the ModeChosen marker)
	TrigEvolved                     // this creature evolves (CR 702.100b; gorge Mode$ Evolved, ValidCard$ Card.Self, trigmatch evolvedMatches on the events.Evolved marker its evolve ability emits once the +1/+1 counter lands): no slot subscription, fired as the evolve trigger finishes resolving (kw_evolve.go)
	TrigBecomeMonarch               // a player (Whose) becomes the monarch (CR 725.3; gorge Mode$ BecomeMonarch, trigmatch BecomeMonarchMatches on events.MonarchChange; N TrigNMonarchBeganTurn: BeginTurn$ You); no slot subscription: the battlefield is scanned (act_monarch.go)
	TrigEventCount                  // number of trigger events; append above
)

// Whose restricts a trigger to an event about a player relative to the
// controller.
type Whose uint8

const (
	WhoseAny Whose = iota
	WhoseYou
	WhoseOpp
	WhoseEnchCtrl // Phase (battlefield listener only): the controller of the permanent the source is attached to (gorge Player.EnchantedController)
)

// Step names a turn step (the kernel's step numbering; TrigPhase uses it).
type Step uint8

const (
	StepUntap Step = iota
	StepUpkeep
	StepDraw
	StepMain1
	StepBeginCombat
	StepAttackers
	StepBlockers
	StepFirstStrikeDamage
	StepDamage
	StepEndCombat
	StepMain2
	StepEnd
	StepCleanup
	NumSteps
)

// TrigFlags qualify a trigger.
type TrigFlags uint8

const (
	TfOptional    TrigFlags = 1 << iota // "you may": the controller is asked on resolution
	TfSelf                              // the event object must be the source itself
	TfCombat                            // DamageDone: combat damage only
	TfToPlayer                          // DamageDone: to a player only
	TfOncePerTurn                       // triggers only once each turn
	TfSecondDraw                        // Drawn: only your second card each turn (the N-th when Trigger.N > 0)
	TfNontokenSrc                       // reserved
	TfNotToGrave                        // Leaves: only to a zone other than a graveyard; always with Tf2Batch (ChangesZoneAll "without dying", kernel trig_oneormore.go)
)

// Trigger is a triggered ability's subscription.
type Trigger struct {
	Event  TrigEvent
	Filter FilterID // the event object must match (ignored with TfSelf)
	Whose  Whose    // for player events and TrigPhase
	Step   Step     // TrigPhase
	Flags  TrigFlags
	From   Zone // TrigLeaves / TrigToGraveyard origin restriction (ZoneNone = any)

	// Appended by dirac/kgen.
	Filter2 FilterID   // DamageDone: the damaged object/player must match; AttackersDeclared: the attackers
	Flags2  TrigFlags2 // more qualifiers
	Cond    CondID     // intervening-if (0 = none): checked when the event happens and on resolution
	Zone    Zone       // the zone the source must be in (ZoneNone = battlefield)
	N       int16      // AttackersDeclared: minimum attackers; CounterAddedOnce: the counter kind (TrigNAnyCounter = every kind); BecomesTarget: TrigNValiant; AbilityCast: TrigAbCast*; LandPlayed: origin mask 1<<kop.Zone, 0 = any; Leaves: destination mask 1<<kop.Zone, 0 = any
	Flags3  TrigFlags3 // still more qualifiers (values.go; a padding byte)
	Flags4  TrigFlags4 // single-target qualifiers (triggers.go)
	Flags5  TrigFlags5 // Drawn: the draw-step position (FirstCardInDrawStep$)

	// TapPlan classes a lowered non-Self TrigTaps watcher for the payment
	// planner (TapPlan); TapPainN is the vetted literal damage amount of a
	// TapPlanPain watcher. Both are read only on a TrigTaps trigger.
	TapPlan  TapPlan
	TapPainN uint8
}

// TapPlan is a lowered Taps watcher's payment-planner class (gorge
// pay.PaymentPlanAbilityTier over the source a planned tap would tap).
type TapPlan uint8

const (
	TapPlanNone     TapPlan = iota // not a lowered non-Self Taps watcher
	TapPlanOrdinary                // vetted damage that cannot reach the payer: the tapped source stays an ordinary unit
	TapPlanPain                    // vetted damage to the payer (Trigger.TapPainN): the tapped source is a last-resort unit
	TapPlanDefer                   // any other body: the planner never taps the source (manual only)
)

// TrigNValiant in Trigger.N of a TrigBecomesTarget trigger (TfSelf, Whose
// WhoseYou) is Valiant (CR 603.2 ability word): only the first targeting
// this turn by a spell or ability the listener's controller controls fires
// it (gorge trigmatch.valiantFirstTimeTargetedByYouThisTurn).
const TrigNValiant int16 = 1

// Trigger.N of a TrigAbilityCast trigger: the activated ability's class
// (gorge abilityCastConstraintHolds). The Opp form also requires the
// ability's controller to be an opponent of the listener's controller.
const (
	TrigAbCastExhaust    int16 = 1 // Activated.Exhaust
	TrigAbCastLoyalty    int16 = 2 // Activated.Loyalty
	TrigAbCastLoyaltyOpp int16 = 3 // Activated.Loyalty+OppCtrl
)

// TokenRider flags (OpCreateToken's B.N).
const (
	TrTapped    = 1 << iota // the token enters tapped
	TrAttacking             // the token enters tapped and attacking (reserved)
	TrHasteEOT              // the token gains haste until end of turn
	TrList                  // one of a comma-listed TokenScript$'s tokens: a Token resolution's one creation event spans the ops, which a ReplAddToken cannot claim once across (stall)
)

// DamageFlags (OpDealDamageEx C).
const (
	DmgTrackExcess         = 1 << iota // record excess damage dealt to the creature (ExExcessDamage)
	DmgExileIfDiesThisTurn             // if the creature would die this turn, exile it instead
	DmgMapped                          // part of a damage map ended by OpResolveDamageMap
	DmgRemember                        // an object dealt the damage on the battlefield joins the remembered set, only when damage above 0 was dealt after prevention and replacement (RememberDamaged$, gorge; a player is not stored)
	DmgExileIfDealt                    // with DmgExileIfDiesThisTurn: the exile-instead applies only when damage above 0 was dealt to the object (ReplaceDyingDefined$ Remembered, gorge; Targeted covers every target)
	DmgNoRegen                         // a creature dealt damage above 0 can't be regenerated this turn (Incinerate's follow-up: the arena's mechNoRegen mark, cleared at the turn's cleanup and when the slot takes a new permanent)
	DmgDivideEvenly                    // OpDealDamageDivided only: no division is chosen; A is divided by the slot's legal targets at resolution, rounded down, each taking the quotient (Fireball's DivideEvenly$ RoundedDown; gorge divideEvenlyDown)
	DmgNoRegenTarget                   // OpDealDamageEx: the target object can't be regenerated this turn once the damage step ran, whether or not damage above 0 was dealt (Rage of Purphoros' ParentTarget follow-up; same arena mark as DmgNoRegen)
)

// TrigFlags2 (Trigger.Flags2).
type TrigFlags2 uint32

const (
	Tf2FirstTime      TrigFlags2 = 1 << iota // the first time each turn
	Tf2YourTurn                              // only during the controller's turn
	Tf2OppTurn                               // only during an opponent's turn
	Tf2NonCombat                             // DamageDone: noncombat damage only
	Tf2ToOpponent                            // DamageDone: to an opponent only
	Tf2SelfOrFilter                          // the source itself OR an object matching Filter
	Tf2SecondDraw                            // Drawn: the player's second card this turn
	Tf2CastSnapshot                          // SpellCast: the trigger amount is the instants and sorceries the controller cast this turn, the triggering spell included, counted when it triggers
	Tf2NthSpell                              // SpellCast: only the caster's N-th spell this turn (gorge ActivatorThisTurnCast$ EQN)
	Tf2BlockedBySelf                         // BecomesBlocked: only an attacker this permanent blocks (gorge ValidBlocker$ Card.Self)
	Tf2TargetsFilter2                        // SpellCast: the spell has a target and some target of its chain matches Filter2 (a battlefield object, a player alternative, or a ZoneAny zone-free object alternative: gorge TargetsValid$)
	Tf2Alone                                 // Attacks: only when exactly one creature was declared as an attacker (gorge Alone$ True, Exalted)
	Tf2AtkYou                                // Attacks with Attacked$: the attacked defender may be the trigger's controller
	Tf2AtkYourPW                             // ... or a planeswalker the trigger's controller controls
	Tf2AtkOpp                                // ... or an opponent of the trigger's controller (a player)
	Tf2Batch                                 // a "one or more" line (ChangesZoneAll on ETB/Dies/Leaves, DamageDoneOnce on DamageDone): queued once per simultaneous batch (kernel trig_oneormore.go)
	Tf2PerPair                               // BecomesBlocked: once per declared (attacker, blocker) pair, Filter on the attacker, Filter2 on the blocker; the trigger object is the pair member that is not the listener (gorge Mode$ AttackerBlockedByCreature)
	Tf2Cycled                                // Discarded, TfSelf, Zone graveyard: "when you cycle this card", fired by the cycling cost's discard (gorge Mode$ Cycled)
	Tf2Unblocked                             // BecomesBlocked: instead, once per attacker matching Filter that is unblocked as the declare blockers step's declarations complete (gorge Mode$ AttackerUnblocked)
	Tf2OrCopy                                // SpellCast: a spell copy put on the stack fires it too (gorge Mode$ SpellCastOrCopy)
	Tf2DefPlayer                             // AttackersDeclared: per defender, the player only (gorge AttackedTarget$ Player / Opponent, Mode$ AttackersDeclaredOneTarget): fires on the player-defender group, counting its attackers matching Filter; with Tf2AtkOpp the defender must be an opponent of the controller
	Tf2XBound                                // ETB / SpellCast: the body reads the X paid for the trigger object's spell, bound as it triggers (CR 107.3m; gorge TriggerContext.TriggerPaidX; kernel trig_xpaid.go)
)

// Tf2ResolvedLimit: ResolvedLimit$ 1 ("do this only once each
// turn"; gorge triggerTurnResolved): the line does not trigger once its
// source's ResolvedLimit$ lines resolved this turn (the count is per source
// object, shared by its limited lines; a mandatory line counts as it
// resolves, an optional one when accepted). Bit 22, beside the iota run.
const Tf2ResolvedLimit TrigFlags2 = 1 << 22

// Tf2CastSpent (opus): SpellCast: the trigger amount (ExTriggerAmount) is
// the triggering spell's pool mana spent, snapshotted as the trigger fires
// (gorge TriggeredCard$CastTotalManaSpent, TriggerManaSpent). Bit 23.
const Tf2CastSpent TrigFlags2 = 1 << 23

// Tf2ToYou: DamageDone with TfToPlayer: only damage to the
// listener's controller (gorge ValidTarget$ You). Bit 24.
const Tf2ToYou TrigFlags2 = 1 << 24

// Tf2PerReferent: DamageDone lowered from Mode$
// DamageDoneOnce: queued once per damaged referent per damage batch, at the
// referent's first matching event, carrying the batch total of the matching
// events to that referent (gorge trigger_match.go damageBatchKey). The
// object referent is queued as the trigger object (its controller the
// trigger player); a player referent queues the first source as the
// trigger object and the damaged player. Trigger.N, when nonzero, is the
// DamageAmount$ gate (DmgGate). Bit 25.
const Tf2PerReferent TrigFlags2 = 1 << 25

// Tf2SpentBound: ETB of the card itself: the body reads
// Count$CastTotalManaSpent, the mana spent to cast the entering spell (CR
// 601.2h), copied into the pending trigger's x as it triggers (bindTrigX)
// and read back by ExCastManaSpent. An object that entered uncast binds 0.
// Bit 27.
const Tf2SpentBound TrigFlags2 = 1 << 27

// Tf2SrcInPlay: DamageDone / DamageDealtOnce: the line's
// ValidSource$ demands inZoneBattlefield (gorge wordInZone: the source's
// live zone is the battlefield), so damage from a spell, or from an ability
// whose source left play, never fires it. Bit 26.
const Tf2SrcInPlay TrigFlags2 = 1 << 26

// Tf2CastFromHand: gorge wasCastFromYourHandByYou on the
// trigger object. SpellCast: the spell was cast from its caster's hand
// (Jodah, the Unifier); ETB with TfSelf: the source entered as the resolving
// spell its controller cast from their hand (Soulhunter Rakshasa).
const Tf2CastFromHand TrigFlags2 = 1 << 29

// Tf2Convoked: an ETB TfSelf trigger whose amount
// (ExTriggerAmount) is the number of creatures that convoked the source's
// spell, bound as it triggers (gorge Convoked$Amount, Knight-Errant of Eos).
const Tf2Convoked TrigFlags2 = 1 << 28

// DmgCmp is a DamageAmount$ comparator.
type DmgCmp uint8

const (
	DmgGE DmgCmp = 1 + iota
	DmgLE
	DmgEQ
	DmgNE
	DmgGT
	DmgLT
)

// DmgGate encodes a DamageAmount$ <cmp><literal> gate for Trigger.N.
func DmgGate(cmp DmgCmp, lit int) int16 { return int16(cmp)<<8 | int16(lit&0xff) }

// DmgGatePass reports whether total satisfies the gate n (0 = no gate).
func DmgGatePass(n int16, total int) bool {
	if n == 0 {
		return true
	}
	lit := int(n & 0xff)
	switch DmgCmp(n >> 8) {
	case DmgGE:
		return total >= lit
	case DmgLE:
		return total <= lit
	case DmgEQ:
		return total == lit
	case DmgNE:
		return total != lit
	case DmgGT:
		return total > lit
	case DmgLT:
		return total < lit
	}
	return false
}

// Tf2Attacked is the Attacked$ defender set (gorge attackedDefenderMatches):
// with any of these bits the Attacks trigger fires only for attackers whose
// defender is in the set.
const Tf2Attacked = Tf2AtkYou | Tf2AtkYourPW | Tf2AtkOpp

// Tf3FirstAttack: an Attacks trigger that fires only for an attacker that has
// not been declared as an attacker earlier this turn (gorge FirstAttack$ True:
// the object's AttacksThisTurn == 1 at the fire).
const Tf3FirstAttack TrigFlags3 = 1 << 7

// Tf2SpellToo (Trigger.Flags2): BecomesTarget watcher --
// besides the battlefield objects its Filter names, a creature spell its
// controller controls on the stack counts as the targeted object (gorge
// ValidTarget$ alternative Spell.Creature+YouCtrl: Surrak, Elusive Hunter).
const Tf2SpellToo TrigFlags2 = 1 << 27

// Tf4CounterCell (Trigger.Flags4): a CounterAddedOnce line lowered from
// Mode$ CounterPlayerAddedAll. Forge's GameEntityCounterTable fires that mode
// once per (putter, recipient) cell, so gorge merges the CounterChange events
// of one put action into one trigger (rules' counterBatch latch). Mode$
// CounterAddedOnce keeps one trigger per event and has no bit.
const Tf4CounterCell TrigFlags4 = 1 << 2

// Tf4NthFiltered (Trigger.Flags4): a Tf2NthSpell line whose ValidCard$ turns some
// spells away counts the caster's pushes that match Filter (the cast ring), not
// every spell they cast this turn. Filter names no zone: an earlier push may have
// left the stack.
const Tf4NthFiltered TrigFlags4 = 1 << 3

// Tf4NamedCast (Trigger.Flags4): a SpellCast line lowered from Mode$
// SpellAbilityCast over the source's chosen card name (ValidCard$
// Card.NamedCard, ValidSA$ Spell,Activated, ValidActivatingPlayer$
// Opponent; Kai Winn, Faithless Leader). It fires when a spell with the
// source's chosen name is cast (the SpellCast arm, castNamedOK) and when an
// ability of a source with that name is activated (the AbilityCast arm,
// fireAbilityCast); the chosen name is the permanent's chosen byte, a CardID.
const Tf4NamedCast TrigFlags4 = 1 << 4

// Tf4StateCtr (Trigger.Flags4): a CR 603.8 state trigger (Mode$ Always)
// lowered over a counter threshold (Mazemind Tome's IsPresent$
// Card.Self+counters_GE4_PAGE). It rides the counter-put event of any kind,
// but one instance at a time: a put while an instance of the same source and
// ability is queued or on the stack does not queue another
// (stateTrigOutstanding), and the instance's leave re-checks the threshold
// (stateCtrRefire, trig_statectr.go). The threshold itself is the trigger's
// Cond, and AfCheckOnTrigger marks it not re-checked on resolution (gorge
// noResolvingCheck exempts Mode$ Always).
const Tf4StateCtr TrigFlags4 = 1 << 6

// the packed constants of the Increment keyword's ops.

// Tf2Increment (Trigger.Flags2): SpellCast -- "whenever
// you cast a spell, if the amount of mana you spent is greater than this
// creature's power or toughness, put a +1/+1 counter on this creature"
// (CR 702.xx, Secrets of Strixhaven). The event-relative comparison rides
// gorge kwIncrement's Increment$ marker, read at fire time by trigmatch.
// incrementAdmits against the SOURCE's layer-output P/T and again as the
// trigger resolves (rules.triggerResolvingCheckHolds, trigmatch.IncrementHolds)
// against the captured spend. The kernel copies both halves in
// State.incrementAdmits / State.incrementResolves (dirac/kw_increment.go),
// the spend read from the spell entry's recorded spend (recordSpent) and
// carried on the trigger entry's trigAmt.
//
// Bit 24, shared with Tf2ToYou: Tf2ToYou is set and read
// only on DamageDone / DamageDealtOnce lines (dirac/kgen/build_ab.go's F2
// block, kernel gen.go's anyDmgRef, trig_damagedonce.go) and no SpellCast
// reader tests the bit, so the two event-disjoint flags cannot collide --
// the Tf2SpellToo / Tf2SpentBound precedent. Tf2Increment has no
// dirac/kgen/names.go entry (one name per shared bit), so a generated literal
// for the bit prints the first registered name, kop.Tf2ToYou; the value is
// identical and the SpellCast readers read it as Increment.
const Tf2Increment TrigFlags2 = 1 << 24

// WhoseNonActive: a SpellCast trigger whose
// ValidActivatingPlayer$ is Player.NonActive: the caster is not the active
// player (gorge playerSpecQualifierNonActive, player_filter.go: p != g.Active,
// evaluated on the caster). An explicit value, not an iota step: kgen's
// whoNonActive (dirac/kgen/lower_castnonactive.go) must equal it.
const WhoseNonActive Whose = 200

// Tf2ManaFromSelf (Trigger.Flags2): SpellCast, ValidSA$
// Spell.ManaFromCard.StrictlySelf -- the cast's payment tapped the
// trigger's own source for mana (gorge trigmatch validSAManaFromCardSelf:
// the source is in Object.ManaSpentSources; Barracks of the Thousand,
// Tecutlan, the Searing Rift). Read at the cast (kernel
// mana_spendtrigger.go). The last bit of the word.
const Tf2ManaFromSelf TrigFlags2 = 1 << 31

// "if you cast a spell this way, it enters with a
// counter" — the cast's MayPlaySource provenance (FlagMayPlay in gorge)
// and the Static$ True SpellCast triggers whose Effect upgrades the cast
// spell's entry (kernel trig_mayplaystatic.go).

// Tf3Static (Trigger.Flags3, bit 4): a SpellCast trigger whose line
// carries Static$ True. gorge resolves such a trigger off the stack, by
// the trigger drain, before the CR 603.3b ordering ask
// (engine/rules/static_trigger.go): no priority pass, no order choice.
// Only a body kgen PROVES ask-free carries the flag (no targets, no Charm,
// no Optional/Unless/Cost rider — the exact rider Effect shapes only);
// everything else refuses in kgen and keeps the stack path. TrigFlags2 is
// full, so it rides Flags3 beside the converge and validsa qualifiers.
const Tf3Static TrigFlags3 = 1 << 4

// Tf3MayPlaySrc (Trigger.Flags3, bit 5): a SpellCast trigger's ValidSA$
// Spell.MayPlaySource — the cast went through a may-play permission, read
// off the spell's stack entry as it triggers (gorge ValidSAMatches reads
// the cast's FlagMayPlay: the flag-only approximation of Forge's
// host-specific MayPlaySource, which records no host; a stack copy never
// carries it). Bit 5, beside Tf3Static (bits 2-3 are triggers.go).
const Tf3MayPlaySrc TrigFlags3 = 1 << 5

// TrigNTraining in Trigger.N of a TrigAttacks trigger (TfSelf) is Training
// (CR 702.149a): it fires only when the source attacks with
// another creature of strictly greater power (gorge trigmatch AttacksMatches
// over the whole declaration, derived power; Hopeful Initiate).
const TrigNTraining int16 = 2

// TrigNTeamworkTap in Trigger.N of a TrigTaps trigger (TfSelf): only a tap
// paying a teamwork cost (gorge trigmatch/tap.go:41).
const TrigNTeamworkTap int16 = 1

// Tf3GameOnce: the line triggers once per source object (GameActivationLimit$ 1).
const Tf3GameOnce TrigFlags3 = 1 << 6

// Tf2Evolve (Trigger.Flags2): ETB, Condition$ Evolve or K:Evolve's Evolve$
// True -- the entering creature's power or toughness is greater than the
// source's, read as the trigger fires and again as it resolves (CR 702.100a,
// 603.4; gorge evolveConditionHolds, evolveResolves; dirac/kw_evolve.go).
// A high bit, clear of the siblings' appended ones.
const Tf2Evolve TrigFlags2 = 1 << 30

// two SpellCast ValidSA$ heads (gorge trigmatch/cast.go
// ValidSAMatches, :731). They extend TrigFlags3 (values.go holds bits 0-1).
const (
	// Tf3ManaFromTreasure: SpellCast, ValidSA$ Spell.ManaFromTreasure: the
	// cast's payment spent mana a Treasure produced (gorge answers it as
	// Card.CastSa Spell.ManaFromTreasure through CastProvenanceAdmits,
	// cast.go:762). Smaug, Wicked Worm.
	Tf3ManaFromTreasure TrigFlags3 = 1 << 2
	// Tf3OrTargets: SpellCast, ValidSA$ "Spell.X,Spell.IsTargeting Valid Y":
	// the cast spell matches Filter OR some target of it matches Filter2
	// (gorge validSATargetingList, cast.go:731-789; spellIsTargetingMatchesPtr
	// tries every target). Danitha, Sword of Hope.
	Tf3OrTargets TrigFlags3 = 1 << 3
)

// Tf4DefEnch (Trigger.Flags4, bit 7, shared with Tf4PerCounter: that flag is
// read only on a TrigCounterAddedOnce line and this one only on an
// AttackersDeclared line, so no trigger carries both; the Tf2ToYou /
// Tf2Increment bit-24 precedent): Mode$
// AttackersDeclared with AttackedTarget$ Player.EnchantedBy, the Commander
// curses' "whenever enchanted player is attacked" (Curse of Opulence,
// Disturbance, Verbosity, Bounty, Inertia, Vitality; Archnemesis). Beside
// Tf2DefPlayer, the per-defender group fires only when the DEFENDER is
// enchanted by THIS listener's Aura (gorge attackedDefenderMatches reads
// Player.EnchantedBy, the source-relative player property player_filter.go
// playerSpecQualifierEnchantedBy: player.isEnchantedBy(source)), through
// fireDefPlayer's Tf4DefEnch arm.
const Tf4DefEnch TrigFlags4 = 1 << 7

// TrigNAnyCounter in Trigger.N of a TrigCounterAddedOnce trigger: every
// counter kind (CounterPlayerAddedAll without CounterType$).
const TrigNAnyCounter int16 = -1

// Tf2LeavesGrave (Trigger.Flags2, bit 17, shared with Tf2Cycled whose
// readers all test a Discarded line): a Leaves line with a destination mask
// (Trigger.N) that also fires for a card leaving a graveyard to a zone in the
// mask, once per batch with the battlefield leaves (gorge Mode$ ChangesZoneAll
// Origin$ Battlefield,Graveyard, ZoneChangeMatches' Origin$ set; Ketramose,
// the New Dawn). The graveyard side is fired where the card is placed
// (kernel trig_leavesgrave.go).
const Tf2LeavesGrave = Tf2Cycled
