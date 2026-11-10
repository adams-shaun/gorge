package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelayedRemembersSacrificed: a delayed program registered over the activation cost's
// sacrificed object (RefSacrificed) keeps that card and the version it has
// where it now lies; with no sacrifice record it remembers nothing of the
// sacrifice (the source's empty record).
func TestDelayedRemembersSacrificed(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Golem", Types: kop.TCreature})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(c, 0)
	sac := sc.perm(c, 0)
	if src == sac {
		t.Fatal("source and sacrificed card are one object")
	}
	s := sc.s
	s.res.e.ctrl, s.res.e.obj = 0, src
	s.res.e.li = s.snapLKI(sac)
	s.leave(sc.k, sc.slot(sac), kop.ZoneGraveyard, false)
	if z := s.objs[sac].zone; z != kop.ZoneGraveyard {
		t.Fatalf("sacrificed card in zone %d, want the graveyard", z)
	}
	in := kop.Instr{Op: kop.OpDelayedProgram, Obj: kop.RefSacrificed, C: 1}
	n0 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n0+1 {
		t.Fatalf("%d delayed programs, want %d", s.g.ndp, n0+1)
	}
	if d := s.g.dp[n0]; d.objs[0] != sac || d.vers[0] != s.objs[sac].ver || d.ab != 1 || d.src != src {
		t.Fatalf("program objs[0] %d ver %d ab %d src %d, want the sacrificed card %d at ver %d, ab 1, src %d", d.objs[0], d.vers[0], d.ab, d.src, sac, s.objs[sac].ver, src)
	}

	// no sacrifice record: the program remembers no object
	s.res.e.li = 0
	n1 := s.g.ndp
	s.exec(sc.k, &in)
	if s.g.ndp != n1+1 || s.g.dp[n1].objs[0] != NoObj {
		t.Fatalf("without a record: %d programs, last objs[0] %d, want an empty program", s.g.ndp, s.g.dp[n1].objs[0])
	}
}
