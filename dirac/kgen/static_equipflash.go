package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The Equip arm of Mode$ CastWithFlash (modeCastWithFlash):
//
//	S:Mode$ CastWithFlash | ValidSA$ Activated.Equip | Caster$ You
//
// Leonin Shikari, and Forge Anew's copy under Condition$ PlayerTurn. The
// controller may activate Equip abilities any time they could cast an instant
// (CR 702.6; gorge rules equipAtInstantSpeed). It lowers to the "EquipFlash"
// static kind, read by the kernel's canActivate (dirac/static_equipflash.go).
// Only the printed battlefield shape gorge reads is lowered: no ValidCard$ or
// EffectZone$, and no condition beyond PlayerTurn.

const equipFlashKind = "EquipFlash"

func init() {
	registerStaticBuild(equipFlashKind, cbEquipFlashStatic)
}

// equipFlashShape lowers the arm's shape onto sc.st.
func (sc *staticCtx) equipFlashShape() error {
	sp := sc.sp
	if sp.str("Caster") != "You" {
		return refuse("CastWithFlash Activated.Equip Caster$ %q (only You is lowered)", sp.str("Caster"))
	}
	for _, k := range []string{"ValidCard", "EffectZone"} {
		if _, ok := sp.get(k); ok {
			return refuse("CastWithFlash Activated.Equip with %s$", k)
		}
	}
	if c := sc.st.Cond.Kind; c != cNone && c != cYourTurn {
		return refuse("CastWithFlash Activated.Equip condition (only PlayerTurn is lowered)")
	}
	sp.consume("ValidSA", "Caster")
	sc.st.Kind = equipFlashKind
	return nil
}

func cbEquipFlashStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatEquipFlash
	return nil, nil
}
