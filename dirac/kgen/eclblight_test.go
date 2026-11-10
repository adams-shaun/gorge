package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// keclBuild lowers and builds a card list (the ratchet basis).
func keclBuild(t *testing.T, names []string) ([]*lCard, []built) {
	t.Helper()
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
	return all, bs
}

// keclSynth lowers a synthetic card script and returns its refusals.
func keclSynth(t *testing.T, name, src string) []refusal {
	t.Helper()
	card, diags := cards.ParseBytes("blight_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false).Refusals
}

func keclReason(rs []refusal) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Reason)
	}
	return strings.Join(out, " | ")
}

func keclIndex(t *testing.T, all []*lCard, name string) int {
	t.Helper()
	for i, c := range all {
		if c.Name == name {
			return i
		}
	}
	t.Fatalf("%s: not lowered", name)
	return -1
}

// TestBlightCostLowering: Blight<N> in an activated ability's cost lowers to
// CostBlight with the creature-you-control filter, and the variable and
// two-counted-part shapes refuse by name.
func TestBlightCostLowering(t *testing.T) {
	all, bs := keclBuild(t, []string{"Sting-Slinger"})
	i := keclIndex(t, all, "Sting-Slinger")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Sting-Slinger refused: %v", r)
	}
	c := bs[i].def.Abilities[0].Cost
	if c.Flags&(kop.CostTap|kop.CostBlight) != kop.CostTap|kop.CostBlight {
		t.Errorf("Sting-Slinger cost flags %b, want CostTap|CostBlight", c.Flags)
	}
	if c.N != 1 {
		t.Errorf("Sting-Slinger Blight N = %d, want 1", c.N)
	}
	if c.Filter == 0 {
		t.Error("Sting-Slinger Blight has no creature-you-control filter")
	}

	const body = "A:AB$ GainLife | Cost$ %s | LifeAmount$ 1"
	if got := keclReason(keclSynth(t, "Blight Probe", "Name:Blight Probe\nManaCost:1 B\nTypes:Creature Human\nPT:1/1\n"+strings.Replace(body, "%s", "PayLife<1> Blight<2>", 1)+"\n")); !strings.Contains(got, "blight cost beside another counted part") {
		t.Errorf("PayLife beside Blight: refusals %q, want the counted-part reason", got)
	}
	if got := keclReason(keclSynth(t, "Blight Probe", "Name:Blight Probe\nManaCost:1 B\nTypes:Creature Human\nPT:1/1\n"+strings.Replace(body, "%s", "Blight<X>", 1)+"\n")); !strings.Contains(got, "a variable amount") {
		t.Errorf("Blight<X>: refusals %q, want the variable-amount reason", got)
	}
}

// TestBlightAltAddLowering: K:AlternateAdditionalCost:Blight<N>:M lowers the
// Blight half to SpellCost and the mana half to AltAddCost.
func TestBlightAltAddLowering(t *testing.T) {
	all, bs := keclBuild(t, []string{"Bogslither's Embrace", "Wild Unraveling"})
	for _, tc := range []struct {
		name string
		n    int
		mana int
	}{
		{"Bogslither's Embrace", 1, 3},
		{"Wild Unraveling", 2, 1},
	} {
		i := keclIndex(t, all, tc.name)
		if r := lowerRefusals(all[i]); len(r) != 0 {
			t.Fatalf("%s refused: %v", tc.name, r)
		}
		d := bs[i].def
		var sc kop.Cost
		for _, a := range d.Abilities {
			if a.Kind == kop.AbSpell {
				sc = a.Cost
			}
		}
		if sc.Flags&kop.CostBlight == 0 {
			t.Errorf("%s: SpellCost flags %b, want CostBlight", tc.name, sc.Flags)
		}
		if int(sc.N) != tc.n {
			t.Errorf("%s: SpellCost Blight N = %d, want %d", tc.name, sc.N, tc.n)
		}
		if !d.HasAltAdd || d.AltAddCost.Flags != 0 {
			t.Errorf("%s: alt-add %v flags %b, want a plain mana part", tc.name, d.HasAltAdd, d.AltAddCost.Flags)
		}
		if int(d.AltAddCost.Mana.Generic) != tc.mana {
			t.Errorf("%s: AltAddCost generic = %d, want %d", tc.name, d.AltAddCost.Mana.Generic, tc.mana)
		}
	}
}

