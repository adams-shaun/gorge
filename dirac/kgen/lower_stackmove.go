package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// stackToZone lowers ChangeZone Origin$ Stack TargetType$ Spell (gorge
// effChangeZone's targeted stack arm: Hullbreaker Horror's "return target
// spell you don't control to its owner's hand"): the countered-spell move
// of OpCounter with kop.CtrDestMove, so a spell that can't be countered
// still moves. Only the hand destination is lowered. A ValidTgts$
// Card.inZoneStack with no TargetType$ names the same stack object (gorge
// SpecTargetsStack; Take It Back).
func (lc *lowerCtx) stackToZone(ps *params) error {
	if tt := ps.str("TargetType"); tt != "Spell" && (tt != "" || ps.str("ValidTgts") != "Card.inZoneStack") || ps.str("TgtZone") != "Stack" || ps.str("Destination") != "Hand" {
		return refuse("ChangeZone Origin$ Stack shape")
	}
	ps.consume("Origin", "Destination", "TgtZone", "TargetType", "TgtPrompt")
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	lc.emit(lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, C: kop.CtrDestHand | kop.CtrDestMove})
	return nil
}
