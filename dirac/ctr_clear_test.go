package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ctrClearCards: an FDN Plains table with test cards appended: a pupil whose
// dies trigger puts its counters on target creature you control
// (OpPutCountersFrom RefTriggerObj), a bear, a free destroy-target
// sorcery, and a "whenever counters are put on a permanent, gain 1 life"
// enchantment.
type ctrClearCards struct{ pupil, bear, destroy, watcher CardID }

func ctrClearScen(t *testing.T) (*scen, ctrClearCards) {
	t.Helper()
	return ctrClearScenHears(t, int16(kop.CtrP1P1))
}

// ctrClearScenHears is ctrClearScen with the watcher listening for counter
// kind hears (kop.TrigNAnyCounter: every kind).
func ctrClearScenHears(t *testing.T, hears int16) (*scen, ctrClearCards) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou}, kop.Filter{Types: kop.TCreature})
	mine, any := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	n := CardID(len(tbl.Cards))
	c := ctrClearCards{pupil: n, bear: n + 1, destroy: n + 2, watcher: n + 3}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Pupil", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig:    kop.Trigger{Event: kop.TrigDies, Flags: kop.TfSelf},
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: mine}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpPutCountersFrom, Obj: kop.RefTarget0, A: kop.Const(1), C: int16(kop.RefTriggerObj)}}}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Destroy", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: any}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}},
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce, N: hears},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// ctrClearKill destroys the pupil with the sorcery and answers the dies
// trigger's target with the bear, then resolves everything.
func (sc *scen) ctrClearKill(t *testing.T, c ctrClearCards, pupil, bear Obj) {
	t.Helper()
	sc.do(t, AkCast, uint8(sc.hand(c.destroy, 0)), 0xff)
	sc.do(t, AkTarget, 0xff, uint8(pupil))
	sc.do(t, AkPass, 0xff, 0xff)
	sc.do(t, AkPass, 0xff, 0xff)
	if sc.onBF(pupil) {
		t.Fatal("the pupil survived the destroy")
	}
	sc.do(t, AkTarget, 0xff, uint8(bear))
	for i := 0; i < 6 && sc.s.StallReason() == 0; i++ {
		sc.do(t, AkPass, 0xff, 0xff)
	}
}

// TestClearCounters: OpClearCounters removes every kind (the +1/+1 count
// and the general slots) but not a Class level, marks the kinds seen, and
// leaves a bare permanent untouched.
func TestClearCounters(t *testing.T) {
	sc, c := ctrClearScen(t)
	bear := sc.perm(c.bear, 0)
	s := sc.s
	p := &s.perms[sc.slot(bear)]
	p.addCtrN(s, int(kop.CtrP1P1), 2)
	p.addCtrN(s, int(kop.CtrStun), 1)
	p.addCtrN(s, int(kop.CtrM1M1), 3)
	p.addCtrN(s, int(kop.CtrLevel), 1)
	s.res.e.tgt[0] = TCode(bear)
	in := kop.Instr{Op: kop.OpClearCounters, Obj: kop.RefTarget0}
	s.clearCounters(sc.k, &in)
	if p.ctrLow(int(kop.CtrP1P1)) != 0 || p.ctrLow(int(kop.CtrStun)) != 0 || p.ctrLow(int(kop.CtrM1M1)) != 0 {
		t.Fatalf("counters left: +1/+1 %d stun %d -1/-1 %d", p.ctrLow(int(kop.CtrP1P1)), p.ctrLow(int(kop.CtrStun)), p.ctrLow(int(kop.CtrM1M1)))
	}
	if p.ctrN(s, int(kop.CtrLevel)) != 1 {
		t.Fatal("the Class level was cleared: a level is not a counter")
	}
	if p.cseen&cseenP1 == 0 {
		t.Fatal("the +1/+1 kind removed to none is not marked seen")
	}
	s.clearCounters(sc.k, &in) // nothing to remove: no change
	if p.ctrN(s, int(kop.CtrLevel)) != 1 {
		t.Fatal("a second clear touched the level")
	}
}

// TestRemoveCounterAll: OpRemoveCounterAll removes n of the kind from
// each matching permanent (the opponent's untouched by a ctrl=You filter),
// every one of the kind with the all flag, and nothing of a kind a
// permanent lacks.
func TestRemoveCounterAll(t *testing.T) {
	sc, c := ctrClearScen(t)
	a, b, opp := sc.perm(c.bear, 0), sc.perm(c.bear, 0), sc.perm(c.bear, 1)
	s := sc.s
	for _, o := range []Obj{a, b, opp} {
		s.perms[sc.slot(o)].addCtrN(s, int(kop.CtrM1M1), 3)
	}
	s.perms[sc.slot(b)].addCtrN(s, int(kop.CtrP1P1), 1)
	mine := kop.FilterID(len(sc.k.t.Filters) - 2) // ctrClearScen: creature you control
	in := kop.Instr{Op: kop.OpRemoveCounterAll, A: kop.Const(1), C: int16(kop.CtrM1M1), F: mine}
	s.removeCounterAll(sc.k, &in)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrM1M1)) != 2 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrM1M1)) != 2 || s.perms[sc.slot(opp)].ctrLow(int(kop.CtrM1M1)) != 3 {
		t.Fatalf("one removed: %d %d, opponent %d", s.perms[sc.slot(a)].ctrLow(int(kop.CtrM1M1)), s.perms[sc.slot(b)].ctrLow(int(kop.CtrM1M1)), s.perms[sc.slot(opp)].ctrLow(int(kop.CtrM1M1)))
	}
	in.B = kop.Const(1)
	s.removeCounterAll(sc.k, &in)
	if s.perms[sc.slot(a)].ctrLow(int(kop.CtrM1M1)) != 0 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrM1M1)) != 0 || s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("the all flag did not clear the kind, or touched another kind")
	}
	in.C = int16(kop.CtrStun)
	s.removeCounterAll(sc.k, &in)
	if s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("a kind nobody holds changed something")
	}
}
