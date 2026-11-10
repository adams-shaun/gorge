package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedManyCapacity: a many-object delayed program (Hide on the
// Ceiling, Temporal Tribulation, Eerie Interlude) carries four objects. Four
// remembered register and fire; a fifth is a declared StallDProg capacity
// stall, never a silent partial return of the set.
func TestDelayedManyCapacity(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Golem", Types: kop.TCreature})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(c, 0)
	s := sc.s
	s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, src, c
	in := kop.Instr{Op: kop.OpDelayedProgram, Obj: kop.RefRemembered, C: 1}
	in.B.N = kop.DpMany

	s.g.nrem = 0
	for i := 0; i < 4; i++ {
		s.remember(sc.perm(c, 0))
	}
	n0 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n0+1 || s.g.stalled != 0 {
		t.Fatalf("four objects: %d programs (want %d), stalled %d (want 0)", s.g.ndp, n0+1, s.g.stalled)
	}
	for i, o := range s.g.dp[n0].objs {
		if o == NoObj {
			t.Fatalf("four objects: objs[%d] is empty", i)
		}
	}

	s.remember(sc.perm(c, 0))
	if s.g.nrem != 5 {
		t.Fatalf("%d objects remembered, want 5", s.g.nrem)
	}
	n1 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n1 || s.g.stalled != StallDProg {
		t.Fatalf("five objects: %d programs (want %d registered none), stalled %d (want StallDProg %d)", s.g.ndp, n1, s.g.stalled, StallDProg)
	}
}
