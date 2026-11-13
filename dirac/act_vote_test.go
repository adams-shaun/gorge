package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// voteScen builds a cmd5 table holding Council's Judgment and puts the
// spell on the stack under seat 0, with seat 1's Serra Angel and Memnite and
// seat 0's own Ornithopter on the battlefield. The program is not run yet.
func voteScen(t *testing.T) (sc *scen, angel, memnite, thopter Obj) {
	t.Helper()
	set := cardsettest.Set(t, "cmd5")
	deck := []cardset.Entry{{Name: "Plains", Count: 10}, {Name: "Council's Judgment", Count: 1},
		{Name: "Serra Angel", Count: 1}, {Name: "Memnite", Count: 1}, {Name: "Ornithopter", Count: 1}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == name {
				return CardID(i)
			}
		}
		t.Fatalf("%s not in the table", name)
		return 0
	}
	sc = newScenTableLib(t, tbl, id("Plains"), Config{NoElide: true})
	cj := id("Council's Judgment")
	if p := sc.k.t.Cards[cj].Abilities; len(p) == 0 || len(p[0].Prog) == 0 || p[0].Prog[0].Ext() != kop.ExtVote {
		t.Fatalf("Council's Judgment does not lower to ExtVote first: %+v", p)
	}
	angel, memnite = sc.perm(id("Serra Angel"), 1), sc.perm(id("Memnite"), 1)
	thopter = sc.perm(id("Ornithopter"), 0)
	o := sc.obj(cj, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: o, ctrl: 0, card: cj, trigObj: NoObj}
	sc.s.push(e)
	sc.s.res.e = e
	sc.s.res.pc = 0
	sc.s.g.nrem = 0
	return sc, angel, memnite, thopter
}

// voteAsk checks the next decision is p's vote over exactly want, and votes
// for pick.
func voteAsk(t *testing.T, sc *scen, p uint8, pick Obj, want ...Obj) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChVoteCard || d.Player != p {
		t.Fatalf("decision kind %d sub %d player %d, want DkChoose ChVoteCard %d", d.Kind, d.Sub, d.Player, p)
	}
	if int(d.N) != len(want) {
		t.Fatalf("%d ballot options, want %d", d.N, len(want))
	}
	for _, w := range want {
		found := false
		for i := 0; i < int(d.N); i++ {
			found = found || d.Options[i] == MakeAction(AkChoose, TCode(w), 0, 0)
		}
		if !found {
			t.Fatalf("object %d not on the ballot", w)
		}
	}
	sc.do(t, AkChoose, uint8(pick), 0xff)
}

// TestVoteSplitExilesBoth: the caster votes first, then the opponent; one
// vote each is a tie for most, so both permanents are exiled. The caster's
// own permanent is never on the ballot (you don't control).
func TestVoteSplitExilesBoth(t *testing.T) {
	sc, angel, memnite, thopter := voteScen(t)
	sc.s.runProg(sc.k)
	voteAsk(t, sc, 0, angel, angel, memnite)
	voteAsk(t, sc, 1, memnite, angel, memnite)
	if z := sc.s.objs[angel].zone; z != kop.ZoneExile {
		t.Fatalf("Serra Angel in zone %d, want exile", z)
	}
	if z := sc.s.objs[memnite].zone; z != kop.ZoneExile {
		t.Fatalf("Memnite in zone %d, want exile", z)
	}
	if !sc.onBF(thopter) {
		t.Fatal("the caster's own Ornithopter left the battlefield")
	}
}

// TestVoteSameExilesOne: both votes on one permanent exile it alone.
func TestVoteSameExilesOne(t *testing.T) {
	sc, angel, memnite, thopter := voteScen(t)
	sc.s.runProg(sc.k)
	voteAsk(t, sc, 0, memnite, angel, memnite)
	voteAsk(t, sc, 1, memnite, angel, memnite)
	if z := sc.s.objs[memnite].zone; z != kop.ZoneExile {
		t.Fatalf("Memnite in zone %d, want exile", z)
	}
	if !sc.onBF(angel) || !sc.onBF(thopter) {
		t.Fatal("a permanent with fewer votes left the battlefield")
	}
}
