package kgen

import "strings"

// landTypeSetter lowers a static's RemoveLandTypes$ (CR 305.7: Spreading
// Seas' "Enchanted land is an Island"). gorge's applyTypeEffect strips every
// land subtype before the effect's AddType$ words, and a land whose land
// types an applied effect sets to basic land types loses every ability its
// rules text generates, as part of that layer-4 effect
// (state.SetsBasicLandSubtypes; rules/abilityloss.go reports the loss with a
// zero timestamp, so a granted ability survives it, and the intrinsic mana
// ability of each new basic type stays). The record is the override's
// SetLandSub beside LoseAbilities (kop.Override.SetLandSub; the kernel's
// reading is dirac/static_landtypesetter.go).
//
// Only the attached form whose AddType$ words are all basic land types is
// lowered (Spreading Seas, Evil Presence, Contaminated Ground). A global
// setter (Blood Moon, Harbinger of the Seas, Celestial Dawn) stays refused
// by name: the walk's lose-all over non-creatures is not modelled. A chosen
// type (Convincing Mirage's AddType$ ChosenType) is refused by AddType$.
func landTypeSetter(v string, st *lStatic, o *lOverride, removesTypes bool) error {
	if strings.TrimSpace(v) != "True" {
		return refuse("RemoveLandTypes %q", v)
	}
	if removesTypes || o.SetTypes != 0 || o.RemoveTypes != 0 || o.AllCreatureTypes || o.LoseAbilities {
		return refuse("RemoveLandTypes$ beside another type change or a lose-all")
	}
	if len(o.SetSubtypes) == 0 {
		return refuse("RemoveLandTypes$ adds no basic land type")
	}
	for _, w := range o.SetSubtypes {
		switch w {
		case "Plains", "Island", "Swamp", "Mountain", "Forest":
		default:
			return refuse("RemoveLandTypes$ adds %q", w)
		}
	}
	if !attachedOnly(st.F) {
		return refuse("global land-type setter (RemoveLandTypes$)")
	}
	o.SetLandSub, o.LoseAbilities = true, true
	return nil
}
