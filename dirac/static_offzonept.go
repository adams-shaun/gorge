package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatOffZonePT (The Muckslinger Gang's "Creature spells you control and
// creature cards you own in any zone other than the battlefield or the stack
// get +3/+3") is a marker the kernel does not apply: no off-battlefield
// characteristic change is modelled. gorge applies the boost at its rules-side
// match seam (statics.go matchesSpec over e.Derived, a SpellCast ValidCard$)
// and reads printed values at its effects-side ones (filter_value.go
// objectPowerInContext, count_ref_property.go refPower). The kernel copies
// neither: every off-battlefield P/T reader asks offZonePTRead first, and a
// read of a card the live static boosts stalls unmodelled. A table without the
// static never opens the check (genPrep.anyOffZonePT).

// StallOffZonePT is an off-battlefield P/T read of a card a live StatOffZonePT
// static boosts.
const StallOffZonePT = 188

func init() { stallNames[StallOffZonePT] = "offzonept" }

// offZonePTRead stalls the game when card c (owned by owner; object o, NoObj
// for a card read without one) is off the battlefield and a live StatOffZonePT
// static boosts it: a creature card its owner's static reaches, or any spell
// on the stack while some such static is in force (a face-down spell is a
// creature; the stack clause reads the spell's controller).
func (s *State) offZonePTRead(k *prep, c CardID, owner uint8, o Obj) {
	if k.gen == nil || !k.gen.anyOffZonePT {
		return
	}
	s.offZonePTCheck(k, c, owner, o)
}

//go:noinline
func (s *State) offZonePTCheck(k *prep, c CardID, owner uint8, o Obj) {
	stack := false
	if o != NoObj {
		switch z := s.objs[o].zone; {
		case z == kop.ZoneBattlefield || z.Phased():
			return // a permanent, phased in or out (CR 702.26d), is never boosted
		case z == kop.ZoneStack:
			stack = true
		}
	}
	if !stack && k.t.Cards[c].Types&kop.TCreature == 0 {
		return
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatOffZonePT && (stack || st.ctrl == owner) {
			hit = true
			return false
		}
		return true
	})
	if hit {
		s.stall(StallOffZonePT)
	}
}
