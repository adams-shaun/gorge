package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachCardsDamage lowers a RepeatEach card loop whose body is
// "the repeated card deals N damage to its controller" exactly:
//
//	DB$ RepeatEach | RepeatCards$ <filter> | RepeatSubAbility$ <body> | DamageMap$ True
//	<body>: DB$ DealDamage | Defined$ RememberedController | DamageSource$ Remembered | NumDmg$ <N>
//
// (Rakdos Charm's third mode: "Each creature deals 1 damage to its
// controller"). gorge effRepeatEach binds each card as Remembered and runs the
// DealDamage once per card: the card is the damage source (DamageSource$
// Remembered, one rider), its controller the recipient (Defined$
// RememberedController), every pass inside the one damage batch DamageMap$
// opens around the loop. The kernel's DealDamageFrom player mode with
// kop.DmgFromEachCtrl deals from RefEach to that card's controller, inside
// OpRepeatEach's batch. The body moves no card, so ChangeZoneTable$ is not
// read; without DamageMap$ True gorge deals each pass as its own batch, which
// the loop's one batch does not model, so it refuses. A constant N only.
func (lc *lowerCtx) repeatEachCardsDamage(ps *params, in lInstr, body *cards.SA) error {
	if !ps.is("DamageMap", "True") {
		return refuse("RepeatEach cards DealDamage body without DamageMap$ True")
	}
	bp := newParams(body.Params)
	for _, w := range [][2]string{{"Defined", "RememberedController"}, {"DamageSource", "Remembered"}} {
		if bp.str(w[0]) != w[1] {
			return refuse("RepeatEach cards DealDamage body %s$ %q", w[0], bp.str(w[0]))
		}
	}
	n, err := lc.amount(bp, "NumDmg", 0)
	if err != nil {
		return err
	}
	if !n.isConst() {
		return refuse("RepeatEach cards DealDamage body NumDmg$ %q: not a constant", bp.str("NumDmg"))
	}
	bp.consume("SpellDescription", "StackDescription")
	if left := bp.unread(); len(left) > 0 {
		return refuse("RepeatEach cards DealDamage body: unread params %v", left)
	}
	in.Skip = 1
	lc.emit(in)
	lc.emit(lInstr{Op: "DealDamageFrom", Obj: lRef{Kind: rEach}, A: n, CS: "eachctrl", Note: "the repeated card deals A to its controller"})
	return nil
}
