package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
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

	// Multiversal Passage: the same setter on the entering land itself. The
	// chain lowers as ask-then-elect: the enter program is the as-enters
	// basic-land ask, then the pay-2-life election, then the tap body the
	// election skips on a payment — the kernel's shock machinery reads the
	// election behind the ask (repl_shockland.go's widened election pc), so
	// the land play's count waits for both asks.
	const passage = "Name:Chosen Setter Land Probe\nManaCost:no cost\nTypes:Land\n" +
		"R:Event$ Moved | ValidCard$ Card.Self | Destination$ Battlefield | ReplaceWith$ DBPickBL2 | ReplacementResult$ Updated | Description$ Enters: pick a basic land type, then pay 2 life or enter tapped.\n" +
		"SVar:DBPickBL2:DB$ ChooseType | Type$ Basic Land | ETB$ True | SubAbility$ DBTap2\n" +
		"SVar:DBTap2:DB$ Tap | UnlessCost$ PayLife<2> | UnlessPayer$ You | ETB$ True | Defined$ Self\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | RemoveLandTypes$ True | AffectedDefined$ Self | Description$ The land is of the chosen type.\n"
	lc = lowerSynth(t, passage)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the ask-then-elect chain refuses: %q", keclReason(r))
	}
	var prog []lInstr
	for i := range lc.Repls {
		if p := lc.Repls[i].Prog; lc.Repls[i].Kind == "EntersWith" && len(p) == 3 && p[0].Op == "ChooseType" {
			prog = p
		}
	}
	if prog == nil {
		t.Fatalf("no ask-then-elect enter program among %+v", lc.Repls)
	}
	if prog[0].B.N != int(kop.ChooseTypeBasicLand) {
		t.Errorf("the ask is not the basic-land list: %+v", prog[0])
	}
	if prog[2].Op != "EnterTapped" {
		t.Errorf("the election's tap body is %q, want EnterTapped", prog[2].Op)
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
