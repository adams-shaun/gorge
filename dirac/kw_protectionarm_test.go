package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestProtectionOverrideArmsDamage: an op-override record (Table.Overrides
// is indexed 0-based by the program's C) that grants protection arms the
// damage sites' rbProtCol bit on its card, including the record at index 0.
func TestProtectionOverrideArmsDamage(t *testing.T) {
	for _, idx := range []int{0, 1} {
		ovs := []kop.Override{{}, {}}
		ovs[idx].ProtFrom = 1
		prog := []kop.Instr{{Op: kop.OpOverride, C: int16(idx)}}
		tab := &kop.Table{
			Cards:     []kop.CardDef{{Abilities: []kop.Ability{{Prog: prog}}}, {}},
			Overrides: ovs,
		}
		g := &genPrep{replBits: make([]uint32, 2)}
		prepProtGrant(tab, g)
		if g.replBits[0]&rbProtCol == 0 {
			t.Errorf("override index %d: the card's damage bit is unarmed", idx)
		}
		if g.replBits[1]&rbProtCol != 0 {
			t.Errorf("override index %d: a card with no program armed the bit", idx)
		}
		tab.Overrides[idx].ProtFrom = 0
		g = &genPrep{replBits: make([]uint32, 2)}
		prepProtGrant(tab, g)
		if g.replBits[0]&rbProtCol != 0 {
			t.Errorf("override index %d: a record without protection armed the bit", idx)
		}
	}
}

// TestProtectionOverrideEmblemArmsDamage: an emblem has no replBits row, so
// its protection grant opens the damage sites table-wide through anyRepl.
func TestProtectionOverrideEmblemArmsDamage(t *testing.T) {
	prog := []kop.Instr{{Op: kop.OpOverride, C: 0}}
	tab := &kop.Table{
		Cards:     []kop.CardDef{{}},
		Overrides: []kop.Override{{ProtFrom: 1}},
		Emblems:   []kop.Emblem{{Abilities: []kop.Ability{{Prog: prog}}}},
	}
	g := &genPrep{replBits: make([]uint32, 1)}
	prepProtGrant(tab, g)
	if g.anyRepl&rbProtCol == 0 {
		t.Fatal("an emblem's override protection left the damage sites closed")
	}
}

// TestProtColKwOfRejectsFilterTerms: a colour clause carrying a term beyond
// a plain colour mask projects no colour bit.
func TestProtColKwOfRejectsFilterTerms(t *testing.T) {
	plain := kop.Filter{Colors: kop.ColB}
	cases := map[string]func(f *kop.Filter){
		"plain":   nil,
		"notsubs": func(f *kop.Filter) { f.NotSubs[0] = 1 },
		"predv":   func(f *kop.Filter) { f.PredV = kop.Const(3) },
		"each":    func(f *kop.Filter) { f.Each = true },
	}
	for name, mod := range cases {
		f := plain
		if mod != nil {
			mod(&f)
		}
		k := &prep{t: &kop.Table{Filters: []kop.Filter{{}, f}}}
		got := protColKwOf(k, 1)
		if name == "plain" && got != kop.ColB || name != "plain" && got != 0 {
			t.Errorf("%s: protColKwOf = %v", name, got)
		}
	}
}
