package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// cantAttackRemembered lowers a no-target Effect whose one static is
// "Mode$ CantAttack | ValidCard$ Creature.RememberedPlayerCtrl" over
// RememberObjects$ TriggeredPlayer (Peace in Our Time: "that player can't
// attack this turn") to an EOT emblem handed to the trigger's player, whose
// own creatures its Creature.YouCtrl filter names. The kernel's trigger
// player is the Phase trigger's active player (lower_effect.go's
// TriggeredPlayer arm), the player gorge's RememberedPlayerCtrl must name.
// ok reports the Effect had this shape (handled, with err the outcome).
func (lc *lowerCtx) cantAttackRemembered(ps *params, ss []cards.Static) (bool, error) {
	if ps.str("RememberObjects") != "TriggeredPlayer" || len(ss) != 1 || ss[0].Mode != "CantAttack" {
		return false, nil
	}
	s := ss[0]
	if s.Params["ValidCard"] != "Creature.RememberedPlayerCtrl" {
		return false, nil
	}
	for k := range s.Params {
		switch k {
		case "Mode", "ValidCard", "Description":
		default:
			return true, refuse("Effect CantAttack param %s over RememberObjects$ TriggeredPlayer", k)
		}
	}
	who, err := lc.defined("TriggeredPlayer")
	if err != nil {
		return true, err
	}
	eot, next, err := effectEmblemDuration(ps)
	if err != nil {
		return true, err
	}
	if next {
		return true, refuse("Effect CantAttack over TriggeredPlayer with a next-turn Duration$")
	}
	cp := make(map[string]string, len(s.Params))
	for k, v := range s.Params {
		cp[k] = v
	}
	cp["ValidCard"] = "Creature.YouCtrl"
	s.Params = cp
	sts, err := lc.staticBody(s)
	if err != nil {
		return true, err
	}
	if len(sts) != 1 {
		return true, refuse("Effect CantAttack lowers to %d statics", len(sts))
	}
	ps.consume("RememberObjects", "Name", "Image", "Stackable")
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("Effect static unread %v", left)
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: sts, UntilEOT: eot})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: who, C: len(lc.out.Emblems) - 1})
	return true, nil
}