// TestBlightResolutionLowering: a trigger's own Blight and an unless Blight
// lower IfPaid/UnlessPaid to a PaySac carrying PayBSubBlight.
func TestBlightResolutionLowering(t *testing.T) {
	all, bs := keclBuild(t, []string{"Blighted Blackthorn", "Gutsplitter Gang"})
	i := keclIndex(t, all, "Blighted Blackthorn")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Blighted Blackthorn refused: %v", r)
	}
	ab := bs[i].def.Abilities
	if len(ab) != 2 {
		t.Fatalf("Blighted Blackthorn has %d abilities, want 2", len(ab))
	}
	for j, a := range ab {
		var got *kop.Instr
		for x := range a.Prog {
			if a.Prog[x].Op == kop.OpIfPaid {
				got = &a.Prog[x]
			}
		}
		if got == nil {
			t.Fatalf("Blighted Blackthorn ability %d: no OpIfPaid", j)
		}
		if got.C&0xf != kop.PaySac || got.B.N != kop.PayBSubBlight || got.A.N != 2 {
			t.Errorf("Blighted Blackthorn ability %d: IfPaid kind %d B %d A %d, want PaySac/PayBSubBlight/2", j, got.C&0xf, got.B.N, got.A.N)
		}
	}

	i = keclIndex(t, all, "Gutsplitter Gang")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Gutsplitter Gang refused: %v", r)
	}
	var got *kop.Instr
	for x := range bs[i].def.Abilities[0].Prog {
		if bs[i].def.Abilities[0].Prog[x].Op == kop.OpUnlessPaid {
			got = &bs[i].def.Abilities[0].Prog[x]
		}
	}
	if got == nil {
		t.Fatal("Gutsplitter Gang: no OpUnlessPaid")
	}
	if got.C&0xf != kop.PaySac || got.B.N != kop.PayBSubBlight || got.A.N != 2 {
		t.Errorf("Gutsplitter Gang: UnlessPaid kind %d B %d A %d, want PaySac/PayBSubBlight/2", got.C&0xf, got.B.N, got.A.N)
	}
}

// TestBlightEffectLowering: DB$ Blight lowers to OpBlight for the defined
// player, and an unknown parameter refuses by name.
func TestBlightEffectLowering(t *testing.T) {
	all, bs := keclBuild(t, []string{"High Perfect Morcant", "Chaos Spewer", "Dose of Dawnglow"})
	i := keclIndex(t, all, "High Perfect Morcant")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("High Perfect Morcant refused: %v", r)
	}
	found := false
	for _, a := range bs[i].def.Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpBlight {
				found = true
				if in.Obj != kop.RefEachOpponent || in.A.N != 1 {
					t.Errorf("High Perfect Morcant: Blight obj %d A %d, want EachOpp/1", in.Obj, in.A.N)
				}
			}
		}
	}
	if !found {
		t.Error("High Perfect Morcant: no OpBlight")
	}

	i = keclIndex(t, all, "Chaos Spewer")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Chaos Spewer refused: %v", r)
	}
	var unless, blight *kop.Instr
	for x := range bs[i].def.Abilities[0].Prog {
		in := &bs[i].def.Abilities[0].Prog[x]
		switch in.Op {
		case kop.OpUnlessPaid:
			unless = in
		case kop.OpBlight:
			blight = in
		}
	}
	if unless == nil || unless.C&0xf != kop.PayMana || unless.A.N != 2 {
		t.Errorf("Chaos Spewer: unless %+v, want a 2-mana payment", unless)
	}
	if blight == nil || blight.Obj != kop.RefController || blight.A.N != 2 {
		t.Errorf("Chaos Spewer: blight %+v, want You/2", blight)
	}

	got := keclReason(keclSynth(t, "Blight Probe", "Name:Blight Probe\nManaCost:1 B\nTypes:Sorcery\nA:SP$ Blight | Defined$ Opponent | Num$ 1 | Frobnitz$ 1\n"))
	if !strings.Contains(got, "Frobnitz") {
		t.Errorf("unknown Blight parameter: refusals %q, want Frobnitz named", got)
	}

	i = keclIndex(t, all, "Dose of Dawnglow")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Errorf("Dose of Dawnglow refused: %v, want its InOwnMainPhase count head lowered", r)
	}
}

