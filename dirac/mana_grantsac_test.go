package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantSacScen: p0 controls Lotus Ring attached to a creature (tapped or
// summoning-sick as asked) and nothing else.
func grantSacScen(t *testing.T, tapped, sick bool) (sc *scen, ring, crt Slot, crtObj Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Lotus Ring", Count: 1}, {Name: "Grand Abolisher", Count: 1}, {Name: "Swamp", Count: 30}}
	tbl, _, _, err := cardsettest.Synth(t, "std-big").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc = newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	ring = sc.slot(sc.perm(id["Lotus Ring"], 0))
	crtObj = sc.perm(id["Grand Abolisher"], 0)
	crt = sc.slot(crtObj)
	sc.s.attachTo(sc.k, ring, crt)
	if tapped {
		sc.s.perms[crt].flags |= pfTapped
	}
	if sick {
		sc.s.perms[crt].flags |= pfSick
	}
	return sc, ring, crt, crtObj
}

// TestGrantSacOnlyPrefix: Lotus Ring gives the equipped creature "Sacrifice
// this creature: Add three mana of any one color". gorge's census lists no
// granted ability, so a {G}{G}{G} play is paid by its prefix search
// activating the grant. The cost has no {T}, so a tapped or a
// summoning-sick creature activates it too (CR 602.2b, 302.6): the kernel's
// search finds that one step, and its replay sacrifices the creature,
// unattaches the Ring at the state-based check (CR 704.5n) and floats {G}{G}{G}.
func TestGrantSacOnlyPrefix(t *testing.T) {
	cost := kop.ManaCost{}
	cost.Pips[kop.G] = 3
	need := grantNeedColours(cost)
	for _, v := range []struct {
		name         string
		tapped, sick bool
	}{{"tapped", true, false}, {"sick", false, true}} {
		t.Run(v.name, func(t *testing.T) {
			sc, ring, crt, crtObj := grantSacScen(t, v.tapped, v.sick)
			s, k := sc.s, sc.k
			// preconditions: the creature holds the granted sacrifice-only
			// ability, cannot tap for mana, the Ring is on it, and the printed
			// sources do not pay the cost
			var gi [maxGrants]int16
			if n := s.grantIdxs(k, crt, &gi); n != 1 || !s.grantView(k.gen, gi[0], crt).grantSac {
				t.Fatalf("the equipped creature: %d granted abilities, want one sacrifice-only", n)
			}
			if s.canTapForMana(k, crt) {
				t.Fatal("the creature can tap for mana: the variant does not exercise the no-{T} cost")
			}
			if s.perms[ring].attached != crt || !sc.onBF(crtObj) {
				t.Fatal("Lotus Ring is not attached to the creature on the battlefield")
			}
			if s.affordsUngrantedFor(k, 0, &cost, NoSlot, payDesc{}) {
				t.Fatal("the printed sources pay {G}{G}{G}: the board does not need the grant")
			}
			var g grantSrch
			if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &g) || g.np != 1 || !g.path[0].sac || g.path[0].col != kop.G || g.path[0].n != 3 {
				t.Fatalf("the grant search: np %d, want one step sacrificing the creature for {G}{G}{G}", g.np)
			}
			s.grantApply(k, &g)
			if z := s.objs[crtObj].zone; z != kop.ZoneGraveyard {
				t.Errorf("the creature is in zone %d after the prefix, want the graveyard", z)
			}
			if s.perms[ring].attached != NoSlot {
				t.Errorf("Lotus Ring still attached to slot %d after the prefix", s.perms[ring].attached)
			}
			if s.pl[0].pool[kop.G] != 3 {
				t.Errorf("pool %v, want {G}{G}{G}", s.pl[0].pool)
			}
		})
	}
}
