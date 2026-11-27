package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// animateKernelSurfaceTable builds a table with a Plains, a plainswalker and
// a blocker, plus one appended Override record (the record the Animate /
// RemoveKeyword paths install).
func animateKernelSurfaceTable(t *testing.T, ov kop.Override) (*scen, CardID, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sub := len(tbl.Subtypes)
	tbl.Subtypes = append(append([]string(nil), tbl.Subtypes...), "Plains")
	var sm kop.SubMask
	sm[sub>>6] |= 1 << (sub & 63)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Sub: uint8(sub + 1)})
	f := kop.FilterID(len(tbl.Filters) - 1)
	plains, walker, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Plains", Types: kop.TLand, Subtypes: sm},
		kop.CardDef{Name: "Test Plainswalker", Types: kop.TCreature, Power: 2, Toughness: 2, Landwalk: f},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	ov.RemoveLandwalk = f
	tbl.Overrides = append(tbl.Overrides, ov)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	return sc, plains, walker, bear
}

func applyTestOverride(t *testing.T, sc *scen, obj Obj, idx int) {
	t.Helper()
	s := sc.s
	s.res.e.ctrl, s.res.e.obj = 0, obj
	s.res.e.card = s.objs[obj].card
	s.res.e.srcVer = s.objs[obj].ver
	s.exec(sc.k, &kop.Instr{Op: kop.OpOverride, Obj: kop.RefSelf, C: int16(idx), A: kop.Const(kop.DurEOT)})
}

// TestAnimateRemoveLandwalkOverride: a RemoveKeyword$ Landwalk:Island record
// on a creature removes its printed islandwalk/plainswalk (kop.Override
// .RemoveLandwalk): the blocker can block it again while the removal lasts.
func TestAnimateRemoveLandwalkOverride(t *testing.T) {
	sc, plains, walker, bear := animateKernelSurfaceTable(t, kop.Override{})
	a, b := sc.perm(walker, 0), sc.perm(bear, 1)
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("blocked before the landwalk removal: want blockable")
	}
	sc.perm(plains, 1)
	if sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("the defender controls the land: want unblockable")
	}
	applyTestOverride(t, sc, a, len(sc.k.t.Overrides)-1)
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("after RemoveLandwalk the walker must be blockable")
	}
}

// TestAnimateExileOnLeaveOverride: an Animate Replacements$ ReplaceLeaves
// record (kop.Override.ExileOnLeave) makes the holder's battlefield leave an
// exile instead of a graveyard move (gorge's continuous replacement over the
// animation's lifetime).
func TestAnimateExileOnLeaveOverride(t *testing.T) {
	sc, _, walker, _ := animateKernelSurfaceTable(t, kop.Override{ExileOnLeave: true})
	a := sc.perm(walker, 0)
	sl := sc.slot(a)
	if got := sc.s.dieZone(sc.k, sl); got != kop.ZoneGraveyard {
		t.Fatalf("without the record: dieZone = %v, want graveyard", got)
	}
	applyTestOverride(t, sc, a, len(sc.k.t.Overrides)-1)
	if got := sc.s.dieZone(sc.k, sl); got != kop.ZoneExile {
		t.Fatalf("with ExileOnLeave: dieZone = %v, want exile", got)
	}
}

