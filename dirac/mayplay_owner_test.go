package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMayPlayOwnerKeepsGrantTurn: a MayPlayOwner record without a surcharge
// keeps its creation turn, so the DurYourNextTurn cleanup ends it at the end
// of the holder's next turn, not at the end of the turn it was made.
// Expedited Inheritance's grant to the damaged creature's controller lives
// through that controller's next turn; zeroing turn dropped it at once.
func TestMayPlayOwnerKeepsGrantTurn(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.Island, 1)
	s.place(o, kop.ZoneExile, false)
	r := mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurYourNextTurn), turn: mayWord(s.turn)}
	s.partitionRecord(&r, o, 0, 0)
	if r.ctrl != 1 {
		t.Fatalf("ctrl %d, want the card owner 1", r.ctrl)
	}
	if r.turn != mayWord(s.turn) {
		t.Fatalf("turn %d, want the creation turn %d", r.turn, s.turn)
	}
	if r.kind&mayKindRaise != 0 {
		t.Fatal("a no-surcharge record carries the raise bit")
	}

	// the end of the creation turn (seat 0's) keeps it for seat 1
	s.g.may[0], s.g.nmay = r, 1
	s.cleanupGen(sc.k)
	if s.FeatMayCount() != 1 {
		t.Fatal("the record ended at the end of the turn it was made")
	}
	// the end of seat 1's next turn (the holder) ends it
	s.turn++
	s.active = 1
	s.cleanupGen(sc.k)
	if s.FeatMayCount() != 0 {
		t.Fatal("the record survived its holder's next turn")
	}

	// the Soul Partition shape overrides turn with the surcharge
	r = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurPermanent), turn: 2}
	s.partitionRecord(&r, o, 0, 2)
	if r.kind&mayKindRaise == 0 || r.turn != 2 {
		t.Fatalf("raise record kind %d turn %d, want the raise 2", r.kind, r.turn)
	}
}
