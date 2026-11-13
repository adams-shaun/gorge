package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newStratBetrayScen is a matchup over the stratbetray pool, libraries of
// Plains; it returns Strategic Betrayal's lowered Battlefield,Graveyard sweep.
func newStratBetrayScen(t *testing.T) (*scen, map[string]CardID, kop.Instr) {
	t.Helper()
	set := cardsettest.Set(t, "stratbetray")
	deck := []cardset.Entry{
		{Name: "Strategic Betrayal", Count: 1}, {Name: "Diregraf Ghoul", Count: 1},
		{Name: "Walking Corpse", Count: 1}, {Name: "Vampire Nighthawk", Count: 1},
		{Name: "Sengir Vampire", Count: 1}, {Name: "Thought Scour", Count: 1},
		{Name: "Swamp", Count: 1}, {Name: "Island", Count: 1}, {Name: "Plains", Count: 31},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for _, a := range tbl.Cards[id["Strategic Betrayal"]].Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpChangeZoneAllEx && in.B.N == kop.CzBattleGrave {
				return sc, id, in
			}
		}
	}
	t.Fatal("Strategic Betrayal has no CzBattleGrave OpChangeZoneAllEx")
	return nil, nil, kop.Instr{}
}

// TestChangeZoneAllBattleGrave: the Battlefield,Graveyard sweep exiles the
// chosen permanent, then the targeted player's graveyard in graveyard order;
// the target's other battlefield permanent and the other seat's battlefield
// and graveyard are untouched. Gorge's effChangeZoneAll walks the origin
// zone-major (battlefield first, one simultaneous departure), which is what
// the kernel's CzBattleGrave B encodes.
func TestChangeZoneAllBattleGrave(t *testing.T) {
	sc, id, in := newStratBetrayScen(t)
	s := sc.s
	if in.C>>3&kop.MvGraveExile == 0 || in.B.N != kop.CzBattleGrave || kop.Zone(in.C&7) != kop.ZoneExile {
		t.Fatalf("lowered sweep %+v is not the Battlefield,Graveyard exile form", in)
	}

	// seat 1: two creatures on the battlefield, three cards in the graveyard
	chosen := sc.perm(id["Diregraf Ghoul"], 1)
	other := sc.perm(id["Walking Corpse"], 1)
	if !sc.onBF(chosen) || !sc.onBF(other) || chosen == other {
		t.Fatalf("seat 1 setup: chosen %v other %v", sc.onBF(chosen), sc.onBF(other))
	}
	g1 := sc.grave(id["Vampire Nighthawk"], 1, 1)[0]
	g2 := sc.grave(id["Sengir Vampire"], 1, 1)[0]
	g3 := sc.grave(id["Thought Scour"], 1, 1)[0]
	if n := s.pl[1].graveN; n != 3 {
		t.Fatalf("seat 1 graveyard holds %d cards, want 3", n)
	}

	// seat 0: one creature on the battlefield, two cards in the graveyard
	own := sc.perm(id["Sengir Vampire"], 0)
	h1 := sc.grave(id["Walking Corpse"], 0, 1)[0]
	h2 := sc.grave(id["Diregraf Ghoul"], 0, 1)[0]
	if !sc.onBF(own) || s.pl[0].graveN != 2 {
		t.Fatalf("seat 0 setup: creature %v graveyard %d", sc.onBF(own), s.pl[0].graveN)
	}

	// the parent ChooseCard picked `chosen` and made seat 1 the player target
	s.g.chosen[0], s.g.nchosen = chosen, 1
	s.res.active = true
	s.res.e = stackEntry{kind: seSpell, ctrl: 0, nt: 1}
	s.res.e.tgt[0] = uint8(PlayerTarget0 + 1)
	if s.g.nchosen != 1 || s.g.chosen[0] != chosen || TCode(s.res.e.tgt[0]) != PlayerTarget0+1 {
		t.Fatalf("setup: chosen %v target %d", s.g.nchosen, s.res.e.tgt[0])
	}

	s.execChangeZoneAll(sc.k, &in, false)

	if s.objs[chosen].zone != kop.ZoneExile {
		t.Errorf("the chosen creature is in zone %d, want Exile", s.objs[chosen].zone)
	}
	if s.objs[other].zone != kop.ZoneBattlefield {
		t.Errorf("seat 1's unchosen creature is in zone %d, want Battlefield", s.objs[other].zone)
	}
	// the battlefield departure is first, then the graveyard in graveyard order
	want := []Obj{chosen, g1, g2, g3}
	if got := s.pl[1].exile[:s.pl[1].exileN]; !slices.Equal(got, want) {
		t.Errorf("seat 1 exile %v, want battlefield then graveyard order %v", got, want)
	}
	if s.objs[own].zone != kop.ZoneBattlefield {
		t.Errorf("seat 0's creature is in zone %d, want Battlefield", s.objs[own].zone)
	}
	if got := s.pl[0].exile[:s.pl[0].exileN]; len(got) != 0 {
		t.Errorf("seat 0's exile %v, want nothing", got)
	}
	wantGrave := []Obj{h1, h2}
	if got := s.pl[0].grave[:s.pl[0].graveN]; !slices.Equal(got, wantGrave) {
		t.Errorf("seat 0's graveyard %v, want %v", got, wantGrave)
	}
	if s.pl[1].graveN != 0 {
		t.Errorf("seat 1's graveyard holds %d cards, want 0", s.pl[1].graveN)
	}
}
