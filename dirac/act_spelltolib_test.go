package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// resolvingSpell sets up a resolving spell: the top card of player 0's
// library moved onto the stack as the resolving entry's object.
func resolvingSpell(t *testing.T) (*State, *prep, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	a := s.pl[0].lib[s.pl[0].libLo]
	s.detach(a)
	s.objs[a].zone = kop.ZoneStack
	s.res.e.kind, s.res.e.obj, s.res.e.ctrl = seSpell, a, 0
	return s, sc.k, a
}

// TestSpellToLib: ExtSpellToLib puts the resolving spell in its owner's
// library and shuffles it (the rng draws), so the finish moves it nowhere.
func TestSpellToLib(t *testing.T) {
	s, k, a := resolvingSpell(t)
	pl := &s.pl[0]
	n0, d0 := pl.libLen(), s.rng.Draws
	s.spellToLib(k)
	if s.objs[a].zone != kop.ZoneLibrary || pl.libLen() != n0+1 || s.rng.Draws == d0 {
		t.Fatalf("zone %d, library %d -> %d, draws %d: want the spell in its owner's library, shuffled", s.objs[a].zone, n0, pl.libLen(), s.rng.Draws-d0)
	}
}

// TestSpellToLibCopy: a spell copy stays on the stack for the finish to
// cease, and the owner's library is shuffled all the same.
func TestSpellToLibCopy(t *testing.T) {
	s, k, a := resolvingSpell(t)
	s.objs[a].of |= ofCopy
	pl := &s.pl[0]
	n0, d0 := pl.libLen(), s.rng.Draws
	s.spellToLib(k)
	if s.objs[a].zone != kop.ZoneStack || pl.libLen() != n0 || s.rng.Draws == d0 {
		t.Fatalf("zone %d, library %d -> %d, draws %d: want the copy on the stack, the library shuffled", s.objs[a].zone, n0, pl.libLen(), s.rng.Draws-d0)
	}
}

// TestSpellToLibFlashback: a flashback spell stays on the stack for the
// finish to exile (CR 702.34a), and the owner's library is still shuffled.
func TestSpellToLibFlashback(t *testing.T) {
	s, k, a := resolvingSpell(t)
	s.res.e.flags |= sfFlashback
	if !s.spellRestExiles(&s.res.e) {
		t.Fatal("precondition: a flashback spell rests in exile")
	}
	pl := &s.pl[0]
	n0, d0 := pl.libLen(), s.rng.Draws
	s.spellToLib(k)
	if s.objs[a].zone != kop.ZoneStack || pl.libLen() != n0 || s.rng.Draws == d0 {
		t.Fatalf("zone %d, library %d -> %d, draws %d: want the flashback spell on the stack, the library shuffled", s.objs[a].zone, n0, pl.libLen(), s.rng.Draws-d0)
	}
}
