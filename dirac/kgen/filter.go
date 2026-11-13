package kgen

import (
	"fmt"
	"math/bits"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Card types (lFilter.Types, lCard.Types).
const (
	tLand uint16 = 1 << iota
	tCreature
	tArtifact
	tEnchantment
	tPlaneswalker
	tInstant
	tSorcery
	tBattle
	tLegendary // supertypes ride in the same mask for the static record
	tBasic
	tKindred   // a card type above the low byte (kop.TKindred is bit 8)
	tEmblem    // an emblem (a filter alternative only; no card has it)
	tPermanent = tLand | tCreature | tArtifact | tEnchantment | tPlaneswalker | tBattle
	tAnyCard   = tPermanent | tInstant | tSorcery
)

var typeBits = map[string]uint16{
	"Land": tLand, "Creature": tCreature, "Artifact": tArtifact, "Enchantment": tEnchantment,
	"Planeswalker": tPlaneswalker, "Instant": tInstant, "Sorcery": tSorcery, "Battle": tBattle,
	"Legendary": tLegendary, "Basic": tBasic, "Kindred": tKindred, "Permanent": tPermanent,
	"Card": tAnyCard, "Emblem": tEmblem,
}

const tSupertype = tLegendary | tBasic

// typeAddBit reports whether w is a card type or a modeled supertype in an
// AddType$ position.
func typeAddBit(w string) (uint16, bool) {
	b, ok := typeBits[w]
	if !ok || b&(tAnyCard|tSupertype) == 0 {
		return 0, false
	}
	return b, true
}

// Filter flags (lClause.Flags).
const (
	fSelf uint64 = 1 << iota
	fOther
	fToken
	fNonToken
	fAttacking
	fBlocking
	fTapped
	fUntapped
	fWithFlying
	fWithoutFlying
	fKicked
	fCastByYou
	fEquippedBy // attached to by the source (Equipment)
	fEnchantedBy
	fEnteredThisTurn // ThisTurnEntered: entered its current zone this turn
	fChosenType
	fChosenColor
	fRemembered
	fNotRemembered
	fChosenCard
	fNotChosenCard
	fAttachedToParentTarget
	fTargetedBy // targeted by the resolving ability
	fNamedAsSource
	fBasic
	fSharesColorWithOtherRemembered
	fNotTriggerObj
	fPromisedGift  // the gift was promised; read only as Card.Self+PromisedGift (a cPromisedGift condition), no filter form
	fWasDamaged    // wasDealtDamageThisTurn
	fLegendary     // Legendary with a type head (Creature.Legendary); a Legendary head alone rides in Types
	fSaddled       // IsSaddled (CR 702.171b; gorge filter.go: SaddledTurn == the turn)
	fSneaked       // the spell was cast for its sneak cost (CR 702.190a; stack reads only)
	fBlockedBySrc  // blockedBySourceLKI: attacking, blocked by the source
	fDamagedBy     // DamagedBy: dealt damage by the source this turn
	fCrewedSrc     // CrewedThisTurn: crewed the source this turn
	fWarped        // CastSa Spell.Warp: a battlefield permanent cast for its warp cost
	fBargained     // bargained: the permanent's spell was bargained (CR 702.166; Card.Self+bargained reads F2Kicked)
	fEscaped       // escaped: the permanent's spell was cast for its escape cost (CR 702.138a; kop.PredEscaped, the arena's mechEscaped)
	fTeamwork      // Teamwork: the permanent's spell was teamworked (CR 702.194b; Card.Self+Teamwork reads F2Kicked)
	fExiledWith    // ExiledWithSource: exiled with the reading source (kop.FExiledWith)
	fNotAttacking  // !attacking (kop.FNotAttacking)
	fNotAttachedBy // !EnchantedBy / !EquippedBy / !AttachedBy: not the source's host (kop.FNotAttachedBySrc)
	fFlagCount     = iota
)

var flagNames = [...]string{"Self", "Other", "Token", "NonToken", "Attacking", "Blocking", "Tapped", "Untapped",
	"WithFlying", "WithoutFlying", "Kicked", "CastByYou", "EquippedBy", "EnchantedBy", "EnteredThisTurn",
	"ChosenType", "ChosenColor", "Remembered", "NotRemembered", "ChosenCard", "NotChosenCard",
	"AttachedToParentTarget", "TargetedBy", "NamedAsSource", "Basic", "SharesColorWithOtherRemembered", "NotTriggerObj", "PromisedGift", "WasDamaged", "Legendary", "Saddled", "Sneaked", "BlockedBySrc", "DamagedBy", "CrewedSrc", "Warped", "Bargained", "Escaped", "Teamwork", "ExiledWith", "NotAttacking", "NotAttachedBy"}

// Controller / owner selectors.
const (
	whoAny uint8 = iota
	whoYou
	whoOpp
	whoEnchCtrl // Phase only: the controller of the permanent the source is attached to (kop.WhoseEnchCtrl)
	// whoTgtPlayer: TargetedPlayerCtrl (the controller is a player target
	// of the resolving entry; gorge matchTargetedPlayerCtrl,
	// kop.F2TgtPlayerCtrl, battlefield only) and, on the Owner side, OwnedBy
	// Targeted / TargetedPlayerOwn (gorge matchTargetedPlayerOwn, off the
	// battlefield only)
	whoTgtPlayer
	// whoDefending: ControlledBy TriggeredDefendingPlayer inside an Attacks
	// trigger's body (gorge controlReferentPlayers' sc.DefendingPlayer;
	// kop.F2DefendingCtrl, battlefield only)
	whoDefending
	// whoEachPl: RememberedPlayerCtrl inside a RepeatEach player body, the
	// repeated player (kop.CtrlEachPl, battlefield only; lower_repeatplayers.go)
	whoEachPl
	// whoHitBySrc: ControlledBy Player.wasDealtCombatDamageThisTurnBySource,
	// the controller was dealt combat damage this turn by the filter's
	// source (gorge dealtCombatDamageBySource; kop.F2CombatHitCtrl,
	// battlefield only, the card's own ability)
	whoHitBySrc
	// whoTgtOrCtrl: ControlledBy TargetedOrController (gorge
	// controlReferentPlayers TargetedOrController): the controller of the
	// current link's target -- a player target is that player, an object
	// target its controller (kop.Filter.CtrlTgt, battlefield only)
	whoTgtOrCtrl
	// whoTgtCtrl: OwnedBy TargetedController (gorge controlReferentPlayers
	// TargetedController, op OwnedBy), on the Owner side only: the card's
	// owner is the controller of the current link's card target, which
	// lies off the battlefield (kop.CtrlTgtOwner; off the battlefield only)
	whoTgtCtrl
)

// Compare statistics (lClause.CmpStat).
const (
	cmpNone uint8 = iota
	cmpPower
	cmpCMC
	cmpCounters // counters of kind CmpCounter
	cmpToughness
	cmpHasX // the printed mana cost has {X} (hasXCost; no operator)
	cmpPowerLTToughness
	cmpMultiColor     // MultiColor: two or more colours (no operator)
	cmpHistoric       // Historic: artifact, legendary or Saga (no operator)
	cmpEquipped       // equipped: an Equipment is attached (no operator)
	cmpEnchanted      // enchanted: an Aura is attached (no operator)
	cmpHasKw          // with<Keyword>: has keyword bit CmpN (no operator)
	cmpAttached       // Attached: attached to a permanent (no operator)
	cmpActiveCtrl     // ActivePlayerCtrl: the active player controls it (no operator)
	cmpModified       // modified: a counter, an Equipment, or an Aura its controller controls (no operator)
	cmpHasCounters    // HasCounters: any counter, internal markers included (no operator)
	cmpTrigPlayerCtrl // the trigger's damaged player controls it (kop.PredTrigPlayerCtrl; no operator)
	cmpMonstrous      // IsMonstrous: the CR 701.37b monstrous designation (no operator)
	cmpNoCounters     // !HasCounters: no counter at all (no operator)
	cmpLacksKw        // !with<Keyword>: lacks keyword bit CmpN (no operator)
	cmpOutlaw         // Outlaw (CmpN 0) / !Outlaw (CmpN 1): an Assassin, Mercenary, Pirate, Rogue or Warlock (no operator)
	cmpMonoColor      // MonoColor: exactly one colour (no operator)
	cmpSameNameT0     // target 0 or a permanent sharing its name (Maelstrom Pulse)
	cmpTotalPT        // totalPT_<op><n>: power plus toughness (Cut Down)
	cmpBlocked        // blocked: attacking with a blocker recorded (no operator; Tactical Advantage)
	cmpSolved         // IsSolved (CmpN 1) / !IsSolved (CmpN 0): a Case's solved designation
	cmpAdventureCard  // AdventureCard: an adventurer card, every zone (no operator)
	cmpSuspected      // IsSuspected (CmpN 1) / !IsSuspected (CmpN 0): the CR 701.60 suspected designation
	cmpExiledWith     // ExiledWithSource (CmpN 0) / Defined$ ExiledWith (CmpN 1): exiled with the reading source (no operator)
	cmpWasCast        // wasCast / !wasCast / wasCastFromGraveyard read as the permanent enters from its own spell (CmpN: wasCast* origin | wasCastNot, origin 3 = wasCastFromYourGraveyard; no operator)
	cmpWasCastSpell   // wasCastFromYourHand[ByYou] / !wasCastFromYourHand / wasCastFromExile / CastSa Spell.MayPlaySource (or wasCast beside them) read on a spell on the stack (CmpN: spell* origin | spellNot; no operator)
	cmpPermMark       // attackedThisTurn / dealtDamageThisTurn / dealtDamagetoAny / dealtCombatDamagetoAny, with or without ! (CmpN: kop.PermMark* | kop.PermMarkNot; no operator)
	cmpTriggerObj     // the resolving trigger's object, wherever it is now (ChooseFromDefined$ TriggeredCards; kernel isTriggerObj; no operator)
	cmpSameNameRem    // shares a name with an object in the remembered set (gorge sharesNameWith Remembered; no operator)
	cmpHasManaAbility // hasManaAbility (CmpN 0) / !hasManaAbility (CmpN 1): the printed face has a mana ability (no operator)
)

// cmpWasCastSpell's CmpN (kernel PredWasCastSpell's PredN): the origin the
// spell must have been cast from, or'd with spellNot for the negated word.
const (
	spellCast    = 0 // wasCast: cast at all (a copy was never cast)
	spellHand    = 1 // wasCastFromYourHand: from a hand, any caster (gorge WasCastFromHand)
	spellHandBy  = 2 // wasCastFromYourHandByYou: from a hand by the reading player
	spellExile   = 3 // wasCastFromExile
	spellNotHand = 4 // wasCast+!wasCastFromYourHand: cast, from anywhere but a hand
	spellMayPlay = 5 // CastSa Spell.MayPlaySource: cast through a may-play permission (gorge FlagMayPlay)
	spellMayhem  = 6 // CastSa Spell.Mayhem: cast for its mayhem cost (gorge FlagMayhem; a copy reads false)
	spellNot     = 16
)

// cmpWasCast's CmpN (kernel PredWasCastEnter's PredN): the origin the entering
// spell must have been cast from, or'd with wasCastNot for "!wasCast".
const (
	wasCastAny       = 0
	wasCastGrave     = 1
	wasCastHand      = 2 // cast from its owner's hand by the permanent's controller; synthesised for Count$wasCastFromYourHandByYou, never a filter word
	wasCastYourGrave = 3 // wasCastFromYourGraveyard: cast from a graveyard by the reading player (gorge castOriginTokens)
	wasCastNot       = 4
	wasCastSpentZero = 8  // CastSa Spell.ManaSpent EQ0: cast, and no mana was spent (gorge castSaTokenHolds)
	wasCastWebSling  = 16 // webSlinged: cast for its web-slinging cost (gorge webSlingedAdmits; a copy reads false)
)

// Comparison operators.
const (
	opGE uint8 = iota + 1
	opLE
	opEQ
	opGT
	opLT
	opNE
)

var opNamesCmp = map[string]uint8{"GE": opGE, "LE": opLE, "EQ": opEQ, "GT": opGT, "LT": opLT, "NE": opNE}

// lClause is one alternative of a Forge Valid expression: an object matches
// when every field matches.
type lClause struct {
	Types      uint16 // any-of card types (0 = any card)
	TypesAll   uint16 // all-of card types (a disjoint type property beside a type head: "Artifact.Creature"; Types holds their union)
	NotTypes   uint16 // none-of card types
	Subtype    string // must have this subtype ("" = none)
	Name       string // named<Name>, normalised (lower.go: the source's own name keeps fNamedAsSource, another is named-other)
	NotSubtype string // must not have this subtype
	Subtype2   string // a second subtype it must also have (a Spider Hero); the kernel filter's Pred slot carries it
	// MoreNotSubtype are further subtypes it must not have, filled only
	// once NotSubtype is (kop.Filter.NotSubs)
	MoreNotSubtype [4]string
	Ctrl           uint8 // controller selector
	Owner          uint8 // owner selector
	Flags          uint64
	CmpStat        uint8
	CmpOp          uint8
	CmpN           int     // the constant, or
	CmpX           string  // the SVar naming the bound (cmcLEX)
	CmpV           *lValue // CmpX lowered
	CmpCounter     string  // counter kind for cmpCounters
	Colors         string  // must have this colour (one of WUBRG; "" = none)
	NotColors      string  // must have none of these colours
	BasePermanent  bool    // the base head word was Permanent (ThisTurnEntered's card form marks it)
	// SpellBase: the base head word was Spell (gorge's Spell base: an
	// object on the stack). Read only by a ReduceCost ValidTarget$
	// (build_ab: a Filter2 in ZoneStack with FSpell); clause refuses it
	// anywhere else.
	SpellBase bool
	// Unblocked: attacking with no blocker recorded (CR 509.1h; gorge
	// filter.go "unblocked", K:Ninjutsu's Return cost). Its own field: the
	// Flags word has no free bit.
	Unblocked bool
	// Special: a RestrictValid$ Static.<tail> alternative's special actions
	// (kop.SpecialMask). Every other clause leaves it 0.
	Special kop.SpecialMask
	// TheirHand: the clause's provenance word is wasCastFromTheirHand (the
	// same predicate as wasCastFromYourHand); only a CantBeCast ValidCard$
	// admits it (lowerCtx.cantCastProv).
	TheirHand bool
	// CtrlTgt is the target slot (index+1) whose controller must control the
	// candidate (kop.Filter.CtrlTgt; whoTgtOrCtrl), set by lowerCtx.filter.
	CtrlTgt uint8
}

// lFilter is a Forge Valid expression: any clause matching (OR), plus the
// player alternatives it admits.
type lFilter struct {
	Clauses []lClause
	Players uint8 // bit 1 = you, bit 2 = opponent
	// Each: Forge's "EACH A & B" list (lowerCtx.pickFilter). Every clause is
	// one listed type, in order, and a pick takes one card per clause.
	Each bool
	// PickSumMax and PickDistinctPower are the pick-set constraints carried
	// on the head clause (kop.Filter.PickSumMax / PickDistinctPower):
	// WithTotalCMC$ and WithDifferentPowers$ on a Dig or a hidden pick. Nil
	// PickSumMax and false PickDistinctPower leave the filter unchanged.
	PickSumMax        *lValue
	PickDistinctPower bool
}

const (
	plYou uint8 = 1 << iota
	plOpp
	plBoth = plYou | plOpp
)

// filterSelfOnly reports whether every clause of f can match only the source
// permanent (Card.Self). A static granting an ability over such a filter is a
// self-grant: the recipient is the grantor, so its SVar references resolve
// against the grantor's face in gorge too. Anything else may affect another
// permanent.
func filterSelfOnly(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&fSelf == 0 {
			return false
		}
	}
	return true
}

