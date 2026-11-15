package kgen

import (
	"strconv"
	"strings"
)

// offZonePTAffected is the one Affected$ spelling an AffectedZone$ All P/T
// grant lowers with (The Muckslinger Gang): creature spells the controller
// controls, and creature cards they own in every zone but the battlefield and
// the stack.
const offZonePTAffected = "Creature.YouCtrl+inZoneStack,Creature.YouOwn+!inZoneBattlefield+!inZoneStack"

// offZonePTShape reports an AffectedZone$ All static that is only that P/T
// grant: no keyword, type, colour or permission beside it.
func offZonePTShape(sp *params, aff string) bool {
	if aff != offZonePTAffected {
		return false
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "AddPower", "AddToughness", "Description":
		default:
			return false
		}
	}
	return true
}

// offZonePTStatic lowers an offZonePTShape static to kop.StatOffZonePT. gorge
// applies the boost in every zone (layers.go's static scan leaves
// PermanentsOnly false for AffectedZone$ All; layers_types.go affectsZone),
// which the kernel does not model off the battlefield: the static is a marker
// the kernel's off-battlefield P/T readers stall on while it is in force
// (dirac/static_offzonept.go). gorge's effects-side readers (Dig ChangeValid$,
// Count$ CardPower of a card off the battlefield) read the printed value, its
// rules-side ones (a SpellCast ValidCard$) the boosted one; the stall keeps
// the kernel from copying either. A gate or a non-constant amount refuses.
func offZonePTStatic(sp *params, st *lStatic) error {
	if st.Cond.Kind != cNone {
		return refuse("AffectedZone \"All\" P/T grant with a gate")
	}
	var amt [2]int
	for i, k := range [2]string{"AddPower", "AddToughness"} {
		v, ok := sp.get(k)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < -127 || n > 127 {
			return refuse("AffectedZone \"All\" P/T grant %s %q", k, v)
		}
		amt[i] = n
	}
	sp.consume("AffectedZone", "Description")
	st.Kind = "OffZonePT"
	st.Power, st.Tough = cst(amt[0]), cst(amt[1])
	return nil
}
