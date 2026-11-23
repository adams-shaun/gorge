package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachReflectCast is eachReflectScen (Birds rewritten to Azorius Chancery's
// "{T}: Add {W}{U}" beside a Mana Flare) with Firdoch Core, a spell, costing
// w {W} and u {U}; the cast is built and parked at its mana step.
func eachReflectCast(t *testing.T, w, u uint8) (*scen, Obj, Slot) {
	t.Helper()
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
		c := &tbl.Cards[id["Firdoch Core"]]
		c.Cost = kop.ManaCost{}
		c.Cost.Pips[kop.W], c.Cost.Pips[kop.U] = w, u
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	b := sc.perm(id["Birds of Paradise"], 0)
	sl := sc.slot(b)
	if pc := sc.s.manaPCGen(sc.k, sl); sc.s.srcEachPick(sc.k, sl, pc) != 1 {
		t.Fatal("precondition: the rewritten Birds are no all-at-once source under a carrier")
	}
	core := sc.hand(id["Firdoch Core"], 0)
	sc.kcastBuild(core, id["Firdoch Core"], 0)
	sc.s.bld().stage = bsMana
	if c := sc.s.buildCost(sc.k); c.Pips[kop.W] != w || c.Pips[kop.U] != u || c.MV() != int(w+u) {
		t.Fatalf("precondition: the cast costs %+v, want %d{W} %d{U}", c, w, u)
	}
	return sc, core, sl
}

// reflectAskOpts is the colours (modes) the pending ctlReflectPick offers.
func reflectAskOpts(t *testing.T, sc *scen) (modes []uint8) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenManaChoice {
		t.Fatalf("decision kind %d sub %d, want the reflected pick's colour ask (DkChoose, ChGenManaChoice)", d.Kind, d.Sub)
	}
	for i := 0; i < int(d.N); i++ {
		modes = append(modes, d.Options[i].Mode())
	}
	return modes
}

// TestEachReflectedPickAskedAtThePlannedTap: the planner taps an all-at-once
// {W}{U} source under one reflect carrier. The tap makes {W}{U} and one more
// mana of a produced type, which the controller picks (CR 106.12a; gorge
// poses "Choose a colour of mana to reflect" with one option per type), so the
// planned payment pauses on that ask instead of stalling, and the answer
// completes the payment.
func TestEachReflectedPickAskedAtThePlannedTap(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, u uint8
		pick uint8 // the answer
		pool [kop.NumManaKinds]uint8
	}{
		{"{W}{W}{U}: the pick is W", 2, 1, 0, [kop.NumManaKinds]uint8{}},
		{"{W}{U}{U}: the pick is U", 1, 2, 1, [kop.NumManaKinds]uint8{}},
	} {
		sc, core, sl := eachReflectCast(t, tc.w, tc.u)
		if sc.s.g.stalled != 0 {
			t.Fatalf("%s: precondition: stalled %d before the payment", tc.name, sc.s.g.stalled)
		}
		modes := reflectAskOpts(t, sc)
		if sc.s.g.stalled != 0 {
			t.Fatalf("%s: the planned tap stalled %d, want a colour ask", tc.name, sc.s.g.stalled)
		}
		if sc.s.ctl != ctlReflectPick {
			t.Fatalf("%s: ctl = %d, want ctlReflectPick", tc.name, sc.s.ctl)
		}
		if len(modes) != 2 || modes[0] != kop.W || modes[1] != kop.U {
			t.Fatalf("%s: options %v, want exactly W and U in WUBRG order", tc.name, modes)
		}
		if sc.s.perms[sl].flags&pfTapped == 0 {
			t.Fatalf("%s: the source is not tapped when asked", tc.name)
		}
		if pool := sc.s.pl[0].pool; pool[kop.W] != 1 || pool[kop.U] != 1 {
			t.Fatalf("%s: pool %v at the ask, want {W}{U} floating", tc.name, pool)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, tc.pick))
		if sc.s.ctl != ctlBuild {
			t.Fatalf("%s: ctl = %d after the answer, want ctlBuild", tc.name, sc.s.ctl)
		}
		sc.s.Step()
		if sc.s.g.stalled != 0 {
			t.Fatalf("%s: stalled %d after the answer", tc.name, sc.s.g.stalled)
		}
		if sc.s.objs[core].zone != kop.ZoneStack {
			t.Fatalf("%s: the cast is in zone %d, want the stack (paid)", tc.name, sc.s.objs[core].zone)
		}
		if pool := sc.s.pl[0].pool; pool != tc.pool {
			t.Fatalf("%s: pool %v after the payment, want it spent exactly", tc.name, pool)
		}
	}
}