func (f *lFilter) String() string {
	if f == nil {
		return "nil"
	}
	var parts []string
	if f.Players != 0 {
		parts = append(parts, fmt.Sprintf("players=%d", f.Players))
	}
	for _, c := range f.Clauses {
		parts = append(parts, c.String())
	}
	return "{" + strings.Join(parts, " | ") + "}"
}

func (c lClause) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "types=%#x", c.Types)
	if c.TypesAll != 0 {
		fmt.Fprintf(&b, " all=%#x", c.TypesAll)
	}
	if c.NotTypes != 0 {
		fmt.Fprintf(&b, " not=%#x", c.NotTypes)
	}
	if c.Subtype != "" {
		fmt.Fprintf(&b, " sub=%s", c.Subtype)
	}
	if c.Subtype2 != "" {
		fmt.Fprintf(&b, " sub2=%s", c.Subtype2)
	}
	if c.NotSubtype != "" {
		fmt.Fprintf(&b, " notsub=%s", c.NotSubtype)
	}
	for _, w := range c.MoreNotSubtype {
		if w != "" {
			fmt.Fprintf(&b, " notsub=%s", w)
		}
	}
	if c.Ctrl != 0 {
		fmt.Fprintf(&b, " ctrl=%d", c.Ctrl)
	}
	if c.Owner != 0 {
		fmt.Fprintf(&b, " owner=%d", c.Owner)
	}
	for i := 0; i < fFlagCount; i++ {
		if c.Flags&(1<<i) != 0 {
			b.WriteString(" " + flagNames[i])
		}
	}
	if c.Unblocked {
		b.WriteString(" Unblocked")
	}
	if c.Colors != "" {
		fmt.Fprintf(&b, " color=%s", c.Colors)
	}
	if c.NotColors != "" {
		fmt.Fprintf(&b, " notcolor=%s", c.NotColors)
	}
	if c.CmpStat != 0 {
		fmt.Fprintf(&b, " cmp(%d %d %d %s %s)", c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter)
	}
	if c.Special != 0 {
		fmt.Fprintf(&b, " special=%#x", c.Special)
	}
	return b.String()
}

