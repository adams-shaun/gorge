package kgen

import "github.com/adams-shaun/gorge/engine/cards"

func init() { apiOp("Unattach", (*lowerCtx).unattach) }

// unattach lowers api:Unattach (gorge engine/effects/unattach.go
// effUnattach): each defined object still attached to a battlefield permanent
// is detached, and the former bearer rides the Unattached event for the
// Mode$ Unattached family (effUnattach's Emit at line 31).
//
// gorge's Defined walk may name a group; the kernel's detach reads one
// object, so only the Defined$ values that name exactly one object lower
// (Enchanted / Equipped / AttachedTo, Self, ChosenCard, the Triggered* object
// forms). A group form (Remembered, DelayTriggerRemembered*, a Valid filter)
// and any selector gorge answers with a Note refuse by name, as do the keys
// effUnattach does not read.
func (lc *lowerCtx) unattach(sa *cards.SA, ps *params) error {
	d, ok := ps.get("Defined")
	if !ok {
		return refuse("Unattach with no Defined$")
	}
	ref, err := lc.defined(d)
	if err != nil {
		return err
	}
	switch ref.Kind {
	case rSelf, rEquipped, rChosen, rTriggered:
	default:
		return refuse("Unattach Defined$ %s: only a Defined$ naming one object lowers", d)
	}
	lc.emit(lInstr{Op: "Unattach", Obj: ref})
	return nil
}