// TestEachReflectedPickOneTypeIsNotAsked: a produced mask with one type beside
// {C} has a single candidate (gorge's ManaReflectedCandidates drops {C}), so
// the tap floats that colour with no ask.
func TestEachReflectedPickOneTypeIsNotAsked(t *testing.T) {
	sc, core, sl := eachReflectCast(t, 2, 0)
	pc := sc.s.manaPCGen(sc.k, sl)
	if m := eachPickMask(pc) &^ (1 << kop.C); m != 1<<kop.W|1<<kop.U {
		t.Fatalf("precondition: pick mask %b", m)
	}
	// {C}{W}: one non-{C} type
	pc.manaCol, pc.manaXv = kop.ColW, [kop.NumManaKinds]uint8{kop.C: 1}
	if m := eachPickMask(pc) &^ (1 << kop.C); m != 1<<kop.W {
		t.Fatalf("precondition: rewritten pick mask %b, want W only", m)
	}
	sc.s.Step()
	if sc.s.g.stalled != 0 || sc.s.ctl == ctlReflectPick {
		t.Fatalf("stalled %d ctl %d: a one-type pick needs no ask", sc.s.g.stalled, sc.s.ctl)
	}
	if sc.s.objs[core].zone != kop.ZoneStack {
		t.Fatalf("the cast is in zone %d, want the stack", sc.s.objs[core].zone)
	}
}

// TestEachReflectedPickStallsOnAFork: a probe has no seat to answer the ask.
func TestEachReflectedPickStallsOnAFork(t *testing.T) {
	sc, _, _ := eachReflectCast(t, 2, 1)
	sc.s.pool = NewForkPool()
	sc.s.pool.forks = append(sc.s.pool.forks[:0], sc.s)
	sc.s.pool.depth = 1
	if !sc.s.isFork() {
		t.Fatal("precondition: the state is not a fork")
	}
	sc.s.Step()
	if sc.s.g.stalled != StallTapExtra || sc.s.ctl == ctlReflectPick {
		t.Fatalf("stalled %d ctl %d, want StallTapExtra and no ask", sc.s.g.stalled, sc.s.ctl)
	}
}

// TestEachReflectedPickParksAResolutionPayment: a resolution's planned PayMana
// asks the pick as well (manaSacParks' second arm). payReplanned stops after the
// tap with the pick pending, the production floated and the plan's own guess
// taken back; once the colour is floated (applyReflectPick's step) the same
// payment plans again, taps nothing more and pays.
func TestEachReflectedPickParksAResolutionPayment(t *testing.T) {
	sc, _, sl := eachReflectCast(t, 2, 1)
	s := sc.s
	s.ctl, s.res.active, s.res.ask = ctlResolve, true, raGenPay
	s.g.pickZ, s.g.pickF, s.g.pickP = kop.PayMana, 0, 0
	if !s.manaSacParks() {
		t.Fatal("precondition: a resolution's planned payment does not park")
	}
	var cost kop.ManaCost
	cost.Pips[kop.W], cost.Pips[kop.U] = 2, 1
	if paid := s.payReplanned(sc.k, 0, &cost); paid || !s.reflectPickPending() {
		t.Fatalf("payReplanned paid=%v pending=%v, want it stopped on the pick", paid, s.reflectPickPending())
	}
	if s.g.stalled != 0 || s.perms[sl].flags&pfTapped == 0 {
		t.Fatalf("stalled %d, tapped %v", s.g.stalled, s.perms[sl].flags&pfTapped != 0)
	}
	if pool := s.pl[0].pool; pool[kop.W] != 1 || pool[kop.U] != 1 {
		t.Fatalf("pool %v at the ask, want {W}{U} floating", pool)
	}
	s.ctl = ctlReflectPick
	if modes := reflectAskOpts(t, sc); len(modes) != 2 || modes[0] != kop.W || modes[1] != kop.U {
		t.Fatalf("options %v, want W and U", modes)
	}
	s.poolAdd(s.g.reflP, kop.W, 1) // applyReflectPick's float
	s.g.reflMask, s.g.reflP = 0, 0
	if paid := s.payReplanned(sc.k, 0, &cost); !paid || s.g.stalled != 0 {
		t.Fatalf("the payment after the answer: paid=%v stalled %d", paid, s.g.stalled)
	}
	if pool := s.pl[0].pool; pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("pool %v after paying, want it spent exactly", pool)
	}
}