// subtypeWords is every subtype word the pool's filters may name; a filter
// word outside both typeBits and this set refuses (never guessed).
var subtypeWords = map[string]bool{}

// propWords maps a filter property word to the flag it sets.
var propWords = map[string]uint64{
	"Self": fSelf, "StrictlySelf": fSelf, "Other": fOther, "StrictlyOther": fOther, "token": fToken, "!token": fNonToken, "nonToken": fNonToken,
	// gorge's tokenCreated (effects/filter.go) is the IsToken marker, the same
	// read as "token": a token created this turn is the entered ring's entry
	"tokenCreated": fToken,
	"attacking":    fAttacking, "!attacking": fNotAttacking, "!TriggeredCard": fNotTriggerObj, "blocking": fBlocking, "tapped": fTapped, "untapped": fUntapped,
	"withFlying": fWithFlying, "withoutFlying": fWithoutFlying, "kicked": fKicked, "wasCastByYou": fCastByYou,
	"EquippedBy": fEquippedBy, "EnchantedBy": fEnchantedBy, "AttachedBy": fEquippedBy, "ThisTurnEntered": fEnteredThisTurn,
	// gorge matchPredicate's '!' of attachedBy: true for
	// every object but the source's live host
	"!EnchantedBy": fNotAttachedBy, "!EquippedBy": fNotAttachedBy, "!AttachedBy": fNotAttachedBy,
	"ChosenType": fChosenType, "ChosenColor": fChosenColor, "IsRemembered": fRemembered,
	"!IsRemembered": fNotRemembered, "ChosenCard": fChosenCard, "nonChosenCard": fNotChosenCard, "!ChosenCard": fNotChosenCard,
	"AttachedTo ParentTarget": fAttachedToParentTarget, "targetedBy": fTargetedBy, "Basic": fBasic,
	"SharesColorWithOther Remembered": fSharesColorWithOtherRemembered,
	"NotDefinedTriggeredCard":         fNotTriggerObj,
	// negation of TriggeredAttacker (gorge notDefinedTriggerReferent), gated
	// to an Attacks trigger's body by lc.referentGate (lower_filterreferents.go)
	"NotDefinedTriggeredAttacker": fNotTriggerObj,
	// gorge's '!' of TriggeredAttacker (matchTriggeredAttacker negated by
	// matchPositive, an unbound referent failing closed as the NotDefined
	// spelling does), under the same gate
	"!TriggeredAttacker": fNotTriggerObj,
	// gorge attachedToReferentCodes: Targeted and ParentTarget are one
	// referent; lc.referentGate admits it only where the node has no target
	// slot of its own
	"AttachedTo Targeted": fAttachedToParentTarget,
	// "other than that creature" on a BecomesTarget watcher's body (Pawpatch
	// Recruit, the only corpus carrier): the trigger object is the targeted
	// object (gorge becomesTargetReferent), as NotDefinedTriggeredCard's is
	// the triggering card
	"NotDefinedTriggeredTarget": fNotTriggerObj,
	"PromisedGift":              fPromisedGift,
	"wasDealtDamageThisTurn":    fWasDamaged,
	"IsSaddled":                 fSaddled,
	"sneaked":                   fSneaked,
	"blockedBySourceLKI":        fBlockedBySrc,
	"DamagedBy":                 fDamagedBy,
	"CrewedThisTurn":            fCrewedSrc,
	"SaddledThisTurn":           fCrewedSrc, // gorge: the exact twin of CrewedThisTurn (one pairedWithSourceThisTurn list)
	"CastSa Spell.Warp":         fWarped,
	"bargained":                 fBargained, // CR 702.166: Card.Self+bargained (kgen gates it on the CardDef.Bargain below)
	"escaped":                   fEscaped,   // CR 702.138a: Card.Self+escaped (kop.PredEscaped; gated on CardDef.Escape below)
	"Teamwork":                  fTeamwork,  // CR 702.194b: Card.Self+Teamwork (gated on CardDef.Teamwork)
	"inZoneBattlefield":         0,          // implied by every battlefield context this pool uses it in
}

