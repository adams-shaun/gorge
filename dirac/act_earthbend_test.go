package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newEBScen is a scenario over the tlaearth pool; ebIn is Earthbending
// Lesson's OpEarthbend (Earthbend 4) retargeted at the land in target slot 0.
func newEBScen(t *testing.T) (sc *scen, id map[string]CardID, ebIn kop.Instr) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Earthbending Lesson", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "tlaearth")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
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
	sc = &scen{s: s, k: s.k()}
	if sc.k.gen == nil || !sc.k.gen.anyEarthbend {
		t.Fatal("anyEarthbend not set for the tlaearth table")
	}
	ebIn = firstInstr(t, sc, id["Earthbending Lesson"], kop.OpEarthbend)
	return sc, id, ebIn
}

// ebCast runs in as Earthbending Lesson's spell targeting land o.
func ebCast(sc *scen, id map[string]CardID, in kop.Instr, o Obj) {
	e := &sc.s.res.e
	e.kind, e.ctrl, e.card, e.ab = seSpell, 0, id["Earthbending Lesson"], 0
	e.tgt[0], e.nt, e.tver[0] = TCode(o), 1, sc.s.objs[o].ver
	sc.s.exec(sc.k, &in)
}

// ebResolve resolves the one pending trigger as the stack would.
func ebResolve(t *testing.T, sc *scen) {
	t.Helper()
	s := sc.s
	if s.npend != 1 || kop.Op(s.pend[0].trigAmt) != kop.OpEarthbendReturn {
		t.Fatalf("%d pending (%+v), want one earthbend return trigger", s.npend, s.pend[0])
	}
	pt := s.pend[0]
	s.npend = 0
	s.resolveDelayed(sc.k, &stackEntry{kind: seTrigger, obj: pt.obj, trigObj: pt.trigObj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl, trigAmt: pt.trigAmt})
}

// TestEarthbendAnimatesAndCounts: Earthbend 4 makes the land a 0/0 hasty
// Creature that stays a land, puts four +1/+1 counters on it, and leaves
// one leave-keyed record.
func TestEarthbendAnimatesAndCounts(t *testing.T) {
	sc, id, in := newEBScen(t)
	s, k := sc.s, sc.k
	f := sc.perm(id["Forest"], 0)
	sl := sc.slot(f)
	if s.isCreature(k, sl) {
		t.Fatal("a Forest is a creature before earthbend")
	}
	ebCast(sc, id, in, f)
	if !s.isCreature(k, sl) || s.typesAt(k, sl)&kop.TLand == 0 {
		t.Fatal("the earthbent land must be a creature and still a land")
	}
	if p, tt := s.pt(k, sl); p != 4 || tt != 4 {
		t.Fatalf("%d/%d, want 0/0 plus four counters", p, tt)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("no haste")
	}
	if s.ndly != 1 || s.dly[0].when != dlyLeave || kop.Op(s.dly[0].act) != kop.OpEarthbendReturn || s.dly[0].obj != f {
		t.Fatalf("delayed %d %+v, want one leave-keyed return record on the land", s.ndly, s.dly[0])
	}
	ebCast(sc, id, in, f) // two earthbends on one land make two records (as gorge)
	if s.ndly != 2 || s.perms[sl].ctrLow(int(kop.CtrP1P1)) != 8 {
		t.Fatalf("delayed %d, counters %d, want 2 records and 8 counters", s.ndly, s.perms[sl].ctrLow(int(kop.CtrP1P1)))
	}
}

func ebReturns(t *testing.T, z kop.Zone) {
	sc, id, in := newEBScen(t)
	s, k := sc.s, sc.k
	f := sc.perm(id["Forest"], 0)
	ebCast(sc, id, in, f)
	s.leave(k, sc.slot(f), z, false)
	if s.objs[f].zone != z {
		t.Fatalf("zone %d, want %d", s.objs[f].zone, z)
	}
	if s.ndly != 0 {
		t.Fatalf("%d records left, want the one consumed", s.ndly)
	}
	ebResolve(t, sc)
	ob := &s.objs[f]
	if ob.zone != kop.ZoneBattlefield || s.perms[ob.slot].flags&pfTapped == 0 {
		t.Fatalf("zone %d: want the land back on the battlefield, tapped", ob.zone)
	}
	if s.isCreature(k, ob.slot) {
		t.Fatal("the returned land is still a creature")
	}
	s.leave(k, ob.slot, z, false) // a second death: the promise is spent
	if s.npend != 0 {
		t.Fatalf("%d pending after a second death, want none", s.npend)
	}
}

