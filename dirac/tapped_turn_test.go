package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPermMarkTappedTurn: a Taps FirstTime$ watcher (Captain America, Living
// Legend) fires only on a permanent's first tap this turn. The arena's
// mechTappedTurn mark is set by every real tap after its own Taps triggers,
// cleared at the turn start and as the permanent leaves.
func TestPermMarkTappedTurn(t *testing.T) {
	// filter 0: a creature its controller controls (gorge Creature.YouCtrl)
	tbl := &kop.Table{
		Filters: []kop.Filter{{Types: kop.TCreature, Owner: kop.CtrlYou}},
		Exprs:   []kop.Expr{{Kind: kop.ExConst, N: 1}}, // a generated table: the arena prep runs
		Cards: []kop.CardDef{
			{Name: "Test Plains", Types: kop.TLand},
			{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		},
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	watcher := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test First Taps Watch", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTaps, Filter: 0, Flags2: kop.Tf2FirstTime}, Prog: gain}}})

	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.ix[mechTappedTurn] == arenaNone {
		t.Fatalf("the table's Taps FirstTime$ line did not bring in mechTappedTurn: gen %v", k.gen)
	}
	tap := func(o Obj) {
		t.Helper()
		s.res.e.trigObj = o
		if !s.exec(k, &kop.Instr{Op: kop.OpTap, Obj: kop.RefTriggerObj}) {
			t.Fatal("the tap suspended")
		}
	}
	untap := func(o Obj) {
		t.Helper()
		s.untapEffect(k, s.objs[o].slot)
		if s.perms[s.objs[o].slot].flags&pfTapped != 0 {
			t.Fatal("precondition: the permanent did not untap")
		}
	}
	// fires counts the watcher's queued trigger (nothing is resolved here).
	fires := func() int { return int(s.npend) }

	// Tapped twice in one turn: the watcher fires once.
	bear := sc.perm(1, 0)
	sl := sc.slot(bear)
	if s.mSl(k, mechTappedTurn, sl) {
		t.Fatal("precondition: the untapped permanent already carries the mark")
	}
	sc.perm(watcher, 0) // the watcher enters before the first tap
	tap(bear)
	if !s.mSl(k, mechTappedTurn, sl) {
		t.Fatal("the first tap did not set the tapped-this-turn mark")
	}
	if n := fires(); n != 1 {
		t.Fatalf("first tap: %d watcher triggers, want 1 (the watcher must be live)", n)
	}
	untap(bear)
	if s.mSl(k, mechTappedTurn, sl) == false {
		t.Fatal("an untap must not clear the tapped-this-turn mark")
	}
	s.npend = 0
	tap(bear)
	if n := fires(); n != 0 {
		t.Fatalf("second tap in the same turn: %d watcher triggers, want 0 (FirstTime$)", n)
	}

	// The turn start clears the mark.
	s.permMarkNewTurn(k)
	if s.mSl(k, mechTappedTurn, sl) {
		t.Fatal("the turn start did not clear the tapped-this-turn mark")
	}
	untap(bear)
	s.npend = 0
	tap(bear)
	if n := fires(); n != 1 {
		t.Fatalf("first tap of the next turn: %d watcher triggers, want 1", n)
	}

	// A leave clears it: the permanent that takes the slot is a new object.
	s.leave(k, sl, kop.ZoneHand, false)
	if s.objs[bear].zone != kop.ZoneHand {
		t.Fatalf("precondition: the bear is in zone %d, want the hand", s.objs[bear].zone)
	}
	bear2 := sc.perm(1, 0)
	sl2 := sc.slot(bear2)
	if s.mSl(k, mechTappedTurn, sl2) {
		t.Fatal("the leaving permanent's tapped-this-turn mark survived into the new permanent")
	}
	s.npend = 0
	tap(bear2)
	if n := fires(); n != 1 {
		t.Fatalf("the new permanent's first tap: %d watcher triggers, want 1", n)
	}

	// A permanent tapped before the watcher entered does not fire on its next
	// tap: the watcher may arrive after the first tap (Captain America).
	tbl2 := &kop.Table{
		Filters: []kop.Filter{{Types: kop.TCreature, Owner: kop.CtrlYou}},
		Exprs:   []kop.Expr{{Kind: kop.ExConst, N: 1}},
		Cards: []kop.CardDef{
			{Name: "Test Plains", Types: kop.TLand},
			{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Test Late First Taps Watch", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered,
					Trig: kop.Trigger{Event: kop.TrigTaps, Filter: 0, Flags2: kop.Tf2FirstTime}, Prog: gain}}},
		},
	}
	sc2 := newScenTableLib(t, tbl2, 0, Config{NoElide: true})
	s, k = sc2.s, sc2.k
	if k.gen == nil || k.gen.ix[mechTappedTurn] == arenaNone {
		t.Fatal("precondition: the second table does not use mechTappedTurn")
	}
	early := sc2.perm(1, 0)
	earlySl := sc2.slot(early)
	tap(early)
	if !s.mSl(k, mechTappedTurn, earlySl) {
		t.Fatal("the tap before the watcher entered did not set the mark")
	}
	untap(early)
	sc2.perm(2, 0) // the watcher enters after that tap
	s.npend = 0
	tap(early)
	if n := fires(); n != 0 {
		t.Fatalf("a tap after a pre-entry tap: %d watcher triggers, want 0", n)
	}
	// A fresh creature still fires: the gate is the mark, not the watcher.
	fresh := sc2.perm(1, 0)
	tap(fresh)
	if n := fires(); n != 1 {
		t.Fatalf("a fresh creature's first tap: %d watcher triggers, want 1", n)
	}
}
