package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// F2TgtPlayerCtrl, F2DefendingCtrl and the stack read of a
// target-gated cost reduction (filter_heads.go).

func newKFHScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Villainous Wrath", Count: 1}, {Name: "Spring Splasher", Count: 1},
		{Name: "Out of Air", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Divination", Count: 1},
		{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "filterheads")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	return sc, id
}

// kfhFilter is the one table filter carrying f2.
func kfhFilter(t *testing.T, sc *scen, f2 kop.FilterFlags2) kop.FilterID {
	t.Helper()
	var got []kop.FilterID
	for i, f := range sc.k.t.Filters {
		if f.Flags2&f2 != 0 {
			got = append(got, kop.FilterID(i))
		}
	}
	if len(got) != 1 {
		t.Fatalf("%d filters carry %#x, want 1", len(got), f2)
	}
	return got[0]
}

// TestFlags2TgtPlayerCtrl: Villainous Wrath's Creature.TargetedPlayerCtrl
// matches the creatures of the resolving entry's player target only, and
// nothing without an entry (gorge: unbound, not guessed).
func TestFlags2TgtPlayerCtrl(t *testing.T) {
	sc, id := newKFHScen(t)
	f := kfhFilter(t, sc, kop.F2TgtPlayerCtrl)
	b0, b1 := sc.slot(sc.perm(id["Grizzly Bears"], 0)), sc.slot(sc.perm(id["Grizzly Bears"], 1))
	m := func(sl Slot) bool { return sc.s.matchPermGen(sc.k, f, sl, 0, NoSlot, layerFull) }
	if m(b0) || m(b1) {
		t.Fatalf("no stack entry: p0's bears %v, p1's bears %v; want neither", m(b0), m(b1))
	}
	sc.s.res.active = true
	sc.s.res.e = stackEntry{kind: seSpell, card: id["Villainous Wrath"], ctrl: 0, nt: 1, trigObj: NoObj}
	sc.s.res.e.tgt[0] = TCode(PlayerTarget0 + 1)
	if m(b0) || !m(b1) {
		t.Fatalf("targeting player 1: p0's bears %v, p1's bears %v; want p1's only", m(b0), m(b1))
	}
	sc.s.res.e.tgt[0] = TCode(PlayerTarget0)
	if !m(b0) || m(b1) {
		t.Fatalf("targeting player 0: p0's bears %v, p1's bears %v; want p0's only", m(b0), m(b1))
	}
	sc.s.res.e.nt = 0
	if m(b0) || m(b1) {
		t.Fatalf("no player target: p0's bears %v, p1's bears %v; want neither", m(b0), m(b1))
	}
	sc.s.res.active = false
}

// TestFlags2DefendingCtrl: Spring Splasher's Creature.ControlledBy
// TriggeredDefendingPlayer matches the non-active player's creatures. Past
// two seats the defending player is the seat the attack event attacked,
// which the attack trigger's check binds (g.atkDef, seat + 1); in a
// two-player game that is the non-active seat, and the two-seat build reads
// it as such without the binding.
func TestFlags2DefendingCtrl(t *testing.T) {
	sc, id := newKFHScen(t)
	f := kfhFilter(t, sc, kop.F2DefendingCtrl)
	b0, b1 := sc.slot(sc.perm(id["Grizzly Bears"], 0)), sc.slot(sc.perm(id["Grizzly Bears"], 1))
	m := func(sl Slot) bool { return sc.s.matchPermGen(sc.k, f, sl, sc.s.active, NoSlot, layerFull) }
	sc.s.active, sc.s.g.atkDef = 0, 1+1
	if m(b0) || !m(b1) {
		t.Fatalf("player 0 active: p0's bears %v, p1's bears %v; want p1's only", m(b0), m(b1))
	}
	sc.s.active, sc.s.g.atkDef = 1, 0+1
	if !m(b0) || m(b1) {
		t.Fatalf("player 1 active: p0's bears %v, p1's bears %v; want p0's only", m(b0), m(b1))
	}
}

// TestCostTargetOKStackSpell: Out of Air costs {2} less while it targets
// (or, before targets, could target) a creature spell on the stack.
func TestCostTargetOKStackSpell(t *testing.T) {
	for _, tc := range []struct {
		spell string
		gen   uint8
	}{{"Grizzly Bears", 0}, {"Divination", 2}} {
		sc, id := newKFHScen(t)
		ooa := id["Out of Air"]
		o := sc.hand(ooa, 0)
		sp := sc.obj(id[tc.spell], 1)
		sc.s.objs[sp].zone = kop.ZoneStack
		sc.s.push(stackEntry{kind: seSpell, obj: sp, ctrl: 1, card: id[tc.spell], trigObj: NoObj})
		base := sc.k.t.Cards[ooa].Cost
		if got := sc.s.adjustCostGen(sc.k, 0, ooa, o, base); got.Generic != tc.gen || got.Pips[1] != 2 {
			t.Fatalf("before targets, %s on the stack: Out of Air costs %+v, want {%d}{U}{U}", tc.spell, got, tc.gen)
		}
		sc.s.ctl = ctlBuild
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: ooa, ctrl: 0, nt: 1, trigObj: NoObj}}
		sc.s.bld().e.tgt[0] = TCode(sp)
		if got := sc.s.adjustCostGen(sc.k, 0, ooa, o, base); got.Generic != tc.gen || got.Pips[1] != 2 {
			t.Fatalf("targeting %s: Out of Air costs %+v, want {%d}{U}{U}", tc.spell, got, tc.gen)
		}
		sc.s.ctl = ctlPriority
	}
}