// TestEarthbendReturnsTappedFromGraveyard: the dying land returns tapped,
// once.
func TestEarthbendReturnsTappedFromGraveyard(t *testing.T) { ebReturns(t, kop.ZoneGraveyard) }

// TestEarthbendReturnsTappedFromExile: an exiled land returns tapped, once.
func TestEarthbendReturnsTappedFromExile(t *testing.T) { ebReturns(t, kop.ZoneExile) }

// TestEarthbendZeroCountersDiesAndReturns: Earthbend 0 leaves a 0/0 that
// dies to the state-based action and still returns.
func TestEarthbendZeroCountersDiesAndReturns(t *testing.T) {
	sc, id, in := newEBScen(t)
	s, k := sc.s, sc.k
	in.A = kop.Const(0)
	f := sc.perm(id["Forest"], 0)
	ebCast(sc, id, in, f)
	if p, tt := s.pt(k, sc.slot(f)); p != 0 || tt != 0 {
		t.Fatalf("%d/%d, want 0/0", p, tt)
	}
	if s.sba(k); s.objs[f].zone != kop.ZoneGraveyard {
		t.Fatalf("zone %d: want the 0/0 land dead", s.objs[f].zone)
	}
	ebResolve(t, sc)
	if s.objs[f].zone != kop.ZoneBattlefield {
		t.Fatalf("zone %d, want it returned", s.objs[f].zone)
	}
}

// TestEarthbendRecordSurvivesEndStep: fireDelayed fires the end-step
// records and keeps the leave-keyed one, in order.
func TestEarthbendRecordSurvivesEndStep(t *testing.T) {
	sc, id, in := newEBScen(t)
	s, k := sc.s, sc.k
	f, g := sc.perm(id["Forest"], 0), sc.perm(id["Forest"], 0)
	s.dly[0] = delayed{act: uint8(kop.OpBounce), ctrl: 0, obj: g, ver: objWord(s.objs[g].ver), creator: NoObj}
	s.ndly = 1
	ebCast(sc, id, in, f)
	s.dly[s.ndly] = delayed{act: uint8(kop.OpBounce), ctrl: 0, obj: g, ver: objWord(s.objs[g].ver), creator: NoObj}
	s.ndly++
	s.fireDelayed(k)
	if s.npend != 2 {
		t.Fatalf("%d end-step triggers, want the two timed records", s.npend)
	}
	if s.ndly != 1 || s.dly[0].when != dlyLeave || s.dly[0].obj != f {
		t.Fatalf("delayed %d %+v, want only the leave-keyed record kept", s.ndly, s.dly[0])
	}
}

// TestEarthbendBounceKeepsPromise: gorge tracks no incarnation, so a land
// bounced and replayed still returns when it dies later.
func TestEarthbendBounceKeepsPromise(t *testing.T) {
	sc, id, in := newEBScen(t)
	s, k := sc.s, sc.k
	f := sc.perm(id["Forest"], 0)
	ebCast(sc, id, in, f)
	s.leave(k, sc.slot(f), kop.ZoneHand, false)
	if s.npend != 0 || s.ndly != 1 {
		t.Fatalf("pending %d, records %d after a bounce, want 0 and 1", s.npend, s.ndly)
	}
	s.detach(f)
	sl := s.enter(k, f, 0, false, kop.ZoneHand)
	s.npend = 0
	s.leave(k, sl, kop.ZoneGraveyard, false)
	ebResolve(t, sc)
	if s.objs[f].zone != kop.ZoneBattlefield {
		t.Fatalf("zone %d, want the replayed land back from the graveyard", s.objs[f].zone)
	}
}
