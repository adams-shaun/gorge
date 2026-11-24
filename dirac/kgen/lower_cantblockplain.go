package kgen

import "strings"

// cantBlockPlainRemembered recognizes the second CantBlockBy shape of an
// Effect whose static carries no ValidBlocker$ at all: the Unblockable pump
// template ("target creature can't be blocked this turn", Cephalid Pathmage;
// Ghostform, Runed Arch, Open into Wonder's X targets) whose remembered
// objects are the chain's own targets. gorge's restriction (rules
// combat/restrictions.go BlockRestricted) is over the registration's
// Remembered ids, with ValidAttacker$ Card.IsRemembered naming the attacker
// and no ValidBlocker$ forbidding every blocker; the kernel grant is the
// object's Unblockable keyword, which lapses when the object leaves the
// battlefield and at the turn's cleanup.
//
// A target that left the battlefield and came back is the one divergence:
// gorge's Remembered set keeps the object id across a move (only
// ForgetOnMoved$/ExileOnMoved$ sweep it, and a no-rider grant prunes a
// may-play card the moment it leaves its zone), so a blinked target is
// restricted again on return. The keyword grant is not. The shape therefore
// requires ForgetOnMoved$ Battlefield, which forgets the remembered card on
// its departure and matches the lapsed grant (CR 400.7: a new object).
// A UntilYourNextTurn duration is the override record's business
// (cantBlockGrant), and stays refused here until a carrier asks for it.
func (lc *lowerCtx) cantBlockPlainRemembered(ps *params, obj lRef) bool {
	if obj.Kind != rTarget {
		return false
	}
	switch ps.str("RememberObjects") {
	case "Targeted", "ThisTargetedCard":
	default:
		return false
	}
	if _, ok := ps.m["Duration"]; ok {
		return false
	}
	if f, ok := ps.m["ForgetOnMoved"]; !ok || strings.TrimSpace(f) != "Battlefield" {
		return false
	}
	return true
}
