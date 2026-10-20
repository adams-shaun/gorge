package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// AnnouncedDivision returns the CR 601.2d division announced for sa's
// targets while the resolving spell was cast or the ability activated
// (state.Object.Division, folded from events.Choose "divide"), aligned with
// ts: shares[i] is what ts[i] receives. A target in ts the announcement
// does not name receives 0. ok is false -- the caller keeps its
// resolution-time path -- unless sa is the resolving object's own root
// target declaration (Ctx.OfferedSA, matched by Line) and the object carries
// a division aligned with its Targets. A target the CR 608.2b recheck
// dropped is absent from ts, so its share is lost, never redistributed.
func AnnouncedDivision(h Host, c *Ctx, sa *cards.SA, ts []state.Target) ([]int32, bool) {
	if sa == nil || c.ResolvingObj == 0 || c.OfferedSA == nil || c.OfferedSA.Line != sa.Line {
		return nil, false
	}
	o := h.Game().Obj(c.ResolvingObj)
	if o == nil || len(o.Division) == 0 || len(o.Division) != len(o.Targets) {
		return nil, false
	}
	used := make([]bool, len(o.Targets))
	shares := make([]int32, len(ts))
	for i, t := range ts {
		for j, ot := range o.Targets {
			if used[j] || ot.IsPlayer != t.IsPlayer || (t.IsPlayer && ot.Player != t.Player) || (!t.IsPlayer && ot.Obj != t.Obj) {
				continue
			}
			used[j] = true
			shares[i] = o.Division[j]
			break
		}
	}
	return shares, true
}