// colorWords maps a filter colour word to its colour letter.
var colorWords = map[string]string{"White": "W", "Blue": "U", "Black": "B", "Red": "R", "Green": "G"}

// splitAlternatives splits the OR grammar the way gorge's filter
// filterAlternatives does: a comma is a boundary unless it sits inside a
// named<Name>/notnamed<Name> argument whose right side does not begin a
// syntactic alternative (Page, Loose Leaf's grandeur Discard cost carries
// Card.namedPage, Loose Leaf; gorge effects/filter.go filterAlternatives).
// isType is the corpus's type vocabulary (pool.isTypeWord), not the pool's
// subtypes: gorge's rawNameComma answers on predicateTypeWords.
func splitAlternatives(spec string, isType func(string) bool) []string {
	var out []string
	start := 0
	for i := 0; i < len(spec); i++ {
		if spec[i] != ',' || rawNameComma(spec[start:i], spec[i+1:], isType) {
			continue
		}
		out = append(out, spec[start:i])
		start = i + 1
	}
	return append(out, spec[start:])
}

// rawNameComma reports whether the comma after left belongs to the last
// predicate of the current alternative (gorge effects/filter.go
// rawNameComma). Once '+' has started another predicate, a name argument is
// complete and cannot own a following comma.
func rawNameComma(left, right string, isType func(string) bool) bool {
	_, preds, has := strings.Cut(left, ".")
	if !has {
		return false
	}
	last := preds[strings.LastIndexByte(preds, '+')+1:]
	if !strings.HasPrefix(last, "named") && !strings.HasPrefix(last, "notnamed") {
		return false
	}
	return !startsFilterAlternative(right, isType)
}

// startsFilterAlternative recognises the base at the start of an
// alternative, independently of the candidate object (gorge
// effects/filter.go startsFilterAlternative): the universal bases and every
// type word, with the ordinary non<X> negation.
func startsFilterAlternative(s string, isType func(string) bool) bool {
	base := s
	if i := strings.IndexAny(base, ".+,"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)
	switch base {
	case "CARDNAME", "Any", "Card", "Permanent", "Spell", "Affinity":
		return true
	}
	base = strings.TrimPrefix(base, "non")
	if _, ok := typeBits[base]; ok {
		return true
	}
	return base != "" && isType(base)
}

// parseFilter parses a Forge Valid expression. knownSub reports whether a
// word is a subtype of some card or token in the pool; isType is the
// corpus's type vocabulary for the alternative boundary.
func parseFilter(expr string, knownSub, isType func(string) bool) (*lFilter, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty filter")
	}
	f := &lFilter{}
	for _, alt := range splitAlternatives(expr, isType) {
		alt = strings.TrimSpace(alt)
		switch alt {
		case "Player":
			f.Players |= plBoth
			continue
		case "Opponent", "Player.Opponent":
			// gorge's player matcher reads Player.Opponent as Opponent
			f.Players |= plOpp
			continue
		case "You":
			f.Players |= plYou
			continue
		case "Any":
			f.Players |= plBoth
			f.Clauses = append(f.Clauses, lClause{Types: tCreature | tPlaneswalker})
			continue
		}
		c, err := parseClause(alt, knownSub)
		if err != nil {
			hc, ok := historicClauses(alt, knownSub, err)
			if !ok {
				return nil, err
			}
			f.Clauses = append(f.Clauses, hc...)
			continue
		}
		f.Clauses = append(f.Clauses, c)
	}
	return f, nil
}

// historicClauses splits a clause whose Historic sits beside another
// comparison ("Permanent.Historic+cmcGE3", two Pred reads in one kernel
// filter) into its three alternatives, an artifact, a legendary and a Saga,
// each keeping the rest: gorge historicMatches is exactly that union.
func historicClauses(alt string, knownSub func(string) bool, err error) ([]lClause, bool) {
	if !strings.Contains(err.Error(), "two comparisons in one clause") {
		return nil, false
	}
	head, props, _ := strings.Cut(alt, ".")
	ps := strings.Split(props, "+")
	at := slices.Index(ps, "Historic")
	if at < 0 {
		return nil, false
	}
	var out []lClause
	for _, w := range [...]string{"Artifact", "Legendary", "Saga"} {
		rest := slices.Clone(ps)
		if w == "Saga" {
			rest = slices.Delete(rest, at, at+1)
		} else {
			rest[at] = w
		}
		alt := head
		if len(rest) > 0 {
			alt += "." + strings.Join(rest, "+")
		}
		c, err := parseClause(alt, knownSub)
		if err != nil {
			return nil, false
		}
		if w == "Saga" {
			// the subtype by name, as PredHistoric reads it, whether or
			// not the pool's vocabulary lists it yet
			if c.Subtype != "" {
				return nil, false
			}
			c.Subtype = "Saga"
		}
		out = append(out, c)
	}
	return out, true
}

