package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// kplotScen appends a plot card ({1}{R} for {2}{R}, CR 702.170a) to a copy
// of the vanilla table and returns it with three Mountains on seat 0.
func kplotScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl := *testTable()
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Conds = []kop.Cond{{}} // a non-nil Conds enables prepareGen for the generated arms
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Plotter", Types: kop.TCreature, Power: 2, Toughness: 2,
		Cost:     kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.R: 1}},
		HasPlot:  true,
		PlotCost: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.R: 1}},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.lands(0, v.Mountain, 3)
	return sc, id
}

// kplotRecord returns the live plot record for o, or false.
func kplotRecord(s *State, o Obj) (mayPlay, bool) {
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.obj == o && r.until == kop.DurPlot {
			return r, true
		}
	}
	return mayPlay{}, false
}

// kplotOffers reports whether d offers o's plotted cast (mode castPlotted).
func kplotOffers(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode()&castPlotted != 0 {
			return true
		}
	}
	return false
}

// TestPlotAction: the plot special action is offered at sorcery speed,
// pays the printed plot cost and exiles the card with the plotted
// designation on a DurPlot record; it pushes nothing (not a cast) and the
// same turn offers no plotted cast (CR 702.170a-d).
func TestPlotAction(t *testing.T) {
	sc, id := kplotScen(t)
	o := sc.hand(id, 0)
	d := sc.s.Step()
	if d.Kind != DkPriority || !kplotOffersAction(&d, o) {
		for i := 0; i < int(d.N); i++ {
			t.Logf("opt %d kind %d src %d mode %d", i, d.Options[i].Kind(), d.Options[i].Source(), d.Options[i].Mode())
		}
		t.Fatalf("plot action not offered: kind %d opts %v gen %v anyPlot %v hasPlot %v sorc %v", d.Kind, d.Options[:d.N], sc.k.gen != nil, sc.k.gen != nil && sc.k.gen.anyPlot, sc.k.t.Cards[id].HasPlot, sc.s.sorcerySpeed(0))
	}
	// the action waits for sorcery timing even on a plot card (CR 702.170a)
	s := sc.s
	s.active = 1
	s.step = kop.StepMain1
	var inst Decision
	s.priorityOptions(s.k(), &inst)
	if kplotOffersAction(&inst, o) {
		t.Fatalf("plot action offered at instant speed: %v", inst.Options[:inst.N])
	}
	s.active = 0
	spells, ring := s.g.hist[0][histSpells], s.g.hist[0][histCastN]
	sc.do(t, AkPlot, TCode(o), scAny)
	if s.objs[o].zone != kop.ZoneExile {
		t.Fatalf("zone %d after plot, want exile", s.objs[o].zone)
	}
	if s.sp != 0 {
		t.Fatalf("plot pushed %d stack entries, want 0", s.sp)
	}
	if s.g.hist[0][histSpells] != spells || s.g.hist[0][histCastN] != ring {
		t.Fatalf("plot bumped the cast history: spells %d->%d ring %d->%d", spells, s.g.hist[0][histSpells], ring, s.g.hist[0][histCastN])
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v not spent", s.pl[0].pool)
	}
	r, ok := kplotRecord(s, o)
	if !ok || r.turn != mayWord(s.turn) || r.ctrl != 0 || r.ver != s.objs[o].ver {
		t.Fatalf("plot record %+v (ok %v), want turn %d ctrl 0 ver %d", r, ok, s.turn, s.objs[o].ver)
	}
	d = s.Step()
	if kplotOffers(&d, o) {
		t.Fatalf("plotted cast offered in the plot turn: %v", d.Options[:d.N])
	}
}

// TestPlotFreeCast: on a later turn the plotted card is cast from exile
// without paying its mana cost at sorcery speed, and the cast's push
// makes the record stale, so it is not offered again (CR 702.170b).
func TestPlotFreeCast(t *testing.T) {
	sc, id := kplotScen(t)
	o := sc.hand(id, 0)
	sc.do(t, AkPlot, TCode(o), scAny)
	sc.s.Step() // seat 0's priority after the action
	sc.nextTurn()
	d := sc.s.Step()
	if !kplotOffers(&d, o) {
		t.Fatalf("plotted cast not offered on the later turn: %v", d.Options[:d.N])
	}
	if sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool not empty before the free cast")
	}
	// on the later turn the free cast still waits for sorcery timing: with
	// the opponent active (main phase, empty stack) it is not offered.
	sc.s.active = 1
	sc.s.step = kop.StepMain1
	var inst Decision
	sc.s.priorityOptions(sc.s.k(), &inst)
	if kplotOffers(&inst, o) {
		t.Fatalf("plotted cast offered at instant speed: %v", inst.Options[:inst.N])
	}
	sc.s.active = 0
	sc.castMode(t, o, castPlotted)
	for i := 0; i < 8 && sc.s.objs[o].zone != kop.ZoneStack; i++ {
		sc.s.Step()
	}
	if sc.s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("zone %d after the plotted cast, want stack", sc.s.objs[o].zone)
	}
	if sc.s.plotRecord(0, o) {
		t.Fatalf("the plot record survived the cast's push")
	}
}

// TestPlotLeftExile: a plotted card moved out of exile and back loses the
// designation (CR 701.34c), so no plotted cast is offered on a later turn.
func TestPlotLeftExile(t *testing.T) {
	sc, id := kplotScen(t)
	o := sc.hand(id, 0)
	sc.do(t, AkPlot, TCode(o), scAny)
	sc.s.Step()
	from := sc.s.detach(o)
	sc.s.placeFrom(o, kop.ZoneGraveyard, false, from, false)
	from = sc.s.detach(o) // and back to exile: a new object (CR 400.7)
	sc.s.placeFrom(o, kop.ZoneExile, false, from, false)
	sc.nextTurn()
	d := sc.s.Step()
	if kplotOffers(&d, o) {
		t.Fatalf("plotted cast offered after the card left exile: %v", d.Options[:d.N])
	}
}

// kplotOffersAction reports whether d offers o's plot special action.
func kplotOffersAction(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkPlot && Obj(a.Source()) == o {
			return true
		}
	}
	return false
}
