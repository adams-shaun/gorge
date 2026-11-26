package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// OffZoneCharStats counts the card matches an off-zone layer-4 grant changed
// (offZoneChars: Maskwood Nexus's every creature type, Rootpath Purifier's
// basic supertype).
var OffZoneCharStats struct{ Hits atomic.Int64 }

// offZoneChars reads the layer-4 grants of the StatOffZoneGrant statics that
// carry an Override (Maskwood Nexus, Rootpath Purifier) into the card types and
// subtypes of object o, a card off the battlefield (gorge applies the static in
// every zone it names: chars.StaticZoneAdmitsStatic). all reports that o has
// every creature type. The battlefield half of such a static is its twin
// StatOverride, read by the layer walk, so a battlefield object is left alone.
//
// The grant's filter is matched against the printed card: neither grant changes
// a characteristic its own filter reads (CR 613.8). Only the view reads this
// fold. Gorge's type and subtype filters read the layer-4 table, which is built
// from battlefield objects only (engine/rules/layer4types.go
// buildDerivedTypesWalk), so a search, a cast trigger or a count reads the
// printed card in every other zone, and the kernel's card matchers do too.
func (s *State) offZoneChars(k *prep, c CardID, owner uint8, o Obj, ty kop.TypeMask, subs kop.SubMask) (kop.TypeMask, kop.SubMask, bool) {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return ty, subs, false
	}
	all := false
	oty, osubs := ty, subs
	s.eachStaticC(k, sclsOffKwG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatOffZoneGrant || a.Static.Override == 0 {
			return true
		}
		if a.Static.Zone != kop.ZoneAny && a.Static.Zone != ob.zone {
			return true
		}
		if !s.statCond(k, st) {
			return true
		}
		if !s.matchCardGenAt(k, a.Static.Filter, c, owner, st.ctrl, o, nil) {
			return true
		}
		ov := &k.t.Overrides[a.Static.Override-1]
		ty |= ov.AddTypes
		if ov.AllCreatureTypes {
			subs, all = subs.Or(k.gen.creatSub), true
		}
		return true
	})
	if ty != oty || subs != osubs {
		OffZoneCharStats.Hits.Add(1)
	}
	return ty, subs, all
}
