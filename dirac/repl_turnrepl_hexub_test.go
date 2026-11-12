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

// hexUBBear is the battlefield slot the tests mark with Veil's grant.
const hexUBBear Slot = 3

// newHexUBState lays out a State with hexUBBear controlled by seat 0 and the
// mechHexUB plane armed for Veil's TurnReplHexUB.
func newHexUBState(t *testing.T) (*State, *prep) {
	t.Helper()
	tbl := hexUBTbl()
	k := &prep{gen: &genPrep{}}
	prepArena(tbl, k.gen, &k.cold)
	prepTurnRepl(tbl, k.gen) // anyTurnScope: the cold exMark table is sized
	prepCold(k.gen, &k.cold)
	k.cold.size()
	s := &State{}
	s.layOut(&k.cold)
	s.perms[hexUBBear].ctrl = 0
	s.ctrlM[0].set(hexUBBear)
	return s, k
}

// resolveHexUB resolves one TurnReplHexUB for player pl (seat 0 or 1) and
// returns the record's timestamp.
func resolveHexUB(s *State, k *prep, pl uint8) uint16 {
	s.res.e.ctrl, s.res.e.obj = pl, 1
	s.turnRepl(k, &kop.Instr{Op: kop.OpExt, C: kop.TurnReplHexUB})
	return s.ts
}

// giveHexUBBear gives hexUBBear to player to, as a control change would.
func giveHexUBBear(s *State, to uint8) {
	s.ctrlM[s.perms[hexUBBear].ctrl].clr(hexUBBear)
	s.perms[hexUBBear].ctrl = to
	s.ctrlM[to].set(hexUBBear)
}

// TestHexUBColoursLossControllerScope: the grant belongs to the object, so a
// control change neither takes it nor lets a later Veil of the old controller
// that did not mark this slot refresh its timestamp. The plane mark is the
// first Veil's; a loss of abilities after it takes the grant, a later Veil
// while the permanent was another player's does not put it back.
func TestHexUBColoursLossControllerScope(t *testing.T) {
	ub := kop.ColU | kop.ColB

	// A loss after the grant takes it.
	s, k := newHexUBState(t)
	grant := resolveHexUB(s, k, 0)
	if !s.mSl(k, mechHexUB, hexUBBear) {
		t.Fatal("precondition: seat 0's Veil did not mark the permanent")
	}
	if got := s.hexUBColours(k, hexUBBear); got != ub {
		t.Fatalf("no loss yet: colours %v, want blue and black", got)
	}
	loss := s.tick()
	if loss <= grant {
		t.Fatalf("precondition: loss %d not after the grant %d", loss, grant)
	}
	s.perms[hexUBBear].ovr.flags |= ovLoseAll
	if !s.setLossFx(hexUBBear, loss, true) {
		t.Fatal("setLossFx: pool full")
	}
	if got := s.hexUBColours(k, hexUBBear); got != 0 {
		t.Fatalf("loss after the grant: colours %v, want none", got)
	}

	// The grant survives a control change: gorge's per-object Pump grant is an
	// object-level Card.Self effect, which events.ControlChange does not
	// remove, and a later loss still takes it from the new controller.
	s, k = newHexUBState(t)
	grant = resolveHexUB(s, k, 0)
	giveHexUBBear(s, 1)
	if s.perms[hexUBBear].ctrl != 1 {
		t.Fatal("precondition: the control change did not move the permanent")
	}
	if got := s.hexUBColours(k, hexUBBear); got != ub {
		t.Fatalf("after a control change: colours %v, want blue and black (the object keeps the grant)", got)
	}
	loss = s.tick()
	s.perms[hexUBBear].ovr.flags |= ovLoseAll
	if !s.setLossFx(hexUBBear, loss, true) {
		t.Fatal("setLossFx: pool full")
	}
	if got := s.hexUBColours(k, hexUBBear); got != 0 {
		t.Fatalf("a later loss under the new controller: colours %v, want none", got)
	}

	// A later Veil of the permanent's old controller does not mark it while it
	// is another player's, so the later record's timestamp must not replace the
	// first grant's: a loss between the two Veils still takes the grant.
	s, k = newHexUBState(t)
	grant = resolveHexUB(s, k, 0)
	giveHexUBBear(s, 1)
	loss = s.tick()
	s.perms[hexUBBear].ovr.flags |= ovLoseAll
	if !s.setLossFx(hexUBBear, loss, true) {
		t.Fatal("setLossFx: pool full")
	}
	later := resolveHexUB(s, k, 0)
	if !(grant < loss && loss < later) {
		t.Fatalf("precondition: want grant %d < loss %d < later Veil %d", grant, loss, later)
	}
	giveHexUBBear(s, 0)
	if !s.mSl(k, mechHexUB, hexUBBear) {
		t.Fatal("precondition: the first Veil's plane mark did not survive the control change")
	}
	if got := s.hexUBColours(k, hexUBBear); got != 0 {
		t.Fatalf("a later Veil that did not mark the permanent: colours %v, want none", got)
	}

	// An unmarked permanent has none.
	s.mSlClr(k, mechHexUB, hexUBBear)
	if got := s.hexUBColours(k, hexUBBear); got != 0 {
		t.Fatalf("an unmarked permanent: colours %v, want none", got)
	}
}
