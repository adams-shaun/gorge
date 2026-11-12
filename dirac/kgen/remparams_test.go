package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kremparamsAccepted are the carriers that lower and
// build: 21 Std-20 cards plus Titania's Command, Sunfall and Zephyr Sentinel.
var kremparamsAccepted = []string{
	"Elixir", "Clive, Ifrit's Dominant", "Jill, Shiva's Dominant", "Stiltzkin, Moogle Merchant",
	"Hotshot Investigators", "Unyielding Gatekeeper", "Steamcore Scholar",
	"Arid Archway", "Nurturing Pixie", "Neutralize the Guards",
	"Thirst for Identity", "Boomerang Basics", "How to Start a Riot", "Splash Portal",
	"Settle the Wreckage", "The Black Arrow", "Hex Magic", "Thirst for Knowledge",
	"Alpharael, Dreaming Acolyte", "Overwrite the Multiverse", "Wail of War",
	"Titania's Command", "Sunfall", "Zephyr Sentinel",
	"Sunpearl Kirin", "Mirkwood Nurturer", "Wanderwine Farewell", "Bob, Reluctant HYDRA Agent",
	"For the Common Good",              // PumpAll Duration$ UntilYourNextTurn
	"Mouth of the Storm",               // PumpAll Duration$ UntilYourNextTurn with a constant P/T pump
	"Grub's Command", "Sygg's Command", // four modes, four target slots
	"Brigid's Command",                                          // + four targeted modes plus the fight link, five target slots
	"Cultivator Colossus", "Primal Surge", "Tidehollow Sculler", // ForgetOtherRemembered$ hand/Dig writers
}

// kremparamsRefused are the cards that stay refused, with a substring of the
// refusal each must carry.
var kremparamsRefused = []struct{ card, why string }{
	{"Sonic Shrieker", "RememberDamaged$ with a remembered player observed"},
	{"Screaming Nemesis", "RememberDamaged$ with a remembered player observed"},
	{"Vanish into Memory", ""},
	{"Stolen Uniform", "RememberObjects"},
}

// TestRemparamsLowering: the remember/scope params lower to the named
// instruction flags, and the out-of-scope shapes
// refuse.
func TestRemparamsLowering(t *testing.T) {
	names := append([]string{}, kremparamsAccepted...)
	for _, r := range kremparamsRefused {
		names = append(names, r.card)
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range all {
		byName[c.Name] = i
	}
	for _, n := range kremparamsAccepted {
		i, ok := byName[n]
		if !ok {
			t.Errorf("%s: not lowered", n)
			continue
		}
		if r := lowerRefusals(all[i]); len(r) > 0 {
			t.Errorf("%s: refused: %v", n, r)
		}
	}
	for _, r := range kremparamsRefused {
		i, ok := byName[r.card]
		if !ok {
			t.Errorf("%s: not lowered", r.card)
			continue
		}
		hit := false
		for _, x := range lowerRefusals(all[i]) {
			hit = hit || strings.Contains(x.Reason, r.why)
		}
		if !hit {
			t.Errorf("%s: refusals %v, want one containing %q", r.card, lowerRefusals(all[i]), r.why)
		}
	}
	// find is the first instruction with op o in card n's programs.
	find := func(n string, o kop.Op) (kop.Instr, bool) {
		i, ok := byName[n]
		if !ok {
			return kop.Instr{}, false
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == o {
					return in, true
				}
			}
		}
		return kop.Instr{}, false
	}
	for _, c := range []struct {
		card string
		op   kop.Op
		ok   func(kop.Instr) bool
		want string
	}{
		{"Hex Magic", kop.OpChangeZoneAllEx, func(in kop.Instr) bool { return in.C>>3&kop.MvRemember != 0 }, "MvRemember"},
		{"Splash Portal", kop.OpReturnEx, func(in kop.Instr) bool {
			return in.C>>3&(kop.MvForgetOther|kop.MvRemember) == kop.MvForgetOther|kop.MvRemember
		}, "MvForgetOther|MvRemember"},
		{"Sunpearl Kirin", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember }, "MvRemember"},
		{"Mirkwood Nurturer", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember }, "MvRemember"},
		{"Wanderwine Farewell", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember }, "MvRemember"},
		{"Bob, Reluctant HYDRA Agent", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember }, "MvRemember"},
		{"Boomerang Basics", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember|kop.MvLKI }, "MvRemember|MvLKI"},
		{"Unyielding Gatekeeper", kop.OpCreateToken, func(in kop.Instr) bool { return in.Obj == kop.RefRememberedController }, "Obj RefRememberedController"},
		{"Stiltzkin, Moogle Merchant", kop.OpGainControl, func(in kop.Instr) bool { return in.B.N&kop.GcRemember != 0 }, "GcRemember"},
		{"The Black Arrow", kop.OpDealDamageEx, func(in kop.Instr) bool { return in.C&kop.DmgRemember != 0 }, "DmgRemember"},
		{"Neutralize the Guards", kop.OpPumpAll, func(in kop.Instr) bool { return in.Obj == kop.RefTarget0 }, "Obj RefTarget0 (the player target slot)"},
		{"Thirst for Knowledge", kop.OpDiscardUnless, func(in kop.Instr) bool { return in.F != 0 }, "OpDiscardUnless with the UnlessType$ filter"},
	} {
		in, ok := find(c.card, c.op)
		if !ok {
			t.Errorf("%s: no %s", c.card, c.op)
			continue
		}
		if !c.ok(in) {
			t.Errorf("%s: %s %+v, want %s", c.card, c.op, in, c.want)
		}
	}
}
