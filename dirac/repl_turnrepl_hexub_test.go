package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// hexUBTbl is a table whose one ability carries ExtTurnRepl TurnReplHexUB, so
// the arena carries the mechHexUB plane and nothing else.
func hexUBTbl() *kop.Table {
	in := kop.ExtInstr(kop.ExtTurnRepl)
	in.C = kop.TurnReplHexUB
	return &kop.Table{Cards: []kop.CardDef{{Name: "T", Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Prog: []kop.Instr{in}}}}}}
}

// TestHexUBColoursLossControllerScope: a permanent marked by Veil of Summer
// keeps hexproof from blue and from black only against a loss of abilities
// that is older than its controller's own grant. A later Veil of another
// player must not refresh the timestamp the loss is compared against, and the
// plane marking does not follow a permanent that has changed controller
// (gorge's per-object pump grant is YouCtrl of the Veil's controller).
func TestHexUBColoursLossControllerScope(t *testing.T) {
	ub := kop.ColU | kop.ColB
	const sl = Slot(3)
	newState := func() (*State, *prep) {
		tbl := hexUBTbl()
		k := &prep{gen: &genPrep{}}
		prepArena(tbl, k.gen, &k.cold)
		prepTurnRepl(tbl, k.gen) // anyTurnScope: the cold exMark table is sized
		prepCold(k.gen, &k.cold)
		k.cold.size()
		s := &State{}
		s.layOut(&k.cold)
		s.mSlSet(k, mechHexUB, sl)
		return s, k
	}
	mark := func(s *State, i int, pl uint8, ts uint16) {
		s.cold.exMark[i] = exMarkRec{sl: slotExPlHex, src: 1, live: true, pl: pl, col: ub, ts: ts}
	}

	s, k := newState()
	r0, lts, r1 := s.tick(), s.tick(), s.tick() // seat 0's Veil, a loss, seat 1's Veil
	mark(s, 0, 0, r0)
	mark(s, 1, 1, r1)
	if got := s.hexUBColours(k, sl); got != ub {
		t.Fatalf("no loss yet: colours %v, want blue and black", got)
	}
	// The loss sits between the two Veils: the permanent's own grant (seat
	// 0's, older) is newer-lost, so the grant goes even though seat 1's Veil
	// resolved after the loss. The old max-over-all-records reading compared
	// against seat 1's ts and wrongly kept it.
	s.perms[sl].ovr.flags |= ovLoseAll
	if !s.setLossFx(sl, lts, true) {
		t.Fatal("setLossFx: pool full")
	}
	if got := s.hexUBColours(k, sl); got != 0 {
		t.Fatalf("loss between the two Veils: colours %v, want none", got)
	}
	// The plane marking does not follow the permanent across a controller
	// change: seat 1's records count only while sl is seat 1's.
	s.perms[sl].ctrl = 1
	if got := s.hexUBColours(k, sl); got != ub {
		t.Fatalf("the newer Veil is the controller's own: colours %v, want blue and black", got)
	}

	// A loss newer than every record takes the grant however many Veils
	// there are, and an unmarked permanent has none.
	s, k = newState()
	mark(s, 0, 0, s.tick())
	late := s.tick()
	s.perms[sl].ovr.flags |= ovLoseAll
	if !s.setLossFx(sl, late, true) {
		t.Fatal("setLossFx: pool full")
	}
	if got := s.hexUBColours(k, sl); got != 0 {
		t.Fatalf("a loss newer than the grant: colours %v, want none", got)
	}
	s.mSlClr(k, mechHexUB, sl)
	if got := s.hexUBColours(k, sl); got != 0 {
		t.Fatalf("an unmarked permanent: colours %v, want none", got)
	}
}
