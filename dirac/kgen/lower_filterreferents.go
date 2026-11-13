package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// triggeredTargetCtrl rewrites the filter word "ControlledBy
// TriggeredTarget" (gorge controlReferentPlayers: the trigger's target
// player) to trigPlayerCtrlWord (kop.PredTrigPlayerCtrl: the trigger
// entry's trigger player, the damaged seat at any seat count, dirac
// dmgSeat) inside a combat-damage-to-a-player trigger whose source is the
// controller's own (Self or YouCtrl). Anywhere else the word is left for
// the parser to refuse.
func (lc *lowerCtx) triggeredTargetCtrl(expr string) string {
	const word = "ControlledBy TriggeredTarget"
	if !strings.Contains(expr, word) || lc.ab == nil || lc.ab.Trigger == nil {
		return expr
	}
	t := lc.ab.Trigger
	if t.Event != "DamageDone" || t.Combat != 1 || t.F2 == nil || t.F2.Players == 0 || len(t.F2.Clauses) != 0 || t.F == nil || len(t.F.Clauses) != 1 {
		return expr
	}
	if c := t.F.Clauses[0]; c.Flags&fSelf == 0 && c.Ctrl != whoYou {
		return expr
	}
	for _, alt := range strings.Split(expr, ",") {
		typ, rest, ok := strings.Cut(strings.TrimSpace(alt), ".")
		if !ok || rest != word || strings.ContainsAny(typ, "+_ ") || typ == "Card" {
			// the predicate is the clause's one comparison, read on the
			// battlefield: any other shape keeps the two-player reading
			lc.twoSeat("ControlledBy TriggeredTarget read as OppCtrl")
			return strings.ReplaceAll(expr, word, "OppCtrl")
		}
	}
	return strings.ReplaceAll(expr, word, trigPlayerCtrlWord)
}

// trigPlayerCtrlWord is the filter word triggeredTargetCtrl writes: kgen's
// own spelling, no Forge property.
const trigPlayerCtrlWord = "kgenTrigPlayerCtrl"

// sameNameT0Filter is Maelstrom Pulse's DestroyAll ValidCards$
// "TargetedCard.Self,Permanent.NotDefinedTargeted+sharesNameWith Targeted"
// (the target and every other permanent with its name), lowered whole as
// one PredSameNameT0 clause over permanents. It needs exactly one object
// target slot in the chain.
func (lc *lowerCtx) sameNameT0Filter() (*lFilter, error) {
	if len(lc.ab.Targets) != 1 || lc.ab.Targets[0].F == nil || lc.ab.Targets[0].F.Players != 0 {
		return nil, refuse("sharesNameWith Targeted without one object target")
	}
	return &lFilter{Clauses: []lClause{{Types: tPermanent, CmpStat: cmpSameNameT0}}}, nil
}

// Two ChangeZoneAll ChangeType$ spellings of "the target and every other
// card with its name". The graveyard one (Rat King, Verminister) lowers to
// one PredSameNameT0 card clause; the Legions to Ashes one is the battlefield
// token sweep (sameNameT0TokenFilter).
const (
	czaSameNameGrave  = "TargetedCard.Self,Card.NotDefinedTargeted+sharesNameWith Targeted"
	czaSameNameTokens = "TargetedCard.Self,Card.NotDefinedTargeted+token+sharesNameWith Targeted+ControlledBy TargetedController"
)

// changeZoneAllFilter reads a ChangeZoneAll's ChangeType$, routing the
// same-name sweep spellings to sameNameT0CardFilter.
func (lc *lowerCtx) changeZoneAllFilter(ps *params) (*lFilter, error) {
	switch ct := ps.str("ChangeType"); ct {
	case czaSameNameGrave:
		return lc.sameNameT0CardFilter(ps)
	case czaSameNameTokens:
		return lc.sameNameT0TokenFilter(ps)
	default:
		return lc.filter(ct)
	}
}

