package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stackToZone lowers ChangeZone Origin$ Stack TargetType$ Spell (gorge
// effChangeZone's targeted stack arm: Hullbreaker Horror's "return target
// spell you don't control to its owner's hand", Mindbreak Trap's "exile any
// number of target spells"): the countered-spell move of OpCounter with
// kop.CtrDestMove, so a spell that can't be countered still moves. Hand takes
// one target; Exile takes the any-number shape only, so a one-target stack
// exile (Gale's Redirection, which also carries RememberChanged) keeps its
// earlier refusal. A ValidTgts$ Card.inZoneStack with no TargetType$ names
// the same stack object (gorge SpecTargetsStack; Take It Back).
func (lc *lowerCtx) stackToZone(ps *params) error {
	dest := map[string]int{"Hand": kop.CtrDestHand, "Exile": kop.CtrDestExile}[ps.str("Destination")]
	if tt := ps.str("TargetType"); tt != "Spell" && (tt != "" || ps.str("ValidTgts") != "Card.inZoneStack") || ps.str("TgtZone") != "Stack" || dest == 0 {
		return refuse("ChangeZone Origin$ Stack shape")
	}
	ps.consume("Origin", "Destination", "TgtZone", "TargetType", "TgtPrompt")
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	lt := lTarget{F: f, Zone: zStack, Spell: true}
	if dest == kop.CtrDestExile {
		anyNumber, err := lc.stackAnyNumber(ps, &lt)
		if err != nil {
			return err
		}
		if !anyNumber {
			return refuse("ChangeZone Origin$ Stack shape")
		}
	}
	t := lc.addTarget(lt)
	lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, C: dest | kop.CtrDestMove})
	return nil
}

// stackAnyNumber reads Mindbreak Trap's "any number of target spells":
// TargetMin$ 0 with a TargetMax$ SVar of Count$ValidStack Card. gorge counts
// every object on the stack (cbCard matches any; count_body_zone.go scans
// the one shared stack once), and the casting spell is already there when its
// targets are announced (CR 601.2a, 601.2c), so the bound always exceeds the
// spells the slot can take (a spell cannot target itself, CR 115.5): the
// slot is "up to" a maximum that never binds, and choosing every candidate
// still ends on gorge's done-only ask. The kernel holds that as the widest
// literal bound (tgtBoundsV's 254). The bool reports the exact shape; no
// TargetMin$ / TargetMax$ (Gale's Redirection) or any other bound refuses.
func (lc *lowerCtx) stackAnyNumber(ps *params, lt *lTarget) (bool, error) {
	tmin, hasMin := ps.get("TargetMin")
	tmax, hasMax := ps.get("TargetMax")
	if !hasMin && !hasMax {
		return false, nil
	}
	if strings.TrimSpace(tmin) != "0" || strings.TrimSpace(lc.face.SVars[strings.TrimSpace(tmax)]) != "Count$ValidStack Card" {
		return false, refuse("ChangeZone Origin$ Stack TargetMin$ %q TargetMax$ %q: only 0 up to Count$ValidStack Card is read", tmin, tmax)
	}
	lt.Min, lt.Max = cst(0), cst(254)
	return true, nil
}
