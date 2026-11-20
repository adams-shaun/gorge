package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
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
		// Boomerang Basics carried the bounce's RememberLKI$ until upstream
		// Forge read its condition off Targeted; Nurturing Pixie still does
		{"Nurturing Pixie", kop.OpBounce, func(in kop.Instr) bool { return in.C == kop.MvRemember|kop.MvLKI }, "MvRemember|MvLKI"},
		// upstream's TokenOwner$ TargetedController (was RememberedController)
		{"Unyielding Gatekeeper", kop.OpCreateToken, func(in kop.Instr) bool { return in.Obj == kop.RefTargetController }, "Obj RefTargetController"},
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

// TestRememberedControllerTokenOwner: a token made for the controller of the
// card a RememberLKI$ exile moved builds over kop.RefRememberedController.
// Unyielding Gatekeeper carried the shape until upstream Forge respelled it
// TargetedController (TestRemparamsLowering), and no lowered corpus card
// carries it now, so a synthetic probe keeps it covered.
func TestRememberedControllerTokenOwner(t *testing.T) {
	const probe = `Name:Remembered Owner Probe
ManaCost:1 W
Types:Creature Human
PT:2/2
T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self | Execute$ PrbAway | TriggerDescription$ probe enters
SVar:PrbAway:DB$ ChangeZone | ValidTgts$ Permanent.nonLand+Other | Origin$ Battlefield | Destination$ Exile | RememberLKI$ True | IsCurse$ True | TgtPrompt$ probe target | SubAbility$ PrbMake
SVar:PrbMake:DB$ Token | TokenScript$ wu_2_2_detective | TokenOwner$ RememberedController | SubAbility$ PrbClear
SVar:PrbClear:DB$ Cleanup | ClearRemembered$ True
`
	card, diags := cards.ParseBytes("probe.txt", []byte(probe))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Remembered Owner Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range all {
		if c.Name != name {
			continue
		}
		if r := lowerRefusals(c); len(r) > 0 {
			t.Fatalf("probe refused: %v", r)
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == kop.OpCreateToken && in.Obj == kop.RefRememberedController {
					return
				}
			}
		}
		t.Fatal("probe: no CreateToken over RefRememberedController")
	}
	t.Fatal("probe not lowered")
}
