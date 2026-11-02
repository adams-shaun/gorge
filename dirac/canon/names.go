package canon

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// CounterNames is gorge's name for each counter kind the kernel keeps.
var CounterNames = func() [kop.NumCounterKinds2]string {
	var n [kop.NumCounterKinds2]string
	n[kop.CtrP1P1], n[kop.CtrM1M1], n[kop.CtrLoyalty], n[kop.CtrStun] = "P1P1", "M1M1", "LOYALTY", "STUN"
	n[kop.CtrShield], n[kop.CtrFlying], n[kop.CtrOther] = "SHIELD", "Flying", "OTHER"
	n[kop.CtrIncubation], n[kop.CtrBait], n[kop.CtrSoul] = "INCUBATION", "BAIT", "SOUL"
	n[kop.CtrFellowship], n[kop.CtrRevival], n[kop.CtrStash] = "FELLOWSHIP", "REVIVAL", "STASH"
	return n
}()

// FaceSA is the spell ability a face index names: Face.Abilities, Face.Triggers' effect
// (0x80|i), or the face's spell ability (255).
func FaceSA(f *cards.Face, gi uint8) *cards.SA {
	switch {
	case gi == 255:
		return f.SpellAbility()
	case gi&0x80 != 0:
		if i := int(gi &^ 0x80); i < len(f.Triggers) {
			return f.Triggers[i].Effect
		}
	case int(gi) < len(f.Abilities):
		return f.Abilities[gi]
	}
	return nil
}
