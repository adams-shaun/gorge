package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTurnUpListedKeepsTheMorphLifeGate: a face-down Zombie Cutthroat
// ("Morph--Pay 5 life", no mana) is offered the turn-up only at 5 or more
// life, whether the offer is priced from the pool or from the untapped sources
// (turnUpListed's second route), and a submitted turn-up below 5 life taps
// nothing and turns nothing up.
func TestTurnUpListedKeepsTheMorphLifeGate(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Zombie Cutthroat", Count: 4},
		{Name: "Swamp", Count: 36},
	}
	set := cardsettest.Set(t, "mbccosts9")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	cut := sc.obj(id["Zombie Cutthroat"], 0)
	sl := sc.s.enterFaceDown(sc.k, cut, 0, kop.MorphMorph)
	land := sc.perm(id["Swamp"], 0)
	if cd := &tbl.Cards[id["Zombie Cutthroat"]]; cd.MorphLife != 5 || cd.MorphFamily != kop.MorphMorph {
		t.Fatalf("Zombie Cutthroat: MorphLife %d family %d, want 5 and Morph", cd.MorphLife, cd.MorphFamily)
	}

	sc.s.pl[0].life = 5
	if n := sc.turnUpOffers(0); n != 1 {
		t.Fatalf("at 5 life: %d turn-ups offered, want 1 (the precondition the gate below depends on)", n)
	}
	sc.s.pl[0].life = 4
	if n := sc.turnUpOffers(0); n != 0 {
		t.Errorf("at 4 life: %d turn-ups offered, want 0", n)
	}
	tapped := sc.s.perms[sc.s.objs[land].slot].flags
	sc.s.turnFaceUp(sc.k, cut, 0)
	if !sc.s.faceDown(sc.k, sl) || sc.s.pl[0].life != 4 {
		t.Errorf("a submitted turn-up at 4 life: face down %v, life %d, want face down at 4", sc.s.faceDown(sc.k, sl), sc.s.pl[0].life)
	}
	if got := sc.s.perms[sc.s.objs[land].slot].flags; got != tapped {
		t.Errorf("a refused turn-up changed the untapped Swamp's flags %#x -> %#x", tapped, got)
	}
	sc.s.pl[0].life = 5
	sc.s.turnFaceUp(sc.k, cut, 0)
	if sc.s.faceDown(sc.k, sl) || sc.s.pl[0].life != 0 {
		t.Errorf("a turn-up at 5 life: face down %v, life %d, want face up at 0", sc.s.faceDown(sc.k, sl), sc.s.pl[0].life)
	}
}
