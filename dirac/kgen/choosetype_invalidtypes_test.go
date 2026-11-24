package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseTypeBasicLandInvalidTypesLowers: an as-enters ChooseType | Type$
// Basic Land | InvalidTypes$ lowers to the enters program's OpChooseType with
// A = the excluded-type mask (bit i = Forest, Island, Mountain, Plains,
// Swamp), the kernel's narrowed ask; shapes whose offer gorge would not pose
// as a five-or-fewer-label ask refuse.
func TestChooseTypeBasicLandInvalidTypesLowers(t *testing.T) {
	const basic = "Name:Basic Probe\nManaCost:U\nTypes:Creature Vedalken Wizard\nPT:1/1\n" +
		"K:ETBReplacement:Other:DBPickLand\n" +
		"SVar:DBPickLand:DB$ ChooseType | Type$ Basic Land | AILogic$ MostNeededType | SpellDescription$ As CARDNAME enters, choose a basic land type.\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | Affected$ Land.YouCtrl | Description$ Your lands gain the chosen type.\n"
	with := func(extra string) string {
		return strings.Replace(basic, "Type$ Basic Land", "Type$ Basic Land | "+extra, 1)
	}

	// Control: no InvalidTypes$ carries no mask.
	ins := chooseTypeInstrs(lowerSynth(t, basic))
	if len(ins) != 1 || ins[0].A.N != 0 || ins[0].B.N != kop.ChooseTypeBasicLand {
		t.Fatalf("plain Basic Land lowers to %+v, want A = 0", ins)
	}

	lc := lowerSynth(t, with("InvalidTypes$ Mountain,Forest,Plains"))
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("InvalidTypes$ Mountain,Forest,Plains refuses: %q", keclReason(r))
	}
	ins = chooseTypeInstrs(lc)
	want := 1<<0 | 1<<2 | 1<<3
	if len(ins) != 1 || ins[0].B.N != kop.ChooseTypeBasicLand || ins[0].C != 0 || ins[0].A.N != want {
		t.Fatalf("lowers to %+v, want one ChooseType with B = ChooseTypeBasicLand, C = 0, A = %d", ins, want)
	}

	for name, extra := range map[string]string{
		"unknown word":  "InvalidTypes$ Wastes",
		"all five":      "InvalidTypes$ Forest,Island,Mountain,Plains,Swamp",
		"four of five":  "InvalidTypes$ Forest,Island,Mountain,Plains",
		"with Valid":    "InvalidTypes$ Forest | ValidTypes$ Island,Swamp",
		"empty element": "InvalidTypes$ Forest,,Swamp",
	} {
		if len(lowerSynth(t, with(extra)).Refusals) == 0 {
			t.Errorf("%s: lowers, want a refusal", name)
		}
	}
}