// TestAnimateAtEOTSetGroups: the any-number Animate's next-end-step
// sacrifices register one grouped delayed trigger (CR 603.7a/b, gorge
// scheduleAtEOTGroup): the first target is the head, later targets are
// followers that never go on the stack themselves.
func TestAnimateAtEOTSetGroups(t *testing.T) {
	sc, _, walker, bear := animateKernelSurfaceTable(t, kop.Override{})
	s := sc.s
	a, b := sc.perm(walker, 0), sc.perm(bear, 1)
	s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: a, trigObj: NoObj}, active: true}
	n0 := s.ndly
	s.res.e.tgt[0], s.res.e.tver[0] = TCode(a), uint8(s.objs[a].ver)
	// outside a union loop's second object the join must not apply: the first
	// object registers the head, an unrelated later call never joins it
	s.atEOTRider(a, false)
	s.res.e.tgt[0], s.res.e.tver[0] = TCode(b), uint8(s.objs[b].ver)
	if s.atEOTUnionJoin(sc.k, &kop.Instr{Op: kop.OpAtEOT, Obj: tgtRef(0), C: 1}) {
		t.Fatal("an exile act joined a sacrifice head")
	}
	if !s.atEOTUnionJoin(sc.k, &kop.Instr{Op: kop.OpAtEOT, Obj: tgtRef(0)}) {
		t.Fatal("the later object did not join the head's promise")
	}
	if s.ndly != n0+2 {
		t.Fatalf("delayed records %d -> %d, want head and one follower", n0, s.ndly)
	}
	if d := s.dly[n0]; d.when != dlyUntracked || d.obj != a {
		t.Fatalf("head record = %+v, want the untracked sacrifice of %d", d, a)
	}
	if d := s.dly[n0+1]; d.when != dlyGrpFollow || d.creator != a || d.obj != b {
		t.Fatalf("follower record = %+v, want a follower of %d", d, a)
	}
}

// TestAnimateAtEOTPlainRiderDoesNotJoin: two plain atEOTRider calls of one
// resolution (a Repeat loop's separate effects, no union set) stay two
// separate promises: only execUnion's later objects join (atEOTUnionJoin).
func TestAnimateAtEOTPlainRiderDoesNotJoin(t *testing.T) {
	sc, _, walker, bear := animateKernelSurfaceTable(t, kop.Override{})
	s := sc.s
	a, b := sc.perm(walker, 0), sc.perm(bear, 1)
	s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: a, trigObj: NoObj}, active: true}
	n0 := s.ndly
	s.g.inLoop = true
	s.atEOTRider(a, false)
	s.atEOTRider(b, false)
	s.g.inLoop = false
	if s.ndly != n0+2 {
		t.Fatalf("delayed records %d -> %d, want two", n0, s.ndly)
	}
	if d := s.dly[n0+1]; d.when == dlyGrpFollow {
		t.Fatalf("second record = %+v, want its own promise, not a follower", d)
	}
}

// TestAnimateExileOnLeaveBounce: the carriers' Replacements$ line names no
// Destination$, so gorge exiles the permanent on EVERY battlefield departure
// ("instead of putting it anywhere else", Geth): a bounce or tuck of the
// animated permanent goes to exile, not the hand or library, and a batch
// departure the kernel does not redirect stalls.
func TestAnimateExileOnLeaveBounce(t *testing.T) {
	for _, z := range []kop.Zone{kop.ZoneHand, kop.ZoneLibrary} {
		sc, _, walker, _ := animateKernelSurfaceTable(t, kop.Override{ExileOnLeave: true})
		a := sc.perm(walker, 0)
		sl := sc.slot(a)
		if got := sc.s.overrideLeaveZone(sc.k, sl, z); got != z {
			t.Fatalf("without the record: leave to %v = %v, want unchanged", z, got)
		}
		applyTestOverride(t, sc, a, len(sc.k.t.Overrides)-1)
		sl = sc.slot(a)
		if got := sc.s.overrideLeaveZone(sc.k, sl, z); got != kop.ZoneExile {
			t.Fatalf("with ExileOnLeave: leave to %v = %v, want exile", z, got)
		}
		sc.s.leave(sc.k, sl, z, false)
		if got := sc.s.objs[a].zone; got != kop.ZoneExile {
			t.Fatalf("leave to %v put the permanent in %v, want exile", z, got)
		}
	}
	sc, _, walker, _ := animateKernelSurfaceTable(t, kop.Override{ExileOnLeave: true})
	a := sc.perm(walker, 0)
	applyTestOverride(t, sc, a, len(sc.k.t.Overrides)-1)
	sc.s.fireLeave(sc.k, sc.slot(a), kop.ZoneHand)
	if sc.s.g.stalled != StallReplLeave {
		t.Fatalf("a batch bounce past the redirect: stalled = %d, want StallReplLeave", sc.s.g.stalled)
	}
}
