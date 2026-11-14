package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A mid-resolution ChooseType | Type$ Basic Land (Navigator's Compass): gorge
// effChooseType offers TypeChoiceLabels("Basic Land") -- Forest, Island,
// Mountain, Plains, Swamp, in that order -- and records the answer on the
// source; the Animate | Types$ ChosenType after it adds that type to the
// target land at resolution (combatfx.go animateGrant), and with it the
// type's intrinsic mana ability (CR 305.6, static_landtype.go). kgen marks
// the OpChooseType with B = kop.ChooseTypeBasicLand and lowers the Animate to
// an OpOverride with B = kop.OvChosenType naming the first of five records,
// one added basic land type each. A label outside the creature-type
// vocabulary is the oracle's 0x8000 | FNV-1a(label) & 0x7fff in (target,
// mode), the card-type ask's encoding (act_choosetype.go cardTypeLabelH). The
// answer is the chosen type's matchup subtype index in res.chosen (and the
// source's chosen), so a ChosenType filter reads it as a subtype.

// basicLandChoice is gorge's Type$ Basic Land option list, in its order.
var basicLandChoice = [5]string{"Forest", "Island", "Mountain", "Plains", "Swamp"}

// basicLandLabelH is each basicLandChoice label's canonical option value.
var basicLandLabelH = func() (h [len(basicLandChoice)]uint16) {
	for i, c := range basicLandChoice {
		v := uint32(2166136261)
		for j := 0; j < len(c); j++ {
			v ^= uint32(c[j])
			v *= 16777619
		}
		h[i] = 0x8000 | uint16(v&0x7fff)
	}
	return
}()

// DebugChosenLandTypeFirst (TestNavigatorsCompassLowers' mutation) always
// applies the first (Forest) record, whatever type was chosen.
var DebugChosenLandTypeFirst bool

// DebugChooseTypeReuse (TestNavigatorsCompassLowers' mutation) restores the
// retired reuse: an ability whose battlefield source already carries a chosen
// type keeps it and asks nothing (CR 608.2 has each resolution choose).
var DebugChooseTypeReuse bool

// isBasicLandChoose reports whether in is a ChooseType Basic Land.
func isBasicLandChoose(in *kop.Instr) bool {
	return in.Op == kop.OpChooseType && in.C == 0 && in.B.N == kop.ChooseTypeBasicLand
}

// chooseBasicLandOptions adds the basic land type ask's five options.
//
//go:noinline
func chooseBasicLandOptions(d *Decision) {
	for _, h := range basicLandLabelH {
		d.add(MakeAction(AkChoose, 0, TCode(h>>8), uint8(h)))
	}
}

// basicLandAnswer is the matchup subtype index an answer names, 255 when the
// label is no basic land type or the type is not in the matchup.
//
//go:noinline
func basicLandAnswer(k *prep, a Action) uint8 {
	v := uint16(a.Target())<<8 | uint16(a.Mode())
	for i, h := range basicLandLabelH {
		if h != v {
			continue
		}
		for j, n := range k.t.Subtypes {
			if n == basicLandChoice[i] {
				return uint8(j)
			}
		}
	}
	return 255
}

// basicLandKey is a basic land type option's place in gorge's option order,
// false for any other option value.
func basicLandKey(v int) (uint32, bool) {
	for i, h := range basicLandLabelH {
		if int(h) == v {
			return uint32(i), true
		}
	}
	return 0, false
}

// ovChosenType is OpOverride's chosen-land-type arm (interp_gen.go): of the
// five records from C, the one adding the resolution's chosen subtype. No
// choice (or a type no record adds) applies nothing, gorge's fail-closed
// animateGrant.
//
//go:noinline
func (s *State) ovChosenType(k *prep, sl Slot, C int, card CardID, fl uint8) {
	if DebugChosenLandTypeFirst {
		s.applyOverrideFl(k, sl, C, card, fl)
		return
	}
	if s.res.bad&resChosenType == 0 || s.res.chosen == 255 {
		return
	}
	for i := 0; i < len(basicLandChoice) && C+i < len(k.t.Overrides); i++ {
		if k.t.Overrides[C+i].AddSub.Has(int(s.res.chosen)) {
			s.applyOverrideFl(k, sl, C+i, card, fl)
			return
		}
	}
}
