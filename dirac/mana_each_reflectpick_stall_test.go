package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachWinScen is eachReflectScen plus an {X}{W}-shaped creature in hand ({W}{W}{U}
// through the CR 601.2g window, HasModeCost) and the all-at-once source under
// one reflect carrier.
func eachWinScen(t *testing.T, cfg Config) (*scen, Obj, Obj) {
	t.Helper()
	var spell CardID
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
		spell = CardID(len(tbl.Cards))
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = 2, 1
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test WWU", Types: kop.TCreature, HasModeCost: true, Cost: c, Power: 1, Toughness: 1})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], cfg)
	sc.perm(id["Judge of Currents"], 0)
	b := sc.perm(id["Birds of Paradise"], 0)
	return sc, b, sc.hand(spell, 0)
}

// TestEachReflectedPickIsNotPosedInTheWindow pins a SCOPING decision: gorge's
// reflected pick of an all-at-once source (engine/effects ManaReflected, a KChoose
// whose options are all Kind "mana", ResumeKind "manareflected") is answered by
// the oracle harness internally (dirac/oracle gorge.go internal: allMana ->
// payexec.WindowChoices over windowOwed), never surfaced as a canonical
// decision. The kernel must therefore not pose a ChGenManaChoice for it: a
// live cast window that taps the source floats the production and stalls
// (StallTapExtra), exactly as every other caller does.
func TestEachReflectedPickIsNotPosedInTheWindow(t *testing.T) {
	sc, birds, sp := eachWinScen(t, Config{NoElide: true})
	sl := sc.slot(birds)
	pc := sc.s.manaPCGen(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcEachPick = %d, want 1", r)
	}
	sc.do(t, AkCast, TCode(sp), scAny)
	for i := 0; i < 50 && sc.s.g.stalled == 0; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenManaChoice {
			t.Fatalf("the window posed the reflected pick (gorge's harness answers it internally)")
		}
		if d.Kind == DkPriority {
			break
		}
	}
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("stalled %d, want StallTapExtra", sc.s.g.stalled)
	}
	if !sc.tapped(birds) {
		t.Error("precondition: the window did not tap the source")
	}
}

// TestEachReflectedPickStallsInAProbe: a walk that is no live cast's (no
// build) taps the same source and stalls on the pick, floating the production
// and none of the pick.
func TestEachReflectedPickStallsInAProbe(t *testing.T) {
	sc, birds, _ := eachWinScen(t, Config{NoElide: true})
	if sc.s.buildLive() {
		t.Fatal("precondition: a build is live")
	}
	var wu [kop.NumManaKinds]uint8
	wu[kop.W], wu[kop.U] = 1, 1
	var c kop.ManaCost
	c.Pips[kop.W], c.Pips[kop.U] = 1, 1
	sc.s.g.stalled = 0
	if ok, _ := sc.s.windowTapWalk(sc.k, 0, &c, &c, false, false, false, false, false, false, false); !ok {
		t.Fatal("the probe walk failed")
	}
	if !sc.tapped(birds) {
		t.Fatal("precondition: the walk did not tap the source")
	}
	if sc.s.g.stalled != StallTapExtra {
		t.Errorf("stalled %d, want StallTapExtra", sc.s.g.stalled)
	}
	if got := sc.s.pl[0].pool; got != wu {
		t.Errorf("pool %v, want the production %v and no pick", got, wu)
	}
}
