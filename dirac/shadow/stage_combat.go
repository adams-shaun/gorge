package shadow

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/view"
)

// markAttack sets the attacker marks of o from the view of its card: whether
// it attacks and which player, and whether it became blocked. A creature
// attacking a planeswalker or a battle is refused by checkCard.
func (b *builder) markAttack(o *dirac.WorldObj, cv *view.CardView, name string) error {
	if cv.Attacking {
		if cv.AttackingPlayer == nil || int(*cv.AttackingPlayer) >= len(b.in.View.Players) {
			return bad(dirac.ReasonCombat, "%s attacks no seat", name)
		}
		o.Attacking, o.AtkSeat = true, uint8(*cv.AttackingPlayer)
	}
	o.Blocked = cv.Blocked
	return nil
}

// combat adds the blocks the view shows. A view lists a blocker on the attacker
// it blocks, in block order, and marks every blocker Blocking: one marked and
// listed nowhere blocks nothing (its attacker has left combat, CR 506.4). A
// blocker listed under two attackers, or one that is not on a battlefield the
// view shows, is not rebuildable. An attacker blocked with no blocker listed
// (every blocker has left, CR 509.1h) is dirac's to place: legal from the
// damage steps on.
func (b *builder) combat() error {
	at := map[dirac.Obj]int{} // object index -> its place in b.all
	for i := range b.all {
		at[b.all[i].obj] = i
	}
	for _, p := range b.in.View.Players {
		for i := range p.Battlefield {
			if cv := &p.Battlefield[i]; len(cv.BlockedBy) != 0 {
				if err := b.blocks(cv, at); err != nil {
					return err
				}
			}
		}
	}
	b.orphans(at)
	return nil
}

// orphans marks the blockers no attacker lists as blockers of nothing.
func (b *builder) orphans(at map[dirac.Obj]int) {
	for _, p := range b.in.View.Players {
		for i := range p.Battlefield {
			cv := &p.Battlefield[i]
			if !cv.Blocking {
				continue
			}
			o, _ := b.in.ObjOf(int(cv.ID))
			if k, ok := at[o]; ok && !b.all[k].wo.Blocking {
				b.all[k].wo.Blocking, b.all[k].wo.Blocks = true, dirac.NoObj
			}
		}
	}
}

// blocks marks the blockers of attacker cv.
func (b *builder) blocks(cv *view.CardView, at map[dirac.Obj]int) error {
	a, ok := b.in.ObjOf(int(cv.ID))
	if !ok {
		return bad(ReasonUnmapped, "%s (%d)", cv.Name, cv.ID)
	}
	for j, id := range cv.BlockedBy {
		bo, ok := b.in.ObjOf(int(id))
		if !ok {
			return bad(ReasonUnmapped, "a blocker of %s (%d)", cv.Name, id)
		}
		k, ok := at[bo]
		if !ok || b.all[k].wo.Blocking {
			return bad(dirac.ReasonCombat, "a blocker of %s is not on the battlefield or blocks two attackers", cv.Name)
		}
		w := &b.all[k].wo
		w.Blocking, w.Blocks, w.BlockOrd = true, a, uint8(j+1)
	}
	return nil
}
