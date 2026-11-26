package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sweepOppsScen builds a scenario over the sweepfrom fixture pool: a vanilla
// creature of power 3 on each seat, one more on the resolving seat, and the
// pool's plain creature filter.
func sweepOppsScen(t *testing.T) (*scen, map[string]CardID, kop.FilterID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "sweepfrom")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	creature := kop.FilterID(len(tbl.Filters) - 1)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	return sc, id, creature
}

// TestSweepFromOpponents: ExtSweepFrom's C&4 player arm (Chandra's Ignition:
// DamageAll DamageSource$ ParentTarget beside ValidPlayers$ Player.Opponent)
// is one damage batch that hits every other creature and then only the
// resolving controller's LIVING OPPONENTS — the controller takes no hit from
// its own sweep, and the source creature's own slot is excluded (C&1). The
// NoSlot arm (the source creature gone) takes the same opponent hits from the
// resolving effect.
func TestSweepFromOpponents(t *testing.T) {
	sc, id, creature := sweepOppsScen(t)
	s := sc.s
	src := sc.perm(id["Hill Giant"], 0) // the sweep source, a 3-power creature
	v0 := sc.perm(id["Grizzly Bears"], 0)
	v1a := sc.perm(id["Grizzly Bears"], 1)
	v1b := sc.perm(id["Colossal Dreadmaw"], 1)
	if !sc.onBF(src) || !sc.onBF(v1a) || !sc.onBF(v1b) {
		t.Fatal("precondition: the permanents did not enter")
	}
	s.res.e = stackEntry{kind: seSpell, ctrl: 0, card: id["Hill Giant"], tgt: [kop.MaxTargets]TCode{TCode(s.perms[sc.slot(src)].obj)}}
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtSweepFrom)),
		Obj: kop.RefTarget0, B: kop.Const(3), C: int16(sweepNotSrc | sweepOpps), F: creature}
	if !s.exec(sc.k, &in) {
		t.Fatal("the sweep suspended")
	}
	dmgOf := func(o Obj) int { return int(s.perms[sc.slot(o)].damage) }
	for _, c := range []struct {
		o   Obj
		dmg int
	}{{src, 0}, {v0, 3}, {v1a, 3}, {v1b, 3}} {
		if got := dmgOf(c.o); got != c.dmg {
			t.Fatalf("damage on the object at slot %v = %d, want %d", c.o, got, c.dmg)
		}
	}
	if s.pl[0].life != 20 {
		t.Fatalf("the resolving controller took %d life of its own sweep, want none", 20-s.pl[0].life)
	}
	if s.pl[1].life != 17 {
		t.Fatalf("the opponent's life = %d, want 17 (one 3-damage player hit)", s.pl[1].life)
	}

	// The source gone: the resolving effect deals the same hits, its player
	// arm opponents-only.
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	if !sc.onBF(v0) {
		t.Fatal("precondition: the other creature left with the source")
	}
	s.res.e = stackEntry{kind: seSpell, ctrl: 0, card: id["Hill Giant"]}
	in = kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtSweepFrom)),
		Obj: kop.RefTarget0, B: kop.Const(2), C: int16(sweepOpps), F: creature}
	if !s.exec(sc.k, &in) {
		t.Fatal("the gone-source sweep suspended")
	}
	for _, o := range []Obj{v0, v1a, v1b} {
		if got := dmgOf(o); got != 5 {
			t.Fatalf("the gone-source sweep put %d damage on the object at slot %v, want 5", got, o)
		}
	}
	if s.pl[0].life != 20 {
		t.Fatalf("the gone-source sweep's player arm hit the controller too: life %d, want 20", s.pl[0].life)
	}
	if s.pl[1].life != 15 {
		t.Fatalf("the opponent's life = %d, want 15 (a second 2-damage player hit)", s.pl[1].life)
	}
}
