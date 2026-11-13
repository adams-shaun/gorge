package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTypedPermissionCastLowered: gorge's cast planner withholds a cast a
// typed permission allows (Muldrotha's per-type statics carry a
// MayPlayPerm), so the oracle lowers it: the witness Island is tapped and
// its {U} floats when Witness Protection asks its target (pairs 2981 6283
// 8099; 1465 for Arbiter of Woe). Before, the kernel took the planner's
// price and tapped nothing until the payment.
func TestTypedPermissionCastLowered(t *testing.T) {
	deck := []cardset.Entry{{Name: "Muldrotha, the Gravetide", Count: 1}, {Name: "Witness Protection", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Island", Count: 10}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	s := sc.s
	sc.perm(id["Muldrotha, the Gravetide"], 0)
	isl := []Obj{sc.perm(id["Island"], 0), sc.perm(id["Island"], 0)}
	sc.perm(id["Eager Trufflesnout"], 1)
	wp := sc.obj(id["Witness Protection"], 0)
	s.place(wp, kop.ZoneGraveyard, false)
	d := s.Step()
	found := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(wp)) {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Witness Protection's graveyard cast not offered: %v", d.Options[:d.N])
	}
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	if tappedN(sc, isl) != 1 || s.pl[0].pool[kop.U] != 1 {
		t.Fatalf("at the target ask: %d Islands tapped, pool %v; want the witness Island's {U} floating", tappedN(sc, isl), s.pl[0].pool)
	}
}

// TestFlashbackUnplannedCastLowered: gorge's cast planner does not price
// Luminous Rebuke (its target-conditional reduction), so its granted
// flashback cast is a potential play the oracle lowers at the full {4}{W}
// before the target (pairs 2770 7576). Before, the kernel lowered only hand
// and permission casts.
func TestFlashbackUnplannedCastLowered(t *testing.T) {
	deck := []cardset.Entry{{Name: "Luminous Rebuke", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	s := sc.s
	lands := []Obj{sc.perm(id["Plains"], 0)}
	for i := 0; i < 4; i++ {
		lands = append(lands, sc.perm(id["Island"], 0))
	}
	sc.perm(id["Eager Trufflesnout"], 1)
	reb := sc.obj(id["Luminous Rebuke"], 0)
	s.place(reb, kop.ZoneGraveyard, false)
	s.grantFB(sc.k, reb)
	d := s.Step()
	found := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(reb)) && a.Mode()&2 != 0 {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the flashback cast is not offered: %v", d.Options[:d.N])
	}
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	if n := tappedN(sc, lands); n != 5 {
		t.Fatalf("at the target ask %d lands tapped, pool %v; want the {4}{W} witness tapped", n, s.pl[0].pool)
	}
}
