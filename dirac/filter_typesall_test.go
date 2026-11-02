package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTypesAllPredicate: PredTypesAll (typesall) matches a permanent only
// when it has EVERY type in PredN -- an artifact creature for
// Artifact|Creature, an artifact land for Land|Artifact -- reading the
// DERIVED types on the battlefield (a type override animating a Mind
// Stone makes it match) and the printed types off it.
func TestTypesAllPredicate(t *testing.T) {
	deck := []cardset.Entry{{Name: "Juggernaut", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Mind Stone", Count: 1},
		{Name: "Seat of the Synod", Count: 1}, {Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "typesall")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.ctl = 2, 0, kop.StepMain1, ctlPriority
	sc := &scen{s: s, k: s.k()}
	add := func(m kop.TypeMask) kop.FilterID {
		sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: m, Pred: kop.PredTypesAll, PredN: int16(m)})
		return kop.FilterID(len(sc.k.t.Filters) - 1)
	}
	artCre, artLand := add(kop.TArtifact|kop.TCreature), add(kop.TLand|kop.TArtifact)
	jug, bear, stone, seat := sc.perm(id["Juggernaut"], 0), sc.perm(id["Grizzly Bears"], 0), sc.perm(id["Mind Stone"], 0), sc.perm(id["Seat of the Synod"], 0)
	for _, c := range []struct {
		f    kop.FilterID
		o    Obj
		want bool
	}{{artCre, jug, true}, {artCre, bear, false}, {artCre, stone, false}, {artCre, seat, false},
		{artLand, seat, true}, {artLand, jug, false}, {artLand, stone, false}} {
		if got := sc.s.matchPerm(sc.k, c.f, sc.slot(c.o), 0, NoSlot); got != c.want {
			t.Errorf("filter %d on %s: got %v want %v", c.f, tbl.Cards[sc.s.objs[c.o].card].Name, got, c.want)
		}
	}
	// the derived read: an animated Mind Stone (a type override, as Ensoul
	// Artifact's AddType$ Creature lands it) is an artifact creature
	p := &sc.s.perms[sc.slot(stone)]
	p.ovr.flags |= ovTypes
	p.ovr.types = kop.TArtifact | kop.TCreature
	if !sc.s.matchPerm(sc.k, artCre, sc.slot(stone), 0, NoSlot) {
		t.Error("an animated artifact is not read as an artifact creature (derived types)")
	}
	// the card path reads the printed types
	if !sc.s.matchCard(sc.k, artCre, id["Juggernaut"], 0, 0) || sc.s.matchCard(sc.k, artCre, id["Mind Stone"], 0, 0) ||
		sc.s.matchCard(sc.k, artCre, id["Grizzly Bears"], 0, 0) || !sc.s.matchCard(sc.k, artLand, id["Seat of the Synod"], 0, 0) ||
		sc.s.matchCard(sc.k, artLand, id["Plains"], 0, 0) {
		t.Error("the card matcher misreads PredTypesAll")
	}
}
