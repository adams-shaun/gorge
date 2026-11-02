package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// triggeredTargetCtrl rewrites the filter word "ControlledBy
// TriggeredTarget" (gorge controlReferentPlayers: the trigger's target
// player) to OppCtrl inside a combat-damage-to-a-player trigger whose
// source is the controller's own (Self or YouCtrl): two-player, the player
// such a creature deals combat damage to is the controller's opponent.
// Anywhere else the word is left for the parser to refuse.
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
	return strings.ReplaceAll(expr, word, "OppCtrl")
}

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
	if c.Ctrl == whoTgtPlayer || c.Ctrl == whoDefending || c.Ctrl == whoHitBySrc {
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
//   - NotDefinedTriggeredAttacker is the negation of TriggeredAttacker, the
//     attacking creature an Attacks trigger captured (gorge
//     notDefinedTriggerReferent). The kernel's Attacks trigger carries the
//     attacker as trigObj (combat.go objTrigGen over the declared attacker's
//     obj), which F2NotTriggerObj reads; outside an Attacks trigger's body
//     that object is some other event's.
//   - AttachedTo Targeted is gorge's attachedToReferentTargeted, the same
//     referent as ParentTarget: target 0 of the entry. chooseCard sets
//     attachedTgtOK only when the node declares no target slot of its own.
func (lc *lowerCtx) referentGate(expr string) error {
	if strings.Contains(expr, "NotDefinedTriggeredAttacker") {
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