// sameNameT0CardFilter lowers czaSameNameGrave: a sweep of your graveyard
// (Origin$ Graveyard, Defined$ You: gorge changeZoneAllPlayers scopes the
// sweep to your graveyard) taking target 0, a card in that graveyard, and
// every other card there with its name, as one Card.YouOwn clause carrying
// PredSameNameT0 (the kernel's card form, State.sameNameT0Card).
func (lc *lowerCtx) sameNameT0CardFilter(ps *params) (*lFilter, error) {
	if strings.TrimSpace(ps.str("Origin")) != "Graveyard" || strings.TrimSpace(ps.str("Defined")) != "You" {
		return nil, refuse("sharesNameWith Targeted sweep beyond your own graveyard")
	}
	if len(lc.ab.Targets) != 1 || lc.ab.Targets[0].F == nil || lc.ab.Targets[0].F.Players != 0 || lc.ab.Targets[0].Zone != zGraveyard {
		return nil, refuse("sharesNameWith Targeted sweep without one graveyard card target")
	}
	f, err := lc.filter("Card.YouOwn")
	if err != nil || len(f.Clauses) != 1 {
		return nil, refuse("sharesNameWith Targeted: Card.YouOwn")
	}
	ps.consume("Defined")
	f.Clauses[0].CmpStat = cmpSameNameT0
	return f, nil
}

// sameNameT0TokenFilter lowers czaSameNameTokens (Legions to Ashes): a
// battlefield sweep taking target 0 itself and every token with its name that
// the target's controller controls, as two OR'd clauses: A is Permanent
// targetedBy (the spell's one target), B is Permanent.token+OppCtrl reading
// PredSameNameT0 (the target, or a permanent with its name; State.sameNameT0).
//
// "ControlledBy TargetedController" is OppCtrl only under this gate: dirac is
// a two-player engine, the one target is an opponent-controlled permanent
// (every clause of its ValidTgts reads OppCtrl), and the spell resolves only
// while that target is legal (CR 608.2b), so its controller is then the
// caster's opponent. The sweep collects its whole match mask before it moves
// anything (czallBattleHalf), so the target is still on the battlefield while
// every candidate is matched.
func (lc *lowerCtx) sameNameT0TokenFilter(ps *params) (*lFilter, error) {
	const word = "same-name tokens ControlledBy TargetedController"
	if strings.TrimSpace(ps.str("Origin")) != "Battlefield" || ps.peek("Defined") != "" {
		return nil, refuse("%s beyond a battlefield sweep", word)
	}
	if len(lc.ab.Targets) != 1 || lc.ab.Targets[0].F == nil || lc.ab.Targets[0].F.Players != 0 || len(lc.ab.Targets[0].F.Clauses) == 0 {
		return nil, refuse("%s without one opponent-controlled target", word)
	}
	for _, c := range lc.ab.Targets[0].F.Clauses {
		if c.Ctrl != whoOpp {
			return nil, refuse("%s without one opponent-controlled target", word)
		}
	}
	a, err := lc.filter("Permanent.targetedBy")
	if err != nil || len(a.Clauses) != 1 {
		return nil, refuse("%s: Permanent.targetedBy", word)
	}
	b, err := lc.filter("Permanent.token+OppCtrl")
	if err != nil || len(b.Clauses) != 1 {
		return nil, refuse("%s: Permanent.token+OppCtrl", word)
	}
	b.Clauses[0].CmpStat = cmpSameNameT0
	lc.twoSeat("ControlledBy TargetedController read as OppCtrl")
	return &lFilter{Clauses: []lClause{a.Clauses[0], b.Clauses[0]}}, nil
}

// ownerRoleF2 reads an Owner role (OwnedBy TriggeredDefendingPlayer, OwnedBy
// Targeted) off the battlefield. gorge's
// controlReferentPlayers (op "OwnedBy") matches a card whose OWNER is one of
// the referent's players; off the battlefield a card has no controller, so the
// owner is the only relation (the kernel's card matcher passes the card's
// owner to ctrlRoleF2, which reads the same F2DefendingCtrl / F2TgtPlayerCtrl
// bits a battlefield Ctrl role sets). On the battlefield the role would mean
// "owned by" while the perm matcher reads the same bit as "controlled by", so
// it refuses; so does a role on both Ctrl and Owner (two readings of one bit).
func ownerRoleF2(c lClause, zone kop.Zone) (kop.FilterFlags2, error) {
	if zone == kop.ZoneBattlefield || zone == kop.ZoneStack {
		return 0, refuse("OwnedBy %s on the battlefield or stack", ownerRoleWord(c.Owner))
	}
	if c.Ctrl == whoTgtPlayer || c.Ctrl == whoTgtOrCtrl || c.Ctrl == whoDefending || c.Ctrl == whoHitBySrc {
		return 0, refuse("OwnedBy %s beside a controller role", ownerRoleWord(c.Owner))
	}
	if c.Owner == whoTgtPlayer {
		return kop.F2TgtPlayerCtrl, nil
	}
	return kop.F2DefendingCtrl, nil
}

