package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// setBlink exiles o and returns it at once: the same card, a new object.
func setBlink(sc *scen, o Obj) {
	sc.s.leave(sc.k, sc.slot(o), kop.ZoneExile, false)
	sc.s.enter(sc.k, o, 0, false, kop.ZoneExile)
	sc.s.npend = 0
}

// TestExecSetLeavesOutReturnedSource: a set read as an object subject (Dragon
// Turtle's "Targeted & Self" under Tap and SkipNextUntap) does not name a
// source that left the battlefield and came back since its ability went on the
// stack (CR 400.7), as a plain Self does not; an unmoved source and a spell's
// own object are still named.
func TestExecSetLeavesOutReturnedSource(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}}
	if i, ok := set.Lookup("Savannah Lions"); !ok || !set.Supported()[i] {
		t.Skip("Savannah Lions not in the FDN table")
	}
	for _, tc := range []struct {
		name            string
		spell, blink    bool
		wantSelfVisited bool
	}{
		{name: "unmoved source", wantSelfVisited: true},
		{name: "returned source", blink: true},
		{name: "spell", spell: true, blink: true, wantSelfVisited: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, da, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			var lions CardID
			for _, c := range da {
				if tbl.Cards[c].Name == "Savannah Lions" {
					lions = c
				}
			}
			tbl.Sets = []kop.SetExpr{{{Kind: kop.SetTarget}, {Kind: kop.SetSelf}, {Kind: kop.SetUnion}}}
			cd := tbl.Cards[lions]
			ab := len(cd.Abilities)
			cd.Abilities = append(cd.Abilities, kop.Ability{
				Prog:    []kop.Instr{{Op: kop.OpTap, Obj: kop.RefSet}, {Op: kop.OpSkipNextUntap, Obj: kop.RefSet}},
				SetRefs: []kop.SetRef{{PC: 0, Set: 0}, {PC: 1, Set: 0}},
			})
			tbl.Cards[lions] = cd
			sc := newScenTable(t, tbl, Config{NoElide: true})
			s := sc.s
			self, tgt := sc.perm(lions, 0), sc.perm(lions, 1)
			e := stackEntry{kind: seTrigger, obj: self, srcVer: s.objs[self].ver, card: lions, ab: uint8(ab), ctrl: 0, trigObj: NoObj}
			if tc.spell {
				e.kind = seSpell
			}
			for i := range e.tgt {
				e.tgt[i] = NoTarget
			}
			e.tgt[0], e.tver[0] = TCode(tgt), s.objs[tgt].ver
			if tc.blink {
				setBlink(sc, self)
			}
			s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
			if !sc.onBF(self) || !sc.onBF(tgt) || (s.objs[self].ver != e.srcVer) != tc.blink {
				t.Fatalf("precondition: both on the battlefield, the source a new object: %v", tc.blink)
			}
			if got := s.objRef(sc.k, kop.RefSelf) != NoSlot; got != tc.wantSelfVisited {
				t.Fatalf("precondition: a plain Self names the source: %v, want %v", got, tc.wantSelfVisited)
			}
			skips := func(o Obj) bool { return s.perms[sc.slot(o)].flags&pfSkipUntap != 0 }
			if sc.tapped(self) || sc.tapped(tgt) || skips(self) || skips(tgt) {
				t.Fatal("precondition: both untapped and unmarked")
			}
			prog := tbl.Cards[lions].Abilities[ab].Prog
			for pc := range prog {
				s.res.pc = uint8(pc)
				if !s.exec(sc.k, &prog[pc]) {
					t.Fatalf("instr %d suspended", pc)
				}
			}
			if !sc.tapped(tgt) || !skips(tgt) {
				t.Errorf("target: tapped %v marked %v, want both", sc.tapped(tgt), skips(tgt))
			}
			if sc.tapped(self) != tc.wantSelfVisited || skips(self) != tc.wantSelfVisited {
				t.Errorf("source: tapped %v marked %v, want %v", sc.tapped(self), skips(self), tc.wantSelfVisited)
			}
		})
	}
}

// TestEvalSetReturnedReferents: a Self or TriggeredCard base that now names a
// new object (CR 400.7) puts no member in a set, while the controller and
// owner readers still see it (gorge CardController), and another base naming
// the new object keeps it.
func TestEvalSetReturnedReferents(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	s := sc.s
	L, S, me, op := TCode(sc.lions), TCode(sc.self), PlayerCode(0), PlayerCode(1)
	self, trig := setOp1(kop.SetSelf, 0), setOp1(kop.SetTriggerObj, 0)
	tgt0, union := setOp1(kop.SetTarget, 0), setOp1(kop.SetUnion, 0)
	ctrlOf, ownerOf := setOp1(kop.SetControllerOf, 0), setOp1(kop.SetOwnerOf, 0)

	setBlink(sc.scen, sc.self)
	if s.objs[sc.self].zone != kop.ZoneBattlefield || s.objs[sc.self].ver == s.res.e.srcVer {
		t.Fatal("precondition: the source is back on the battlefield as a new object")
	}
	setWant(t, "returned Self", sc.eval(self))
	setWant(t, "Target 0 ∪ returned Self", sc.eval(tgt0, self, union), L)
	setWant(t, "returned Self ∪ Target 0", sc.eval(self, tgt0, union), L)
	setWant(t, "returned Self ∪ returned Self", sc.eval(self, self, union))
	setWant(t, "ControllerOf returned Self", sc.eval(self, ctrlOf), me)
	setWant(t, "OwnerOf returned Self", sc.eval(self, ownerOf), me)
	setWant(t, "ControllerOf (Target 0 ∪ returned Self)", sc.eval(tgt0, self, union, ctrlOf), me, op)
	// a target chosen as the new object names it
	s.res.e.tgt[2], s.res.e.tver[2] = TCode(sc.self), s.objs[sc.self].ver
	setWant(t, "Target 2 ∪ returned Self", sc.eval(setOp1(kop.SetTarget, 2), self, union), S)
	setWant(t, "returned Self ∪ Target 2", sc.eval(self, setOp1(kop.SetTarget, 2), union), S)

	P := TCode(sc.plains)
	setWant(t, "TriggeredCard that stayed", sc.eval(trig), P)
	s.res.flags |= rfTrigGone
	if s.objRef(sc.k, kop.RefTriggerObj) != NoSlot {
		t.Fatal("precondition: a plain TriggeredCard names nothing once it left and came back")
	}
	setWant(t, "returned TriggeredCard", sc.eval(trig))
	setWant(t, "returned TriggeredCard ∪ Target 0", sc.eval(trig, tgt0, union), L)
	setWant(t, "ControllerOf returned TriggeredCard", sc.eval(trig, ctrlOf), op)
}
