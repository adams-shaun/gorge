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
// Only a setter whose AddType$ words are all basic land types is lowered:
// the attached form (Spreading Seas, Evil Presence, Contaminated Ground) and
// the global form over a land filter (Blood Moon, Magus of the Moon,
// Harbinger of the Seas: "Nonbasic lands are Mountains"), which the layer
// walk applies to every land the filter matches, its loss reading the
// same landSetLost view (overrideStatic exempts SetLandSub from its refusal
// of a global lose-all over non-creatures). A setter with more than the
// basic types (Celestial Dawn's colour setting, Conversion, Glaciers) stays
// refused by the words it adds. One chosen spelling lowers: AddType$
// ChosenType (Convincing Mirage, Phantasmal Terrain, Multiversal Passage),
// the record adding no subtype of its own and the static carrying AddChosen,
// the source's own as-enters ChooseType answer (lower_override.go); any
// other ChosenType shape on the override path refuses by the word's class.
func landTypeSetter(v string, st *lStatic, o *lOverride, removesTypes bool) error {
	if strings.TrimSpace(v) != "True" {
		return refuse("RemoveLandTypes %q", v)
	}
	if removesTypes || o.SetTypes != 0 || o.RemoveTypes != 0 || o.AllCreatureTypes || o.LoseAbilities {
		return refuse("RemoveLandTypes$ beside another type change or a lose-all")
	}
	if len(o.SetSubtypes) == 0 && !o.Chosen {
		// o.Chosen: the added type is the source's own as-enters ChooseType
		// answer (Convincing Mirage's AddType$ ChosenType); the record adds
		// no subtype of its own and the static's AddChosen reads it.
		return refuse("RemoveLandTypes$ adds no basic land type")
	}
	for _, w := range o.SetSubtypes {
		switch w {
		case "Plains", "Island", "Swamp", "Mountain", "Forest":
		default:
			return refuse("RemoveLandTypes$ adds %q", w)
		}
	}
	o.SetLandSub, o.LoseAbilities = true, true
	return nil
}
