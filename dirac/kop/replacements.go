package kop

// ReplDamagePlus recipient bits (Repl.C), relative to the replacement's
// controller; ReplDmgNonCombat restricts it to noncombat damage.
const (
	ReplDmgOppPlayer = 1 << iota // an opponent
	ReplDmgOppPerm               // a permanent an opponent controls
	ReplDmgPlayer                // any player
	ReplDmgPerm                  // any permanent
	ReplDmgNonCombat             // noncombat damage only (IsCombat$ False)
	ReplDmgRaise                 // raise instead of add: damage less than the host's power becomes the host's power (Ojer Axonil; Repl.A unused)
	ReplDmgSrcInPlay             // ReplDoubleDamageToOpps: the source must be a permanent (ValidSource$ inZoneBattlefield)
	ReplDmgCtrHost               // ReplDamagePlus: Repl.A is a counter kind; the amount added is the host's count of that kind at application time (Fated Firepower)
	ReplDmgPowHost               // ReplDamagePlus: the amount added is the host's power at application time (Repl.A unused; Hawkeye, Young Avenger)
)

// ReplKind is a replacement-lite effect's kind (the kernel's narrow form of
// CR 614: one fixed rewrite per kind, no ordering choice). Append-only.
type ReplKind uint8

const (
	ReplNone               ReplKind = iota
	ReplEntersTapped                // this permanent enters tapped (C=1: unless condition A)
	ReplEntersWithCounters          // this permanent enters with A counters of kind C
	ReplCantBeCountered             // this spell can't be countered
	ReplDoesntUntap                 // this permanent doesn't untap during its controller's untap step
	ReplEntersAsChoice              // as this enters, its controller chooses (C: colour/type/player); stored on the permanent

	// --- appended by dirac/kgen ---
	ReplEntersProgram            // as this enters, run the ability's Prog (EnterTapped / EnterWithCounters / ChooseColor / ChooseType)
	ReplOthersEnterProgram       // as another object matching Repl.Filter enters, run the ability's Prog on it (RefReplaced)
	ReplCantLoseGame             // the controller can't lose the game
	ReplOppsCantWinGame          // the controller's opponents can't win the game
	ReplDoubleDamageToOpps       // a source the controller controls deals double damage to an opponent or a permanent an opponent controls (Twinflame Tyrant)
	ReplPreventNoncombatToOthers // noncombat damage to the controller's other creatures is prevented (Crystal Barricade)
	ReplDoubleTokens             // an effect creates twice as many tokens under the controller's control (Doubling Season)
	ReplDoubleCounters           // an effect puts twice as many counters on a permanent the controller controls; costs excluded (Doubling Season)
	ReplGraveToLibrary           // this card put into a graveyard from anywhere is shuffled into its owner's library instead (Progenitus)
	ReplOthersDoesntUntap        // a permanent matching Repl.Filter (this source's controller and source) doesn't untap during its controller's untap step (gorge cards.ReplUntap, Layer$ CantHappen with ValidStepTurnToController$; the any-event form is ReplAnyUntap)
	ReplDamagePlus               // damage a source matching Repl.Filter would deal to a recipient in Repl.C (ReplDmg* bits) is that much plus Repl.A instead (Valley Flamecaller; ReplaceCount$DamageAmount/Plus.N)
	ReplLifeDouble               // the controller would gain life: twice that much instead (Boon Reflection; ReplaceCount$LifeGained/Twice)
	ReplLifePlus                 // the controller would gain life: that much plus Repl.A instead (Heron of Hope; ReplaceCount$LifeGained/Plus.N)
	ReplExileOnLeave             // a permanent matching Repl.Filter put into a graveyard from the battlefield is exiled instead
	ReplDoubleCountersOn         // an effect puts twice as many counters of kind Repl.C (or of every kind: ReplCtrAny, replacements.go) on a permanent matching Repl.Filter (Branching Evolution; Loading Zone)
	ReplAddCountersOn            // an effect puts that many plus Repl.A counters of kind Repl.C (or of every kind: ReplCtrAny, or ReplCtrAnyYou for the controller's own placements only) on a permanent matching Repl.Filter (Hardened Scales; Doc Samson)
	ReplOnePoison                // the controller would get poison counters: they get one instead and can't get more this turn (Melira, the Living Cure)
	ReplDoubleCountersYou        // the controller would put counters on a permanent or player: twice that many instead (the putter: gorge's in-flight counter adder; Innkeeper's Talent level 3, gated by Repl.Cond)
	ReplLifeLossDouble           // an opponent of the controller would lose life during the controller's turn: twice that much instead, damage's life loss and life costs included (Bloodletter of Aclazotz; gorge LifeReduced ReplaceCount$Amount/Twice)
	ReplTurnUpProgram            // as this face-down permanent is turned face up, run the ability's Prog on it before it turns up (PutCounter on RefReplaced; gorge applyTurnFaceUpReplacements, replacement_phase.go; Bubble Smuggler)
	ReplCantTurnUp               // a permanent matching Repl.Filter (read in this source's frame) can't be turned face up while Repl.Cond holds: the special action is never offered (gorge turnFaceUpCantHappen, replacement_phase.go; Karlov Watchdog, Unable to Scream; CR 614.17a)
	ReplSpellsCantBeCountered    // a spell its controller (this source's controller) casts that matches Repl.Filter can't be countered while this source is on the battlefield (gorge counterReplacementMatchesAll, Layer$ CantHappen with ValidSA$ Spell[.types]+YouCtrl; Chimil, the Inner Sun, Sphinx of the Final Word)
	ReplAnyUntap                 // a permanent matching Repl.Filter (read in this source's frame) can't become untapped at any time: every Untap event is replaced, in any untap step and by an effect or cost (gorge cards.ReplUntap with no ValidStepTurnToController$, rules/replacement_match.go:338; CR 614.17; Frozen in Ice, Blossombind). The untap step's ReplOthersDoesntUntap gates do not cover it
	ReplDrawMore                 // the controller would draw a card: Repl.A cards instead, each a new draw this replacement does not meet again (CR 614.5); Repl.C&ReplDrawNotFirst exempts the first card the controller draws in their draw step (gorge applyReplacements, a ReplaceWith$ DB$ Draw body)
	ReplRewriteTokens            // a token the controller would create is the token of pool token index Repl.C instead, the same number (R:Event$ CreateToken | ReplaceWith$ DB$ ReplaceToken | Type$ ReplaceToken | TokenScript$; Holo-Engineer). The matchup remaps Repl.C to its CardID
	ReplMayNotUntap              // this permanent's controller may choose not to untap it during their untap step: a tapped one asks "untap or keep tapped" (CR 502.3; gorge hasUntapStepChoice, the keyword line "You may choose not to untap CARDNAME during your untap step."); no other field
	ReplDestroyHeal              // a creature matching Repl.Filter would be destroyed: remove all +1/+1 counters from it and heal all damage on it instead (Dr. McCoy, Surly Surgeon; gorge's Destroy ReplaceWith$ RemoveCounter body)
	ReplDiscardOppToBattlefield  // this card in hand discarded by a spell or ability an opponent of its owner controls (not a cost, not cleanup) enters the battlefield under its owner's control instead of the graveyard (Wilt-Leaf Liege; gorge replacement_match.go ReplMoved Discard$ True, EffectOnly$ True, ValidCause$ SpellAbility.OppCtrl)
	ReplPreventMillOpps          // damage a source the controller controls would deal to an opponent is prevented and that opponent mills that many cards instead (The Mindskinner; gorge's ReplaceWith$ DB$ Mill body, NumCards$ ReplaceCount$DamageAmount)
	ReplAddToken                 // a token matching Repl.Filter (none: every token) the controller would create is made, and one token of pool token index Repl.C is made beside the creation as well, once per creation event (R:Event$ CreateToken | ValidPlayer$ You | ReplaceWith$ DB$ ReplaceToken | Type$ AddToken | Amount$ 1 | TokenScript$; Worldwalker Helm). The matchup remaps Repl.C to its CardID
	ReplCount
)

