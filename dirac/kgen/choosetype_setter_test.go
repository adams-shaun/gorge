package kgen

import (
	"strings"
	"testing"
)

// TestChooseTypeChosenSetter pins the CR 305.7 land-type setter whose added
// type is the source's own as-enters ChooseType answer (Convincing Mirage,
// Phantasmal Terrain: RemoveLandTypes$ True with AddType$ ChosenType). The
// record is the setter's kop.Override.SetLandSub beside LoseAbilities with
// no added subtype of its own, and the static carries AddChosen, the flag
// that reads the source's perm.chosen (act_choosetype.go); the walk grants
// the chosen subtype after the strip. A ChosenType word on the override path
// without the setter behind it refuses exactly as before, and the
// fixed-subtype setter (Spreading Seas) keeps its shape.
func TestChooseTypeChosenSetter(t *testing.T) {
	const mirage = "Name:Chosen Setter Probe\nManaCost:1 U\nTypes:Enchantment Aura\n" +
		"K:Enchant:Land\n" +
		"K:ETBReplacement:Other:DBPickBL\n" +
		"SVar:DBPickBL:DB$ ChooseType | Type$ Basic Land | SpellDescription$ As it enters, this picks a basic land type.\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | AddType$ ChosenType | RemoveLandTypes$ True | Description$ The enchanted land is of the chosen type.\n"
	lc := lowerSynth(t, mirage)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the chosen land-type setter refuses: %q", keclReason(r))
	}
	var set *lStatic
	for i := range lc.Statics {
		st := &lc.Statics[i]
		if st.Override != nil && st.Override.SetLandSub {
			if set != nil {
				t.Fatalf("two SetLandSub statics: %+v and %+v", set, st)
			}
			set = st
		}
	}
	if set == nil {
		t.Fatalf("no SetLandSub override static among %+v", lc.Statics)
	}
	if !set.Override.LoseAbilities {
		t.Errorf("the chosen setter record keeps the land's own abilities: %+v", set.Override)
	}
	if len(set.Override.SetSubtypes) != 0 {
		t.Errorf("the chosen setter record adds subtypes %v of its own", set.Override.SetSubtypes)
	}
	if !set.AddChosen {
		t.Errorf("the chosen setter static carries no AddChosen: %+v", set)
	}

	// Multiversal Passage: the same setter on the entering land itself. Its
	// ETB$ True ask lowers; its chained pay-2-life rider still refuses (the
	// chain's Tap election needs the land-play count deferral widened before
	// it lowers, the follow-up ticket's shape), so the ETB$ marker itself is
	// what stays consumed here.
	const passage = "Name:Chosen Setter Land Probe\nManaCost:no cost\nTypes:Land\n" +
		"R:Event$ Moved | ValidCard$ Card.Self | Destination$ Battlefield | ReplaceWith$ DBPickBL2 | ReplacementResult$ Updated | Description$ Enters: pick a basic land type, then pay 2 life or enter tapped.\n" +
		"SVar:DBPickBL2:DB$ ChooseType | Type$ Basic Land | ETB$ True | SubAbility$ DBTap2\n" +
		"SVar:DBTap2:DB$ Tap | UnlessCost$ PayLife<2> | UnlessPayer$ You | ETB$ True | Defined$ Self\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | RemoveLandTypes$ True | AffectedDefined$ Self | Description$ The land is of the chosen type.\n"
	if r := lowerSynth(t, passage).Refusals; len(r) == 0 {
		t.Errorf("the entering land's chosen setter lowers, want the chained rider's refusal")
	} else if !strings.Contains(keclReason(r), "ETB tap UnlessCost on a chained ETB replacement") {
		t.Errorf("the refusal is %q, want the chained rider's own", keclReason(r))
	}

	// The control: a fixed-subtype setter (Spreading Seas) keeps its shape —
	// one added subtype, no AddChosen.
	const seas = "Name:Fixed Setter Probe\nManaCost:1 U\nTypes:Enchantment Aura\n" +
		"K:Enchant:Land\n" +
		"S:Mode$ Continuous | AddType$ Island | RemoveLandTypes$ True | AffectedDefined$ Enchanted | Description$ The enchanted land is an Island.\n"
	lc = lowerSynth(t, seas)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the fixed-subtype setter control refuses: %q", keclReason(r))
	}
	set = nil
	for i := range lc.Statics {
		st := &lc.Statics[i]
		if st.Override != nil && st.Override.SetLandSub {
			set = st
		}
	}
	if set == nil {
		t.Fatalf("the fixed-subtype setter lowers to no SetLandSub static: %+v", lc.Statics)
	}
	if set.AddChosen || len(set.Override.AddSubtypes) != 1 || set.Override.AddSubtypes[0] != "Island" {
		t.Errorf("the fixed-subtype setter moved: %+v", *set.Override)
	}

	// A ChosenType word on the override path without the setter behind it
	// refuses exactly as before this shape, by the AddType$ word's own class.
	const noSet = "Name:Chosen No Setter Probe\nManaCost:1 U\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | RemoveCardTypes$ True | AffectedDefined$ Self | Description$ probe.\n"
	lc = lowerSynth(t, noSet)
	if r := lc.Refusals; len(r) == 0 {
		t.Errorf("AddType$ ChosenType without RemoveLandTypes$ lowers, want a refusal")
	} else if !containsClass(lc, `other:AddType "_"`) {
		t.Errorf("the refusal is %q, want the AddType word's own class", keclReason(r))
	}
}

// containsClass reports whether one of lc's refusals normalises to class
// want (classes.go refusalClasses, over the lowered card's own report).
func containsClass(lc *lCard, want string) bool {
	for _, r := range lc.Refusals {
		for _, c := range refusalClasses(r.Reason) {
			if c == want {
				return true
			}
		}
	}
	return false
}