// parseClause parses one alternative of a Valid expression.
func parseClause(alt string, knownSub func(string) bool) (lClause, error) {
	head, props, _ := strings.Cut(alt, ".")
	var c lClause
	if bits, ok := typeBits[head]; ok {
		c.Types = bits
		if head == "Card" {
			c.Types = 0
		}
		if head == "Permanent" {
			c.BasePermanent = true
		}
	} else if head == "Spell" {
		c.SpellBase = true
	} else if head == "Outlaw" {
		// gorge matchesBaseOutlaw: any object with an outlaw type through
		// the layer-aware type read, the Card.Outlaw spelling
		if err := c.setCmp(cmpOutlaw, 0, 0, "", ""); err != nil {
			return c, fmt.Errorf("%q: %w", alt, err)
		}
	} else if knownSub(head) {
		c.Subtype = head
	} else {
		return c, fmt.Errorf("unknown filter head %q", head)
	}
	if props != "" {
		for _, p := range strings.Split(props, "+") {
			if err := c.prop(p, knownSub); err != nil {
				return c, fmt.Errorf("%q: %w", alt, err)
			}
		}
	}
	return c, nil
}

func (c *lClause) prop(p string, knownSub func(string) bool) error {
	switch p {
	case "nonLegendary":
		c.NotTypes |= tLegendary
		return nil
	case "nonBasic":
		// gorge filter nonBasic: !hasType Basic
		c.NotTypes |= tBasic
		return nil
	case "Colorless":
		// gorge wordColorless: no colour at all (current colours)
		for _, col := range "WUBRG" {
			if !strings.ContainsRune(c.NotColors, col) {
				c.NotColors += string(col)
			}
		}
		return nil
	case "nonColorless":
		// at least one colour: the any-of Filter.Colors over all five
		if c.Colors != "" {
			return fmt.Errorf("two colours in one clause")
		}
		c.Colors = "WUBRG"
		return nil
	}
	if fl, ok := propWords[p]; ok {
		c.Flags |= fl
		return nil
	}
	switch p {
	case "attackingYou":
		// gorge attackingYou: attacking "you", the evaluating player
		// (o.Attacking names the defending seat, a planeswalker's
		// controller or a battle's protector too). In two players every
		// attacker attacks the non-active player's side, so it is an attacking creature the
		// controller's opponent controls (Confront the Assault;
		// twoSeat-at-caller: lowerCtx.filter marks the card)
		c.Flags |= fAttacking
		return c.setWho(&c.Ctrl, whoOpp)
	case "YouCtrl":
		return c.setWho(&c.Ctrl, whoYou)
	case "OppCtrl", "YouDontCtrl":
		return c.setWho(&c.Ctrl, whoOpp)
	case "YouOwn":
		return c.setWho(&c.Owner, whoYou)
	case "OppOwn", "YouDontOwn":
		return c.setWho(&c.Owner, whoOpp)
	case "TargetedPlayerCtrl":
		// gorge wordTargetedPlayerCtrl (matchTargetedPlayerCtrl): the
		// controller is one of the resolution's player targets
		return c.setWho(&c.Ctrl, whoTgtPlayer)
	case "ControlledBy TargetedOrController":
		// gorge controlReferentPlayers TargetedOrController: the current
		// link's target's controller, a player target read as itself and an
		// object target as its controller (kop.Filter.CtrlTgt)
		return c.setWho(&c.Ctrl, whoTgtOrCtrl)
	case "ControlledBy TriggeredDefendingPlayer", "DefenderCtrl":
		// gorge controlReferentPlayers TriggeredDefendingPlayer: the
		// trigger's DefendingPlayer (lc.filter admits it only in an
		// Attacks trigger's body)
		return c.setWho(&c.Ctrl, whoDefending)
	case "ControlledBy Player.wasDealtCombatDamageThisTurnBySource", "controllerWasDealtCombatDamageByThisTurn":
		// gorge dealtCombatDamageBySource (filter_word.go:382 reads both
		// spellings as one word): a hit on the candidate's controller by the
		// filter's source this turn (kernel CtrlHitBySrc; lc.filter
		// admits it only against the card's own ability)
		return c.setWho(&c.Ctrl, whoHitBySrc)
	case "RememberedPlayerCtrl":
		return c.setWho(&c.Ctrl, whoEachPl) // lower_repeatplayers.go
	case "RememberedPlayerOwn":
		// a card off the battlefield is controlled by its owner: the same
		// seat as RememberedPlayerCtrl there (Singularity Rupture's
		// Count$ValidLibrary). The clause() emitter keeps it off the
		// battlefield, where owner and controller part
		return c.setWho(&c.Owner, whoEachPl)
	case "OwnedBy TriggeredDefendingPlayer":
		// gorge controlReferentPlayers op OwnedBy; off the battlefield only
		// (build.go ownerRoleF2), in an Attacks trigger's body as the
		// ControlledBy twin is
		return c.setWho(&c.Owner, whoDefending)
	case "OwnedBy TargetedController":
		// gorge controlReferentPlayers TargetedController: the controller of
		// the current link's object target (a player target binds nothing),
		// compared with the candidate's owner; kgen binds the slot in
		// lowerCtx.filter and the card matcher reads it (kop.CtrlTgtOwner)
		return c.setWho(&c.Owner, whoTgtCtrl)
	case "OwnedBy Targeted", "TargetedPlayerOwn":
		// TargetedPlayerOwn is gorge wordTargetedPlayerOwn
		// (matchTargetedPlayerOwn): same referent as OwnedBy Targeted --
		// gorge's controlReferentPlayers reads "Targeted" and "TargetedPlayer"
		// as one code, so the resolution's player targets -- read as
		// OwnedBy on the candidate's owner
		return c.setWho(&c.Owner, whoTgtPlayer) // off the battlefield only (ownerRoleF2)
	}
	if p == "IsSolved" || p == "!IsSolved" {
		// a Case's solved designation (CR 719.3b; gorge filter.go IsSolved:
		// Object.Solved, set by AlterAttribute Solved and never cleared on
		// the battlefield): CmpN 1 solved, 0 not
		n := 1
		if p[0] == '!' {
			n = 0
		}
		return c.setCmp(cmpSolved, 0, n, "", "")
	}
	if p == "IsSuspected" || p == "!IsSuspected" {
		// the suspected designation (CR 701.60; gorge filter.go IsSuspected:
		// Object.Suspected, false off the battlefield): CmpN 1 suspected, 0
		// not
		n := 1
		if p[0] == '!' {
			n = 0
		}
		return c.setCmp(cmpSuspected, 0, n, "", "")
	}
	if n, ok := permMarkWord(p); ok {
		// the per-permanent attacked / dealt-damage marks (kop.PredPermMark;
		// gorge AttackedTurn, DamageDealtThisTurn, DealtDamageToAnyGame,
		// DealtCombatDamageToAnyGame): false off the battlefield, so the
		// negated word is true there
		return c.setCmp(cmpPermMark, 0, n, "", "")
	}
	if p == "blocked" {
		// gorge "blocked" (cr-blocked-filter): IsAttacking and a recorded
		// blocker, the tombstone included
		return c.setCmp(cmpBlocked, 0, 0, "", "")
	}
	if rest, ok := strings.CutPrefix(p, "totalPT_"); ok && len(rest) > 2 {
		// gorge totalPT (cr-totalpt-filter): current power plus toughness
		op, ok := opNamesCmp[rest[:2]]
		n, err := strconv.Atoi(rest[2:])
		if !ok || err != nil {
			return fmt.Errorf("totalPT compare %q", p)
		}
		return c.setCmp(cmpTotalPT, op, n, "", "")
	}
	if p == "unblocked" {
		c.Unblocked = true
		return nil
	}
	if p == "hasXCost" {
		return c.setCmp(cmpHasX, 0, 0, "", "")
	}
	switch p {
	case "MultiColor":
		// gorge wordMultiColor: more than one current colour
		return c.setCmp(cmpMultiColor, 0, 0, "", "")
	case "Historic":
		// gorge historicMatches: artifact, legendary or Saga
		return c.setCmp(cmpHistoric, 0, 0, "", "")
	case "equipped":
		// gorge equipped: an Equipment permanent is attached to it
		return c.setCmp(cmpEquipped, 0, 0, "", "")
	case "withDefender", "withDeathtouch", "withTrample", "withHaste", "withMenace", "withVigilance", "withReach",
		"withLifelink", "withIndestructible", "withFirstStrike", "withDoubleStrike", "withShadow":
		// gorge objectHasKeyword over the derived keywords (filter.go's
		// with<Keyword> registry): PredHasKeyword
		return c.setCmp(cmpHasKw, 0, withKwBit(p), "", "")
	case "hasKeywordFlash":
		// gorge registers hasKeyword<K> as with<K> (objectHasKeyword); only
		// Flash is read
		return c.setCmp(cmpHasKw, 0, bits.TrailingZeros32(uint32(kop.KwFlash)), "", "")
	case "Attached":
		// gorge objectAttachedToSource: the candidate is attached to the source
		return c.setCmp(cmpAttached, 0, 0, "", "")
	case trigPlayerCtrlWord:
		// triggeredTargetCtrl's rewrite of ControlledBy TriggeredTarget in a
		// damage-to-a-player trigger: the damaged player controls it
		return c.setCmp(cmpTrigPlayerCtrl, 0, 0, "", "")
	case "ActivePlayerCtrl", "ControlledBy Player.Active":
		// gorge wordActivePlayerCtrl: its controller is the active player
		return c.setCmp(cmpActiveCtrl, 0, 0, "", "")
	case "enchanted", "Enchanted":
		// gorge enchanted: an Aura permanent is attached to it
		return c.setCmp(cmpEnchanted, 0, 0, "", "")
	case "modified":
		// gorge modifiedPermanent (CR 700.9): a counter (not the
		// internal Shield/Deathtouched markers, which the kernel never
		// stores as counters), an Equipment attached, or an Aura its
		// controller controls attached
		return c.setCmp(cmpModified, 0, 0, "", "")
	case "HasCounters":
		// gorge wordHasCounters: hasCounters over Object.Counters, the
		// internal Shield/Deathtouched markers INCLUDED (unlike modified)
		return c.setCmp(cmpHasCounters, 0, 0, "", "")
	case "!HasCounters":
		// gorge's '!' negation of wordHasCounters (hasCounters, the
		// internal Shield/Deathtouched markers included)
		return c.setCmp(cmpNoCounters, 0, 0, "", "")
	case "Outlaw", "!Outlaw":
		// gorge outlawMatches (the Outlaw batch word): any of the five
		// outlaw creature types through the layer-aware type read
		n := 0
		if p[0] == '!' {
			n = 1
		}
		return c.setCmp(cmpOutlaw, 0, n, "", "")
	case "hasManaAbility", "!hasManaAbility":
		// gorge FaceHasManaAbility (an activated CR 605.1a ability, the
		// intrinsic basic-land one included, or a CR 605.1b triggered one):
		// the kernel reads CardDef.HasManaAbility; the '!' is PredN 1.
		n := 0
		if p[0] == '!' {
			n = 1
		}
		return c.setCmp(cmpHasManaAbility, 0, n, "", "")
	case "ExiledWithSource", "ExiledWithSourceLKI":
		// gorge exiledBySource (both spellings read the one helper): the
		// card's reverse ExiledWith link or the source's forward list; a
		// flag, not a comparison, so it composes with a mana-value bound
		// (Maralen); the kernel reads it against the matcher's source
		// (lc.exwOK / lc.exwStatic)
		c.Flags |= fExiledWith
		return nil
	case "wasCast", "!wasCast", "wasCastFromGraveyard", "wasCastFromYourGraveyard":
		// gorge WasCast / ObjectWasCastFromGraveyard: the permanent entering as
		// the spell it was cast as. Read only where the kernel sees that spell
		// still resolving: a battlefield-entering trigger's ValidCard(s)$
		// (lc.wasCastOK)
		n := wasCastAny
		switch p {
		case "!wasCast":
			n = wasCastNot
		case "wasCastFromGraveyard":
			n = wasCastGrave
		case "wasCastFromYourGraveyard":
			n = wasCastYourGrave
		}
		if c.CmpStat == cmpWasCast && c.CmpN == wasCastWebSling && n == wasCastAny {
			// wasCast beside webSlinged, which already implies the cast
			return nil
		}
		if c.CmpStat == cmpWasCastSpell && n == wasCastAny {
			// wasCast beside a spell word that already implies the cast
			// (wasCastFromExile) or reads it (!wasCastFromYourHand: cast,
			// from anywhere but a hand)
			if c.CmpN == spellHand|spellNot {
				c.CmpN = spellNotHand
			}
			return nil
		}
		return c.setCmp(cmpWasCast, 0, n, "", "")
	case "wasCastFromYourHand", "!wasCastFromYourHand", "wasCastFromTheirHand", "!wasCastFromTheirHand", "wasCastFromYourHandByYou", "wasCastFromExile":
		// gorge's cast-provenance words (rules castProvenanceAdmits), read on
		// a spell on the stack: lc.spellProvOK says the filter is a SpellCast
		// ValidCard$ or a spell's own ConditionPresent$
		n := spellHand
		switch p {
		case "!wasCastFromYourHand", "!wasCastFromTheirHand":
			// gorge reads wasCastFromTheirHand as a cast from a hand, any
			// caster (cast_provenance.go): the same predicate as YourHand
			n = spellHand | spellNot
		case "wasCastFromYourHandByYou":
			n = spellHandBy
		case "wasCastFromExile":
			n = spellExile
		}
		c.TheirHand = c.TheirHand || p == "wasCastFromTheirHand" || p == "!wasCastFromTheirHand"
		if c.CmpStat == cmpWasCast && c.CmpN == wasCastAny {
			c.CmpStat, c.CmpN = cmpNone, 0 // wasCast+<word>: the word implies the cast
			if n == spellHand|spellNot {
				n = spellNotHand
			}
		}
		return c.setCmp(cmpWasCastSpell, 0, n, "", "")
	case "CastSa Spell.Mayhem", "!CastSa Spell.Mayhem":
		// gorge castSaTokens FlagMayhem (cast_provenance.go): the resolving
		// spell was cast for its mayhem cost; a stack copy never carries it.
		// Read where the cast-provenance words read (lc.spellProvOK)
		n := spellMayhem
		if p[0] == '!' {
			n |= spellNot
		}
		return c.setCmp(cmpWasCastSpell, 0, n, "", "")
	case "webSlinged":
		// gorge webSlingedAdmits (CR 702.188): the permanent entering was
		// cast for its web-slinging cost; a stack copy never carries it
		// (CR 707.10). Implies the cast, so it composes with a bare wasCast.
		// Read as the permanent enters from its own spell (kernel
		// wasCastRead bit 16; lc.wasCastOK); a spell-side read refuses in
		// lc.filter. "!webSlinged" stays an unknown property
		if c.CmpStat == cmpWasCast && c.CmpN == wasCastAny {
			c.CmpN = wasCastWebSling
			return nil
		}
		return c.setCmp(cmpWasCast, 0, wasCastWebSling, "", "")
	case "CastSa Spell.ManaSpent EQ0":
		// gorge castSaTokens {ManaSpent EQ0, tag -1}: the object was cast
		// (a PutOnStack window exists) and its total spend is 0. Read as the
		// permanent enters from its own spell (kernel wasCastRead bit 8;
		// lc.wasCastOK); a spell-side read refuses in lc.filter
		return c.setCmp(cmpWasCast, 0, wasCastSpentZero, "", "")
	case "CastSa Spell.MayPlaySource":
		// gorge's castSaTokens FlagMayPlay read (cast_provenance.go): the
		// cast went through a may-play permission. gorge documents the flag
		// as an approximation of Forge's host-specific MayPlaySource — it
		// does not record WHICH host granted the cast, and a stack copy
		// never carries it: the kernel copies the flag-only read
		// Read where the cast-provenance words read
		// (lc.spellProvOK; lower.go refuses it elsewhere).
		return c.setCmp(cmpWasCastSpell, 0, spellMayPlay, "", "")
	case "AdventureCard":
		// gorge wordAdventureCard: the object's card has the Adventure layout
		return c.setCmp(cmpAdventureCard, 0, 0, "", "")
	case "MonoColor":
		// gorge wordMonoColor: exactly one current colour
		return c.setCmp(cmpMonoColor, 0, 0, "", "")
	case "!withDefender", "!withFlying", "!withDeathtouch", "!withTrample", "!withHaste", "!withMenace", "!withVigilance", "!withReach",
		"!withLifelink", "!withIndestructible", "!withFirstStrike", "!withDoubleStrike", "!withShadow":
		// gorge's '!' negation of the with<Keyword> registry
		// (objectHasKeyword over the derived keywords)
		return c.setCmp(cmpLacksKw, 0, withKwBit(p[1:]), "", "")
	case "withoutDefender", "withoutDeathtouch", "withoutTrample", "withoutHaste", "withoutMenace", "withoutVigilance", "withoutReach",
		"withoutLifelink", "withoutIndestructible", "withoutFirstStrike", "withoutDoubleStrike", "withoutShadow":
		// gorge's without<Keyword> registry entry is exactly !with<Keyword>
		// (filter.go: !objectHasKeyword; Gingerbrute's withoutHaste;
		// ). withoutFlying keeps its own flag.
		return c.setCmp(cmpLacksKw, 0, withKwBit("with"+strings.TrimPrefix(p, "without")), "", "")
	case "IsMonstrous", "monstrous":
		// gorge wordIsMonstrous: Object.Monstrous, CR 701.37b's designation
		return c.setCmp(cmpMonstrous, 0, 0, "", "")
	}
	if p == "powerGTbasePower" {
		// gorge numericPred: current power > base power (CR 613.4)
		return c.setCmp(cmpPowerGTBase, 0, 0, "", "")
	}
	if p == "powerLTtoughness" {
		return c.setCmp(cmpPowerLTToughness, opLT, 0, "", "")
	}
	if strings.HasPrefix(p, "named") {
		// named<X> (gorge wordNamed, sharesName; CR 201.2): Name is X
		// normalised as gorge's nameArg does (';' -> ',', '_' -> ' ').
		// The lowering (lowerCtx.filter) clears fNamedAsSource: every
		// named<X>, the source's own name included, is a literal clause,
		// Pred = PredNamed
		c.Flags |= fNamedAsSource
		c.Name = strings.NewReplacer(";", ",", "_", " ").Replace(strings.TrimPrefix(p, "named"))
		return nil
	}
	if strings.HasPrefix(p, "counters_") {
		// counters_GE1_P1P1
		parts := strings.Split(p, "_")
		if len(parts) != 3 || len(parts[1]) < 3 {
			return fmt.Errorf("counter predicate %q", p)
		}
		op, ok := opNamesCmp[parts[1][:2]]
		n, err := strconv.Atoi(parts[1][2:])
		if !ok || err != nil {
			return fmt.Errorf("counter predicate %q", p)
		}
		return c.setCmp(cmpCounters, op, n, "", parts[2])
	}
	for _, st := range []struct {
		pre  string
		stat uint8
	}{{"power", cmpPower}, {"cmc", cmpCMC}, {"toughness", cmpToughness}} {
		if strings.HasPrefix(p, st.pre) && len(p) > len(st.pre)+2 {
			rest := p[len(st.pre):]
			op, ok := opNamesCmp[rest[:2]]
			if !ok {
				return fmt.Errorf("compare %q", p)
			}
			if n, err := strconv.Atoi(rest[2:]); err == nil {
				return c.setCmp(st.stat, op, n, "", "")
			}
			return c.setCmp(st.stat, op, 0, rest[2:], "")
		}
	}
	if col, ok := colorWords[p]; ok {
		// gorge's colour predicate (the object's current colours); the
		// kernel's Filter.Colors is any-of, so a clause takes one colour
		if c.Colors != "" {
			return fmt.Errorf("two colours in one clause")
		}
		c.Colors = col
		return nil
	}
	if strings.HasPrefix(p, "non") {
		w := p[3:]
		if col, ok := colorWords[w]; ok {
			if !strings.Contains(c.NotColors, col) {
				c.NotColors += col
			}
			return nil
		}
		if bits, ok := typeBits[w]; ok && bits&tAnyCard != 0 && w != "Card" && w != "Permanent" {
			c.NotTypes |= bits
			return nil
		}
		if knownSub(w) {
			return c.addNotSub(w)
		}
		return fmt.Errorf("unknown negation %q", p)
	}
	if w, ok := strings.CutPrefix(p, "!"); ok {
		// gorge matchPredicate's '!' negation of a card-type word (the
		// Animate Defined$ Targeted.!Creature qualifier); any other
		// negated word falls through to "unknown property"
		if bits, ok := typeBits[w]; ok && bits&tAnyCard != 0 && w != "Card" && w != "Permanent" {
			c.NotTypes |= bits
			return nil
		}
		if w == "hasKeywordLandwalk:Island" {
			// gorge's '!' over the parameterised exact-keyword word (only
			// the Island parameter is read; the kernel finds the Island
			// subtype in the matchup table, kop.W7LacksIslandwalk)
			return c.setCmp(cmpWord7, 0, int(kop.W7LacksIslandwalk), "", "")
		}
		if w == "IsCommander" {
			// gorge's negated IsCommander: lowered to "always holds" on the
			// assumption that no object is a commander (kop.W7IsCommander);
			// the cmd5 pool guard reads the counter
			noteCmdShortcut(cmdShortcutNegatedWord)
			return nil
		}
		if w == "BottomLibrary" {
			return c.setCmp(cmpWord7, 0, int(kop.W7NotBottomLibrary), "", "") // lower_filterwords.go
		}
		if i, ok := notF2Bit(w); ok {
			return c.setCmp(cmpNotF2, 0, i, "", "") // lower_filterwords.go
		}
		if knownSub(w) {
			// gorge's '!' negation of a subtype word, the same as non<sub>
			return c.addNotSub(w)
		}
	}
	if ok, err := c.noNameTwinGrave(p); ok {
		return err // lower_filterwords.go
	}
	if ok, err := c.faceUpDisguise(p); ok {
		return err // lower_filterwords.go
	}
	if p == "sharesNameWith Remembered" {
		// gorge sharesNameWith Remembered: any remembered object with the
		// candidate's printed name (Tainted Pact's repeat gate)
		return c.setCmp(cmpSameNameRem, 0, 0, "", "")
	}
	if n, ok := filterWordPred(p); ok {
		return c.setCmp(cmpWord7, 0, n, "", "") // lower_filterwords.go
	}
	if p == "faceDown" {
		return c.setCmp(cmpFaceDown, 0, 0, "", "") // lower_filterwords.go
	}
	if p == "Legendary" {
		// The supertype (gorge hasType: the object's current types). The
		// kernel's Filter.Types is any-of with TLegendary riding in it, so
		// it expresses Legendary only with no other type requirement; the
		// Permanent head is "legendary and not instant/sorcery" (every
		// legendary card is one or the other).
		switch c.Types {
		case 0, tLegendary:
			c.Types = tLegendary
		case tPermanent:
			c.Types = tLegendary
			c.NotTypes |= tInstant | tSorcery
		default:
			// a type head: the type AND legendary (the F2Legendary bit)
			c.Flags |= fLegendary
		}
		return nil
	}
	if bits, ok := typeBits[p]; ok && bits&tAnyCard != 0 {
		// "Card.Creature": a type word as a property narrows the types.
		if c.Types&tLegendary != 0 {
			// "Legendary.Creature": the type AND legendary (F2Legendary)
			c.Types &^= tLegendary
			c.Flags |= fLegendary
		}
		if c.TypesAll != 0 {
			// a conjunction already: a further single type joins it
			// ("Artifact.Creature+Land"); a set must contain it
			if bits&(bits-1) != 0 {
				if c.TypesAll&bits != c.TypesAll {
					return fmt.Errorf("type set %q beside the type conjunction", p)
				}
				return nil
			}
			c.TypesAll |= bits
			c.Types |= bits
			return nil
		}
		switch {
		case c.Types == 0, c.Types&bits == c.Types:
			// "Card.Creature", "Creature.Permanent": the narrower set
			if c.Types == 0 {
				c.Types = bits
			}
		case c.Types&bits == bits:
			// "Permanent.Creature": B within the head's any-of set
			c.Types = bits
		case c.Types&(c.Types-1) == 0 && bits&(bits-1) == 0:
			// "Land.Artifact", "Artifact.Creature": a card with BOTH
			// types (gorge's type head AND the type property, filter_word
			// wordType: an artifact land, an artifact creature). The
			// any-of Filter.Types keeps the union (the hot pre-check is a
			// superset test); PredTypesAll carries the conjunction
			// (typesall; before it, kcycle refused the pair, and before
			// that the intersection (0) matched every card)
			c.TypesAll = c.Types | bits
			c.Types |= bits
		default:
			// a disjoint type SET on either side ("Permanent" less the
			// property's type cannot happen; "Instant.Permanent" is empty)
			return fmt.Errorf("type property %q beside a disjoint type set", p)
		}
		return nil
	}
	if knownSub(p) {
		return c.addSubtype(p)
	}
	return fmt.Errorf("unknown property %q", p)
}

