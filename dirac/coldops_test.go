package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newBLBScen is newObjScen over the BLB pool: a matchup of deck vs deck,
// libraries of the table's Plains.
func newBLBScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	return newBLBScenTweak(t, deck, nil)
}

// newBLBScenTweak is newBLBScen with tweak run on the matchup table before
// the kernel prepares it (a mechanic a test injects must be in the table the
// mechanic-bit arena is assigned from).
func newBLBScenTweak(t *testing.T, deck []cardset.Entry, tweak func(*kop.Table, map[string]CardID)) (*scen, map[string]CardID) {
	t.Helper()
	return newSetScenTweak(t, "blb", deck, tweak)
}

// newSetScenTweak is newBLBScenTweak on any registered set's table.
func newSetScenTweak(t *testing.T, setName string, deck []cardset.Entry, tweak func(*kop.Table, map[string]CardID)) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, setName)
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the %s table", e.Name, setName)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if tweak != nil {
		tweak(tbl, id)
	}
	pl, ok := id["Plains"]
	if !ok {
		t.Fatal("the scenario deck needs Plains")
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = pl
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// firstInstr is the first instruction with op o in card c's programs.
func firstInstr(t *testing.T, sc *scen, c CardID, o kop.Op) kop.Instr {
	t.Helper()
	for _, a := range sc.k.t.Cards[c].Abilities {
		for _, in := range a.Prog {
			if in.Op == o {
				return in
			}
		}
	}
	t.Fatalf("%s has no %s", sc.k.t.Cards[c].Name, o)
	return kop.Instr{}
}

// TestUntapAll: Valley Floodcaller's "untap them" (gorge effUntapAll)
// untaps the controller's tapped Otters; a stunned one loses the stun
// counter instead (CR 122.1d); the opponent's Otter stays tapped.
func TestUntapAll(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b, c := sc.perm(fc, 0), sc.perm(fc, 0), sc.perm(fc, 1)
	for _, o := range []Obj{a, b, c} {
		sc.s.perms[sc.slot(o)].flags |= pfTapped
	}
	sc.s.perms[sc.slot(b)].setCtrN(sc.s, int(kop.CtrStun), 1)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	in := firstInstr(t, sc, fc, kop.OpUntapAll)
	sc.s.exec(sc.k, &in)
	if sc.tapped(a) {
		t.Error("the controller's tapped Otter stayed tapped")
	}
	if !sc.tapped(b) || sc.s.perms[sc.slot(b)].ctrLow(int(kop.CtrStun)) != 0 {
		t.Error("the stunned Otter: want still tapped, stun counter removed")
	}
	if !sc.tapped(c) {
		t.Error("the opponent's Otter untapped")
	}
}

// TestPutCounterAll: the sweep puts A counters of kind C on each matching
// permanent (gorge putCounterAllSweep) and nothing for A <= 0.
func TestPutCounterAll(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b, c := sc.perm(fc, 0), sc.perm(fc, 0), sc.perm(fc, 1)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	f := firstInstr(t, sc, fc, kop.OpUntapAll).F // Bird/Frog/Otter/Rat you control
	in := kop.Instr{Op: kop.OpPutCounterAll, A: kop.Const(2), C: int16(kop.CtrP1P1), F: f}
	sc.s.exec(sc.k, &in)
	for _, o := range []Obj{a, b} {
		if n := sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)); n != 2 {
			t.Errorf("own Otter has %d +1/+1 counters, want 2", n)
		}
	}
	if n := sc.s.perms[sc.slot(c)].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Errorf("the opponent's Otter has %d counters, want 0", n)
	}
	in.A = kop.Const(0)
	sc.s.exec(sc.k, &in)
	if n := sc.s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Errorf("a zero-count sweep changed the count to %d", n)
	}
}

// TestScryLowered: Rabbit Response's "scry 2" lowers to OpScry, which poses
// gorge's KArrange over the top two cards (raScryTop).
func TestScryLowered(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Rabbit Response", Count: 1}, {Name: "Plains", Count: 8}})
	rr := id["Rabbit Response"]
	in := firstInstr(t, sc, rr, kop.OpScry)
	if in.A != kop.Const(2) {
		t.Fatalf("scry amount %v, want 2", in.A)
	}
	sc.s.res.e.ctrl = 0
	if sc.s.exec(sc.k, &in) {
		t.Fatal("scry 2 did not suspend on a decision")
	}
	if sc.s.res.ask != raScryTop || sc.s.res.peekN != 2 {
		t.Fatalf("ask %d peekN %d, want raScryTop over 2 cards", sc.s.res.ask, sc.s.res.peekN)
	}
}

