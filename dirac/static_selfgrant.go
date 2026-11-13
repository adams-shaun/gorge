package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A permanent's own subtype grant (Roaming Throne's chosen type, Skyknight
// Squire's Knight) under a subtype setter that removes abilities (Eaten by
// Piranhas, Witness Protection) is on the fixed-order path: prepLayerWalk
// sets fxNeedSubSet only for a setter that can meet the grant and keep it.
// The grant begins in layer 4, so the loss in layer 6 does not end it
// (CR 613.6, keptOnLoss), but the setter is newer than the grant's source
// (an Aura or a record lands after the permanent entered; an older global
// setter walks, walkSlot), so in timestamp order it overwrites the grant
// (CR 613.7). The fixed-order readers apply the overrides before the static
// grants, so they mask a self grant with selfGrantKept.

// selfGrantKept is what a subtype grant of sl's own static (src == sl) keeps
// under the subtype setters on sl: keep masks the matchup subtypes, and
// outside reports whether a creature type outside the matchup survives.
// A grant from another permanent keeps everything.
func (s *State) selfGrantKept(k *prep, sl, src Slot) (keep kop.SubMask, outside bool) {
	keep, outside = kop.SubMask{^uint64(0), ^uint64(0), ^uint64(0)}, true
	if src != sl {
		return keep, outside
	}
	s.eachOverride(k, sl, func(o *kop.Override, _ CardID) {
		if o.HasSub {
			keep, outside = keep.And(o.KeepSub), false
		}
		if o.RemoveCreatureSub && !DebugRmCreatSubKeep {
			keep, outside = keep.AndNot(k.gen.creatSub), false
		}
		if o.LandSet {
			keep = keep.AndNot(k.gen.landSub)
		}
	})
	return keep, outside
}