// ReplDrawMore flags (Repl.C).
const ReplDrawNotFirst = 1 // NotFirstCardInDrawStep$ True

// Repl is a replacement-lite effect.
type Repl struct {
	Kind ReplKind
	A    int16
	C    int16

	// Appended by dirac/kgen.
	Filter FilterID // ReplOthersEnterProgram: the entering objects it applies to
	Cond   CondID   // the replacement applies only while Table.Conds[Cond] holds (0 = always)
}

// Repl.C of a ReplCantLoseGame: 0 stops every loss of the game; CantLoseLife
// stops only the loss for having 0 or less life (ValidLoseReason$
// LifeReachedZero: Phyrexian Unlife, Lich's Tomb), leaving poison, a draw from
// an empty library and "loses the game" effects to take their course.
const CantLoseLife = 1

// Repl.C of ReplDoubleCountersOn / ReplAddCountersOn is a counter kind, or one
// of these wildcards: the replacement names no
// ValidCounterType$ and applies to every kind placed (Loading Zone, Doc
// Samson). Negative, so no kind collides.
const (
	ReplCtrAny    int16 = -1 // every kind
	ReplCtrAnyYou int16 = -2 // every kind, only counters the source's controller puts (ValidSource$ You: gorge's in-flight counter adder)
)

// ReplAfterEnter (Repl.C of a ReplEntersProgram): gorge asks the program's
// choice once the object is on the battlefield (an R: line with
// ReplacementResult$ Updated, Banner of Kinship), not while it is still a
// spell on the stack (an ETBReplacement keyword, Heraldic Banner).
const ReplAfterEnter = 1
