package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestConvokeHybrid exercises a convoke cast whose cost has hybrid pips
// ({3}{W/U}{W/U}, Merrow Skyswimmer). A 128-game probe (20 Merrow Skyswimmer
// against a passive land deck) measured 0 Merrow casts or asks on the gorge
// side, so the lockstep cannot observe this arm (cause unproven); the kernel still mirrors gorge's
// announcement: convoke picks pay the generic part only, then the hybrid
// pips are chosen at bsPips and paid from the pool.
// hybridScen is a board for a Merrow Skyswimmer cast: two Halo Hoppers can
// pay two of the three generic mana, the lands pay the rest and the hybrid
// pips.
func hybridScen(t *testing.T) (*scen, Obj, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Merrow Skyswimmer", Count: 1},
		{Name: "Halo Hopper", Count: 2},
		{Name: "Plains", Count: 4},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "convoke")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.lands(0, id["Plains"], 2)
	sc.lands(0, id["Island"], 3)
	m := sc.hand(id["Merrow Skyswimmer"], 0)
	h1 := sc.perm(id["Halo Hopper"], 0)
	h2 := sc.perm(id["Halo Hopper"], 0)
	return sc, m, h1, h2
}

// castHybridConvoke casts Merrow, answers the two generic convoke picks and
// the two hybrid pips, and returns the decision after payment.
func castHybridConvoke(t *testing.T, sc *scen, m Obj) Decision {
	t.Helper()
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("first decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	for picks := 0; picks < 2; picks++ {
		var pick Action
		found := false
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkChoose && a.Source() != 0 && a.Mode() == 5 {
				pick, found = a, true
				break
			}
		}
		if !found {
			t.Fatalf("convoke pick %d not offered: %v", picks, d.Options[:d.N])
		}
		sc.s.Apply(pick)
		d = sc.s.Step()
	}
	if d.Kind != DkChoose || d.Sub != ChGenPip {
		t.Fatalf("decision after the picks %v/%d, want the hybrid pip ask", d.Kind, d.Sub)
	}
	for pips := 0; pips < 2; pips++ {
		if d.Kind != DkChoose || d.Sub != ChGenPip {
			t.Fatalf("pip %d: decision %v/%d", pips, d.Kind, d.Sub)
		}
		sc.do(t, AkChoose, 0, 0)
		d = sc.s.Step()
	}
	return d
}

func TestConvokeHybrid(t *testing.T) {
	sc, m, h1, h2 := hybridScen(t)
	d := castHybridConvoke(t, sc, m)
	if d.Kind != DkPriority {
		t.Fatalf("decision after payment %v, want priority", d.Kind)
	}
	sc.passBoth(t)
	if !sc.onBF(m) {
		t.Fatalf("Merrow Skyswimmer did not resolve")
	}
	for _, h := range []Obj{h1, h2} {
		if sc.s.perms[sc.slot(h)].flags&pfTapped == 0 {
			t.Fatalf("convoke creature %d is untapped", h)
		}
	}
}

// TestConvokeExpend: the expend tally moves by the mana plus the convoke
// picks; CastTotalManaSpent (recordSpent) moves by the mana only.
func TestConvokeExpend(t *testing.T) {
	sc, m, _, _ := hybridScen(t)
	sc.k.gen.anyHist = true
	sc.k.gen.anySpent = true
	d := castHybridConvoke(t, sc, m)
	if d.Kind != DkPriority {
		t.Fatalf("decision after payment %v, want priority", d.Kind)
	}
	// {3}{W/U}{W/U}: MV 5, two picks pay two generic, the pool pays 3.
	if got := sc.s.g.spent[0]; got != 5 {
		t.Fatalf("expend tally %d, want 5", got)
	}
	found := false
	for i := 0; i < int(sc.s.sp); i++ {
		if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == m {
			if e.trigAmt != 3 {
				t.Fatalf("CastTotalManaSpent %d, want 3", e.trigAmt)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("Merrow Skyswimmer not on the stack")
	}
}
