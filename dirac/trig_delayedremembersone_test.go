package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedRemembersOne: a delayed program registered over the resolution's
// remembered set (OpDelayedProgram, RefRemembered) keeps one object and the
// version it has now, however many the resolution remembered. That is why a
// lowering that registers over a remembered set must prove it holds one
// object (U.S.S. Farragut's single hand-to-battlefield choice) and refuses a
// set that can hold several (Dragonhawk's exiled cards).
func TestDelayedRemembersOne(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Golem", Types: kop.TCreature})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(c, 0)
	first := sc.perm(c, 0)
	second := sc.perm(c, 0)
	s := sc.s
	s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, src, c
	s.g.nrem = 0
	s.remember(first)
	s.remember(second)
	if s.g.nrem != 2 {
		t.Fatalf("%d objects remembered, want 2", s.g.nrem)
	}
	in := kop.Instr{Op: kop.OpDelayedProgram, Obj: kop.RefRemembered, C: 1}
	n0 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n0+1 {
		t.Fatalf("%d delayed programs, want %d", s.g.ndp, n0+1)
	}
	d := s.g.dp[n0]
	if d.objs[0] != first || d.vers[0] != s.objs[first].ver || d.ab != 1 || d.src != src {
		t.Fatalf("program objs[0] %d ver %d ab %d src %d, want the first remembered card %d at ver %d, ab 1, src %d",
			d.objs[0], d.vers[0], d.ab, d.src, first, s.objs[first].ver, src)
	}
	for i, o := range d.objs[1:] {
		if o == second {
			t.Fatalf("the second remembered card is also held (objs[%d])", i+1)
		}
	}

	// nothing remembered: the program remembers no object
	s.g.nrem = 0
	n1 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n1+1 || s.g.dp[n1].objs[0] != NoObj {
		t.Fatalf("with nothing remembered: %d programs, last objs[0] %d, want an empty program", s.g.ndp, s.g.dp[n1].objs[0])
	}
}
