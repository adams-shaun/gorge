package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStackExileTwoEffectsOneCard: two Effects over one graveyard card
// (two Mission Briefings choosing the same card) each mark the card's
// permission records for the stack-exit replacement. The second mark finds
// the first Effect's record already marked and is not a stall: the card may
// never be cast. Casting it with two marked records live is the stall, as
// gorge's two competing Moved replacements pose the CR 616.1 order ask.
func TestStackExileTwoEffectsOneCard(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Ravine Raider", Count: 1}})
	s := sc.s
	o := s.pl[0].lib[0]
	s.detach(o)
	s.objs[o].card = id["Ravine Raider"]
	s.objs[o].zone = kop.ZoneGraveyard
	grant := func() {
		*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: kop.DurEOT}
		s.g.nmay++
	}
	grant()
	if !s.stackExileGrantOn(o) || s.g.stalled != 0 {
		t.Fatalf("the first mark stalled (%d)", s.g.stalled)
	}
	if s.otherMayMark(0) {
		t.Fatal("a lone marked record reads as a competing one")
	}
	grant()
	if !s.stackExileGrantOn(o) || s.g.stalled != 0 {
		t.Fatalf("the second Effect's mark stalled (%d): the card may never be cast", s.g.stalled)
	}
	for i := 0; i < 2; i++ {
		if s.mayAt(i).until&mayUntilStackExile == 0 {
			t.Fatalf("record %d is unmarked after both Effects marked", i)
		}
	}
	if !s.otherMayMark(0) || !s.otherMayMark(1) {
		t.Fatal("two marked records of one card do not read as competing")
	}
	s.g.on = true
	s.stampMayExile(sc.k, 0)
	if s.g.stalled != StallExileOnMoved {
		t.Fatalf("casting under two marked records: stall %d, want StallExileOnMoved", s.g.stalled)
	}
	s.g.stalled = 0
	s.mayAt(1).ver++ // the second record belongs to an earlier incarnation
	if s.otherMayMark(0) {
		t.Fatal("a stale-version record competes with the current one")
	}
}
