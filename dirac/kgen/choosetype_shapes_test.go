package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// chooseTypeInstrs is every ChooseType instruction of the lowered card, in
// ability then replacement order (an as-enters choice sits in a replacement's
// enters program).
func chooseTypeInstrs(lc *lCard) []lInstr {
	var out []lInstr
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "ChooseType" {
				out = append(out, in)
			}
		}
	}
	for _, r := range lc.Repls {
		for _, in := range r.Prog {
			if in.Op == "ChooseType" {
				out = append(out, in)
			}
		}
	}
	return out
}

// TestChooseTypeShapes pins the as-enters ChooseType shapes. An as-enters
// "choose a basic land type" (Realmwright: K:ETBReplacement ... DB$
// ChooseType | Type$ Basic Land) lowers to the enters program's
// OpChooseType with B = kop.ChooseTypeBasicLand, the mid-resolution Basic
// Land ask's marker (gorge effChooseType asks over the same
// TypeChoiceLabels("Basic Land") whether the choice is as-enters or
// mid-resolution); the as-enters creature-type choice keeps B = 0. A
// spelling the kernel's five-label ask cannot answer (a narrowed pool, a
// random pick, another chooser) refuses loudly.
func TestChooseTypeShapes(t *testing.T) {
	const basic = "Name:Basic Probe\nManaCost:U\nTypes:Creature Vedalken Wizard\nPT:1/1\n" +
		"K:ETBReplacement:Other:DBPickLand\n" +
		"SVar:DBPickLand:DB$ ChooseType | Type$ Basic Land | AILogic$ MostNeededType | SpellDescription$ As CARDNAME enters, choose a basic land type.\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | Affected$ Land.YouCtrl | Description$ Your lands gain the chosen type.\n"
	lc := lowerSynth(t, basic)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the as-enters Basic Land probe refuses: %q", keclReason(r))
	}
	ins := chooseTypeInstrs(lc)
	if len(ins) != 1 || ins[0].B.N != kop.ChooseTypeBasicLand || ins[0].C != 0 {
		t.Fatalf("as-enters Basic Land lowers to %+v, want one ChooseType with B = ChooseTypeBasicLand", ins)
	}

	// The control: the creature-type spelling of the same shape keeps its
	// unmarked ask, so the assertion above can fail.
	creature := strings.Replace(basic, "Type$ Basic Land", "Type$ Creature", 1)
	ins = chooseTypeInstrs(lowerSynth(t, creature))
	if len(ins) != 1 || ins[0].B.N != 0 || ins[0].C != 0 {
		t.Fatalf("as-enters creature-type choice lowers to %+v, want one unmarked ChooseType", ins)
	}

	// A spelling the five-label ask cannot answer refuses, never lowers
	// silently as a plain Basic Land ask.
	for name, src := range map[string]string{
		"AtRandom":     strings.Replace(basic, "Type$ Basic Land", "Type$ Basic Land | AtRandom$ True", 1),
		"InvalidTypes": strings.Replace(basic, "Type$ Basic Land", "Type$ Basic Land | InvalidTypes$ Island", 1),
		"ValidTypes":   strings.Replace(basic, "Type$ Basic Land", "Type$ Basic Land | ValidTypes$ Island,Swamp", 1),
		"Defined":      strings.Replace(basic, "Type$ Basic Land", "Type$ Basic Land | Defined$ Player.Opponent", 1),
	} {
		if len(lowerSynth(t, src).Refusals) == 0 {
			t.Errorf("as-enters Basic Land with %s lowers, want a refusal", name)
		}
	}
}