// addSubtype fills the clause's first free subtype slot; a third refuses.
func (c *lClause) addSubtype(p string) error {
	switch {
	case c.Subtype == "":
		c.Subtype = p
	case c.Subtype2 == "":
		c.Subtype2 = p
	default:
		return fmt.Errorf("three subtypes")
	}
	return nil
}

func (c *lClause) setWho(dst *uint8, v uint8) error {
	if *dst != whoAny && *dst != v {
		return fmt.Errorf("conflicting controller/owner")
	}
	*dst = v
	return nil
}

func (c *lClause) setCmp(stat, op uint8, n int, x, counter string) error {
	if c.CmpStat != cmpNone {
		return fmt.Errorf("two comparisons in one clause")
	}
	c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter = stat, op, n, x, counter
	return nil
}

// filterSubtypes lists the subtypes a filter names (for the pool subtype
// table).
func (f *lFilter) subtypes() []string {
	var out []string
	if f == nil {
		return nil
	}
	for _, c := range f.Clauses {
		if c.Subtype != "" {
			out = append(out, c.Subtype)
		}
		if c.Subtype2 != "" {
			out = append(out, c.Subtype2)
		}
		if c.NotSubtype != "" {
			out = append(out, c.NotSubtype)
		}
		for _, w := range c.MoreNotSubtype {
			if w != "" {
				out = append(out, w)
			}
		}
	}
	sort.Strings(out)
	return out
}

// cmpStatName names a CmpStat for a refusal reason.
func cmpStatName(st uint8) string {
	names := [...]string{"none", "power", "cmc", "counters", "toughness", "hasX", "powerLTtoughness", "multicolor", "historic", "equipped", "enchanted", "keyword", "attached", "activectrl"}
	if int(st) < len(names) {
		return names[st]
	}
	return "stat"
}

// addNotSub records one more negated subtype: the first in NotSubtype, up to
// four more in MoreNotSubtype (the kernel filter's NotSub and NotSubs). A word
// already negated adds nothing.
func (c *lClause) addNotSub(w string) error {
	if c.NotSubtype == "" {
		c.NotSubtype = w
		return nil
	}
	if c.NotSubtype == w {
		return nil
	}
	for i, m := range c.MoreNotSubtype {
		if m == w {
			return nil
		}
		if m == "" {
			c.MoreNotSubtype[i] = w
			return nil
		}
	}
	return fmt.Errorf("six negated subtypes")
}