// TestChangelingMatchesCreatureTypes: Barkform Harvester (Changeling, CR
// 702.73a) has every creature type a filter in its matchup names: Valley
// Floodcaller's "Birds, Frogs, Otters, and Rats you control" includes it on
// the battlefield, and its card record matches in any zone.
func TestChangelingMatchesCreatureTypes(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Barkform Harvester", Count: 1}, {Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc, bh := id["Valley Floodcaller"], id["Barkform Harvester"]
	f := firstInstr(t, sc, fc, kop.OpUntapAll).F
	o := sc.perm(bh, 0)
	if !sc.s.matchPerm(sc.k, f, sc.slot(o), 0, NoSlot) {
		t.Error("the Changeling is not a Bird/Frog/Otter/Rat")
	}
	pl := id["Plains"]
	po := sc.perm(pl, 0)
	if sc.s.matchPerm(sc.k, f, sc.slot(po), 0, NoSlot) {
		t.Error("a Plains matched the creature-type filter")
	}
	if !sc.k.t.Cards[bh].Subtypes.Any() {
		t.Error("the Changeling's card record has no subtypes")
	}
}

// TestSacrificedTrigger: Carrot Cake's "when you sacrifice it" (Mode$
// Sacrificed, gorge sacrificedMatches) subscribes as it enters and queues
// its trigger when it is sacrificed; a destroy does not.
func TestSacrificedTrigger(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Carrot Cake", Count: 2}, {Name: "Plains", Count: 8}})
	cc := id["Carrot Cake"]
	sacAb := uint8(255)
	for j, a := range sc.k.t.Cards[cc].Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSacrificed {
			sacAb = uint8(j)
		}
	}
	if sacAb == 255 {
		t.Fatal("Carrot Cake has no Sacrificed trigger")
	}
	a := sc.perm(cc, 0)
	b := sc.perm(cc, 0)
	if !sc.s.subs[kop.TrigSacrificed].has(sc.slot(a)) {
		t.Fatal("Carrot Cake did not subscribe to TrigSacrificed")
	}
	sc.s.npend = 0
	sl := sc.slot(a)
	sc.s.fireSac(sc.k, sl)
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if sc.s.npend != 1 || sc.s.pend[0].ab != sacAb || sc.s.pend[0].obj != a {
		t.Fatalf("after the sacrifice: %d pending, want Carrot Cake's Sacrificed trigger", sc.s.npend)
	}
	sc.s.npend = 0
	sc.s.destroy(sc.k, sc.slot(b))
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].ab == sacAb {
			t.Fatal("a destroyed Carrot Cake fired its Sacrificed trigger")
		}
	}
}

// TestCountAtLeastUnlessCost: Dazzling Denial counters unless the target's
// controller pays {2}, or {4} if the caster controls a Bird (gorge
// evalCompare, Count$Compare Z GE1.4.2); a Changeling is a Bird.
func TestCountAtLeastUnlessCost(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Dazzling Denial", Count: 1}, {Name: "Barkform Harvester", Count: 1}, {Name: "Plains", Count: 8}})
	dd := id["Dazzling Denial"]
	in := firstInstr(t, sc, dd, kop.OpUnlessPaid)
	if in.Obj != kop.RefTargetController {
		t.Errorf("unless payer %v, want the target's controller", in.Obj)
	}
	sc.s.res.e.ctrl = 0
	if v := sc.s.value(sc.k, in.A); v != 2 {
		t.Fatalf("without a Bird the cost is %d, want 2", v)
	}
	sc.perm(id["Barkform Harvester"], 1)
	if v := sc.s.value(sc.k, in.A); v != 2 {
		t.Fatalf("with the opponent's Bird the cost is %d, want 2", v)
	}
	sc.perm(id["Barkform Harvester"], 0)
	if v := sc.s.value(sc.k, in.A); v != 4 {
		t.Fatalf("with a Bird the cost is %d, want 4", v)
	}
}

// TestDealDamageFrom: Polliwallop's target creature deals twice its power
// to the other target (gorge DealDamage DamageSource$ ParentTarget).
func TestDealDamageFrom(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Polliwallop", Count: 1}, {Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	in := firstInstr(t, sc, id["Polliwallop"], kop.OpDealDamageFrom)
	fc := id["Valley Floodcaller"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 1)
	e := &sc.s.res.e
	e.ctrl = 0
	e.tgt[0], e.tver[0] = TCode(a), sc.s.objs[a].ver
	e.tgt[1], e.tver[1] = TCode(b), sc.s.objs[b].ver
	e.nt = 2
	sc.s.exec(sc.k, &in)
	if d := sc.s.perms[sc.slot(b)].damage; d != 4 {
		t.Fatalf("the target took %d damage, want 4 (twice power 2)", d)
	}
	if d := sc.s.perms[sc.slot(a)].damage; d != 0 {
		t.Fatalf("the source took %d damage (a one-sided deal)", d)
	}
}
