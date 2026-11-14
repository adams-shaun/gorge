package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAnyNumberKickClosesAtEveryCandidate: a kicked Divine Resilience
// takes "any number of target creatures you control"; gorge's TargetMax$
// is the count of creatures you control, so once both are picked the slot
// closes and the spell is paid with no done-only ask. Before, the kernel's
// maximum was 255 and it posed a target decision holding only done (pair
// 1473 and 13 more of the 10k slice's Pool class).
func TestAnyNumberKickClosesAtEveryCandidate(t *testing.T) {
	deck := []cardset.Entry{{Name: "Divine Resilience", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 6}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.perm(id["Savannah Lions"], 0)
	sc.perm(id["Savannah Lions"], 0)
	dr := sc.hand(id["Divine Resilience"], 0)
	d0 := sc.s.Step()
	cast := false
	for i := 0; i < int(d0.N); i++ {
		if a := d0.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(dr)) && a.Mode() == 1 {
			sc.s.Apply(a)
			cast = true
			break
		}
	}
	if !cast {
		t.Fatal("kicked Divine Resilience not offered")
	}
	for i := 0; i < 2; i++ {
		d := sc.s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("pick %d: decision kind %d, want a target", i, d.Kind)
		}
		for j := 0; j < int(d.N); j++ {
			if d.Options[j].Kind() == AkTarget {
				sc.s.Apply(d.Options[j])
				break
			}
		}
	}
	if d := sc.s.Step(); d.Kind == DkTarget {
		t.Fatalf("a target decision with %d options after every creature is picked", d.N)
	}
	if sc.s.objs[dr].zone != kop.ZoneStack {
		t.Fatalf("Divine Resilience zone %d, want the stack", sc.s.objs[dr].zone)
	}
}
