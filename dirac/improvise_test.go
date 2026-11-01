package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestImproviseExpend: improvise taps are not mana, so neither the expend
// tally nor CastTotalManaSpent counts them (gorge cast_commit.go: Improvise
// does not spend mana; CountsMana is convoke-only).
func TestImproviseExpend(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Foundry Assembler", Count: 1},
		{Name: "Ornithopter", Count: 2},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "improvise")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.k.gen.anyHist = true
	sc.k.gen.anySpent = true
	sc.lands(0, id["Island"], 3)
	m := sc.hand(id["Foundry Assembler"], 0)
	sc.perm(id["Ornithopter"], 0)
	sc.perm(id["Ornithopter"], 0)
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	for picks := 0; picks < 2; picks++ {
		if d.Kind != DkChoose || d.Sub != ChGenConvoke {
			t.Fatalf("pick %d: decision %v/%d, want the improvise ask", picks, d.Kind, d.Sub)
		}
		var pick Action
		found := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkChoose && a.Source() != 0 && a.Mode() == 6 {
				pick, found = a, true
				break
			}
		}
		if !found {
			t.Fatalf("improvise pick %d not offered: %v", picks, d.Options[:d.N])
		}
		sc.s.Apply(pick)
		d = sc.s.Step()
	}
	if d.Kind != DkPriority {
		t.Fatalf("decision after payment %v, want priority", d.Kind)
	}
	if got := sc.s.g.spent[0]; got != 3 {
		t.Fatalf("expend tally %d, want 3 (the improvise taps are not mana)", got)
	}
	for i := 0; i < int(sc.s.sp); i++ {
		if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == m {
			if e.trigAmt != 3 {
				t.Fatalf("CastTotalManaSpent %d, want 3", e.trigAmt)
			}
			return
		}
	}
	t.Fatalf("Foundry Assembler not on the stack")
}
