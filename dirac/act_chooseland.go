package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An as-enters ChooseType | Type$ Land (Traveler's Cloak: "choose a land
// type", landwalk of the chosen type for the enchanted creature). gorge
// effChooseType offers TypeChoiceLabels("Land") -- the 5 basic and 13 nonbasic
// land types, sorted -- and records the answer as the permanent's ChosenType.
// kgen marks the enters program's OpChooseType with B = kop.ChooseTypeLand;
// typeOptions poses the 18 labels (landChoice order) and every as-enters
// answer site's typeVocabCode maps one to the chosen type's matchup subtype
// index (landValue), or 255 for a type no card of the matchup has or names:
// a ChosenType read then matches no land, the same on gorge's side (no land
// of the game has the type). The encoding is the basic ask's: 0x8000 |
// FNV-1a(label) & 0x7fff in (target, mode).

// landChoice is gorge's Type$ Land option list (effects.TypeChoiceLabels
// ("Land")), in its order.
var landChoice = [18]string{
	"Cave", "Desert", "Forest", "Gate", "Island", "Lair", "Locus", "Mine", "Mountain",
	"Omenpath", "Plains", "Planet", "Power-Plant", "Sphere", "Swamp", "Tower", "Town", "Urza's",
}

// landLabelH is each landChoice label's canonical option value.
var landLabelH = func() (h [len(landChoice)]uint16) {
	for i, c := range landChoice {
		h[i] = labelHash(c)
	}
	return
}()

// labelHash is the oracle's option value of a label outside the creature-type
// vocabulary: 0x8000 | FNV-1a(label) & 0x7fff.
func labelHash(c string) uint16 {
	v := uint32(2166136261)
	for j := 0; j < len(c); j++ {
		v ^= uint32(c[j])
		v *= 16777619
	}
	return 0x8000 | uint16(v&0x7fff)
}

// landKey is a land type option's place in gorge's option order, false for
// any other option value.
func landKey(v int) (uint32, bool) {
	for i, h := range landLabelH {
		if int(h) == v {
			return uint32(i), true
		}
	}
	return 0, false
}

// isLandChoose reports whether in is an as-enters ChooseType Land.
func isLandChoose(in *kop.Instr) bool {
	return in.Op == kop.OpChooseType && in.C == 0 && in.B.N == kop.ChooseTypeLand
}

// chooseLandOptions adds the land type ask's 18 options.
//
//go:noinline
func chooseLandOptions(d *Decision) {
	for _, h := range landLabelH {
		d.add(MakeAction(AkChoose, 0, TCode(h>>8), uint8(h)))
	}
}

// landValue is the matchup subtype index of the land type option value v
// names, 255 when v names no land type or the type is not in the matchup.
func landValue(k *prep, v uint16) uint8 {
	for i, h := range landLabelH {
		if h != v {
			continue
		}
		for j, n := range k.t.Subtypes {
			if n == landChoice[i] {
				return uint8(j)
			}
		}
	}
	return 255
}
