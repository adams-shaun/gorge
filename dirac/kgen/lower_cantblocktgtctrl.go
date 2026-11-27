package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// cantBlockTargetedController lowers a no-ValidTgts Effect that remembers the
// controller of the chain's own target (RememberObjects$ TargetedController)
// and delivers one CantBlockBy over that player's creatures: The Motherlode,
// Excavator's reflexive "creatures that player controls without flying can't
// block this turn" after the Destroy of its targeted land. gorge's
// effectRememberedPlayers reads the controller of each captured target LIVE at
// the Effect's resolution (a land the parent Destroy just sent to the graveyard
// answers its owner, CR 400.7), the kernel's RefTargetCtrlLive. The emblem is
// created for that player, so the static's ValidBlocker$ ...+RememberedPlayerCtrl
// clause reads as its own YouCtrl, exactly as Peace in Our Time's
// (lower_effectcantattackplayer.go). ok reports the Effect had this shape.
func (lc *lowerCtx) cantBlockTargetedController(ps *params, ss []cards.Static) (bool, error) {
	if ps.str("RememberObjects") != "TargetedController" || len(ss) != 1 || ss[0].Mode != "CantBlockBy" {
		return false, nil
	}
	s := ss[0]
	const want = "+RememberedPlayerCtrl"
	vb := s.Params["ValidBlocker"]
	if s.Params["ValidAttacker"] != "Creature" || len(vb) <= len(want) || vb[len(vb)-len(want):] != want {
		return false, nil
	}
	for k := range s.Params {
		switch k {
		case "Mode", "ValidAttacker", "ValidBlocker", "Description":
		default:
			return true, refuse("Effect CantBlockBy param %s over RememberObjects$ TargetedController", k)
		}
	}
	// the chain's own permanent target, read live at resolution
	t := lc.lastTarget()
	// lastTarget is -1 when the chain has no target: that lands here too.
	if t != 0 {
		return true, refuse("Effect CantBlockBy over TargetedController without target slot 0")
	}
	if pt := lc.ab.Targets[t]; pt.Spell || pt.StackAb || pt.Zone == zStack || pt.F == nil || pt.F.Players != 0 {
		return true, refuse("Effect CantBlockBy over TargetedController of a non-permanent target")
	}
	eot, next, host, err := effectEmblemDuration(ps)
	if err != nil {
		return true, err
	}
	if next || host || !eot {
		return true, refuse("Effect CantBlockBy over TargetedController with a next-turn or host-leaves Duration$")
	}
	cp := make(map[string]string, len(s.Params))
	for k, v := range s.Params {
		cp[k] = v
	}
	cp["ValidBlocker"] = vb[:len(vb)-len(want)] + "+YouCtrl"
	s.Params = cp
	sts, err := lc.staticBody(s)
	if err != nil {
		return true, err
	}
	if len(sts) != 1 || sts[0].Kind != "CantBlockBy" {
		return true, refuse("Effect CantBlockBy lowers to %d statics", len(sts))
	}
	if err := emblemFiltersRefuse("Effect CantBlockBy over TargetedController", sts[0].F, sts[0].F2); err != nil {
		return true, err
	}
	ps.consume("RememberObjects", "Name", "Image", "Stackable")
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("Effect static unread %v", left)
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: sts, UntilEOT: eot})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rTargetCtrlLive}, C: len(lc.out.Emblems) - 1})
	return true, nil
}