// TestCounterCostForms: the counter-removal and counter-placement cost
// spellings lower to the shared CostSubAmong/CostAddCtr forms, and the
// three-field unless AddCounter lowers through its own test.
func TestCounterCostForms(t *testing.T) {
	all, bs := keclBuild(t, []string{"Loch Mare", "Sunstar Chaplain", "Jessica Jones, Private Eye", "Slumbering Walker", "Sanctuary Wall"})

	i := keclIndex(t, all, "Loch Mare")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Loch Mare refused: %v", r)
	}
	for j, want := range []int{1, 2} {
		c := bs[i].def.Abilities[j].Cost
		if c.Flags&kop.CostSubCtrPick == 0 {
			t.Errorf("Loch Mare ability %d: flags %b, want CostSubCtrPick", j, c.Flags)
		}
		if c.Ctr != kop.CtrAll || c.Filter != 0 || int(c.N) != want {
			t.Errorf("Loch Mare ability %d: Ctr %d Filter %d N %d, want CtrAll/0/%d", j, c.Ctr, c.Filter, c.N, want)
		}
	}

	i = keclIndex(t, all, "Sunstar Chaplain")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Sunstar Chaplain refused: %v", r)
	}
	c := bs[i].def.Abilities[0].Cost
	if c.Flags&kop.CostSubCtrPick == 0 || c.Ctr != kop.CtrP1P1 || c.Filter == 0 || c.N != 1 {
		t.Errorf("Sunstar Chaplain: flags %b Ctr %d Filter %d N %d, want CostSubCtrPick/CtrP1P1/filtered/1", c.Flags, c.Ctr, c.Filter, c.N)
	}

	i = keclIndex(t, all, "Jessica Jones, Private Eye")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Jessica Jones refused: %v", r)
	}
	c = bs[i].def.Abilities[0].Cost
	if c.Flags&(kop.CostTap|kop.CostAddCtr) != kop.CostTap|kop.CostAddCtr || c.Ctr != kop.CtrStun || c.N != 1 {
		t.Errorf("Jessica Jones: flags %b Ctr %d N %d, want CostTap|CostAddCtr/CtrStun/1", c.Flags, c.Ctr, c.N)
	}

	// A source-anchored fixed SubCounter keeps CostRemoveCtr.
	synth := "Name:Counter Probe\nManaCost:1 B\nTypes:Creature Human\nPT:1/1\nA:AB$ GainLife | Cost$ SubCounter<1/P1P1> | LifeAmount$ 1\n"
	if r := keclSynth(t, "Counter Probe", synth); len(r) != 0 {
		t.Fatalf("source SubCounter refused: %v", r)
	}

	i = keclIndex(t, all, "Slumbering Walker")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Slumbering Walker refused: %v", r)
	}
	var paid *lInstr
	for x := range all[i].Abilities[0].Prog {
		if all[i].Abilities[0].Prog[x].Op == "IfPaid" {
			paid = &all[i].Abilities[0].Prog[x]
		}
	}
	if paid == nil || paid.CS != "SubCounter" || paid.B.N != int(kop.CtrAll) || paid.A.N != 1 {
		t.Errorf("Slumbering Walker: IfPaid %+v, want SubCounter/CtrAll/1", paid)
	}

	i = keclIndex(t, all, "Sanctuary Wall")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Errorf("Sanctuary Wall refused: %v, want its unless AddCounter lowered", r)
	}
}
