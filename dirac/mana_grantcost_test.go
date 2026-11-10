package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantLifeScen: p0 controls Forgotten Monument and Cavernous Maw (a Cave
// whose own ability makes {C}) at the given life, nothing else.
func grantLifeScen(t *testing.T, life int16) (*scen, Slot) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Forgotten Monument", Count: 1}, {Name: "Cavernous Maw", Count: 1}, {Name: "Island", Count: 30}}
	tbl, _, _, err := cardsettest.Synth(t, "std-lci").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.perm(id["Forgotten Monument"], 0)
	maw := sc.slot(sc.perm(id["Cavernous Maw"], 0))
	sc.s.pl[0].life = life
	return sc, maw
}

// TestGrantPayLifePrefix: Forgotten Monument gives Cavernous Maw "{T}, Pay 1
// life: Add one mana of any color". gorge's census lists no granted ability,
// so a {R} play is paid by its prefix search activating the grant, whose
// life is paid (CR 119.4): the kernel's search finds that one step and its
// replay taps the Maw, floats {R} and costs 1 life. With no life to pay
// (life 0) the grant is not activatable, and at life 1 the payment ends the
// game before the play (CR 704.5a), so neither finds a prefix.
func TestGrantPayLifePrefix(t *testing.T) {
	cost := kop.ManaCost{}
	cost.Pips[kop.R] = 1
	need := grantNeedColours(cost)

	sc, maw := grantLifeScen(t, 20)
	s, k := sc.s, sc.k
	// precondition: the Maw holds the granted life-cost ability, and its own {C}
	// does not pay {R}
	var gi [maxGrants]int16
	if n := s.grantIdxs(k, maw, &gi); n != 1 || grantCostLife(s.grantView(k.gen, gi[0], maw)) != 1 {
		t.Fatalf("Cavernous Maw: %d granted abilities, want one costing 1 life", n)
	}
	if s.affordsUngrantedFor(k, 0, &cost, NoSlot, payDesc{}) {
		t.Fatal("the printed sources pay {R}: the board does not need the grant")
	}
	var g grantSrch
	if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &g) || g.np != 1 || g.path[0].gi != gi[0] || g.path[0].col != kop.R {
		t.Fatalf("the grant search: np %d, want one step activating the grant for {R}", g.np)
	}
	before := s.pl[0].life
	s.grantApply(k, &g)
	if s.pl[0].life == before || s.pl[0].life != 19 {
		t.Errorf("life %d after the prefix, want 19 (from %d)", s.pl[0].life, before)
	}
	if s.perms[maw].flags&pfTapped == 0 {
		t.Error("Cavernous Maw untapped after the prefix")
	}
	if s.pl[0].pool[kop.R] != 1 {
		t.Errorf("pool %v, want one {R}", s.pl[0].pool)
	}

	for _, life := range []int16{0, 1} {
		sc, _ := grantLifeScen(t, life)
		if sc.s.grantSearch(sc.k, 0, &cost, NoSlot, payDesc{}, need, &g) {
			t.Errorf("life %d: the grant search pays {R} through the life-cost grant (np %d)", life, g.np)
		}
	}
}