func ownerRoleWord(w uint8) string {
	if w == whoTgtPlayer {
		return "Targeted"
	}
	return "TriggeredDefendingPlayer"
}

// referentGate admits the words only where their referent
// is bound exactly as the existing twin's is; it reads the raw filter
// text, so every spelling and alternative is covered.
//
//   - NotDefinedTriggeredAttacker (and its !TriggeredAttacker spelling) is
//     the negation of TriggeredAttacker, the
//     attacking creature an Attacks trigger captured (gorge
//     notDefinedTriggerReferent). The kernel's Attacks trigger carries the
//     attacker as trigObj (combat.go objTrigGen over the declared attacker's
//     obj), which F2NotTriggerObj reads; outside an Attacks trigger's body
//     that object is some other event's. sharesCreatureTypeWith
//     TriggeredAttacker (kop.W7SharesCrTrigObj) reads the same object
//     under the same gate.
//   - AttachedTo Targeted is gorge's attachedToReferentTargeted, the same
//     referent as ParentTarget: target 0 of the entry. chooseCard sets
//     attachedTgtOK only when the node declares no target slot of its own.
func (lc *lowerCtx) referentGate(expr string) error {
	if strings.Contains(expr, "NotDefinedTriggeredAttacker") || strings.Contains(expr, "!TriggeredAttacker") ||
		strings.Contains(expr, "sharesCreatureTypeWith TriggeredAttacker") {
		tr := (*lTrigger)(nil)
		if lc.ab != nil {
			tr = lc.ab.Trigger
		}
		if tr == nil || tr.Event != "Attacks" || !lc.trigBody || lc.granted > 0 {
			return refuse("filter %q: NotDefinedTriggeredAttacker outside an Attacks trigger", expr)
		}
	}
	if strings.Contains(expr, "AttachedTo Targeted") && !lc.attachedTgtOK {
		return refuse("filter %q: AttachedTo Targeted beside the node's own target", expr)
	}
	return nil
}

// targetedPlayerView reads a ThisTargetedPlayer$Valid spec, whose "You" is
// the targeted player (gorge evalPlayerRefProperty), as that player's
// battlefield at any seat count: a YouCtrl clause becomes the
// TargetedPlayerCtrl role (kop.F2TgtPlayerCtrl: the controller is a player
// target of the resolving entry), a clause with no controller stays. It is
// exact when the entry has the one player slot, as the entry's other targets
// are objects. nil means the spec reads another role (an opponent of the
// targeted player, an owner, a source-relative word) or the ability has a
// second player slot, and the caller keeps the
// one-opponent view, which marks the card for the one-opponent game.
func (lc *lowerCtx) targetedPlayerView(f *lFilter) *lFilter {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return nil
	}
	players := 0
	for _, t := range lc.ab.Targets {
		if t.F != nil && t.F.Players != 0 {
			players++
		}
	}
	if players != 1 {
		return nil
	}
	out := &lFilter{}
	for _, c := range f.Clauses {
		if c.Flags != 0 || c.CmpX != "" || c.Owner != whoAny {
			return nil
		}
		switch c.Ctrl {
		case whoAny:
		case whoYou:
			c.Ctrl = whoTgtPlayer
		default:
			return nil
		}
		out.Clauses = append(out.Clauses, c)
	}
	return out
}

// offFieldCardSlot reports a target slot of zone z (lTarget.Zone) that names
// a card off the battlefield, never a player or a permanent: the slot an
// OwnedBy TargetedController role reads (kop.CtrlTgtOwner).
func offFieldCardSlot(z int) bool {
	return z == zGraveyard || z == zExile || z == zHand || z == zLibrary
}

// anyOtherTarget reports f's object half is exactly "any other target"'s:
// one bare Other clause each for Creature, Planeswalker and Battle (gorge
// Creature.Other,Planeswalker.Other,Battle.Other beside Player), the
// object types the kernel's TgtAny walk admits.
func anyOtherTarget(f *lFilter) bool {
	if len(f.Clauses) != 3 {
		return false
	}
	var types uint16
	for _, c := range f.Clauses {
		if c != (lClause{Types: c.Types, Flags: fOther}) {
			return false
		}
		types |= c.Types
	}
	return types == tCreature|tPlaneswalker|tBattle
}
