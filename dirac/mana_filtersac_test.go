package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A filter whose activation cost sacrifices its source (Omni-Cheese Pizza:
// "{1}, {T}, Sacrifice this artifact: Add one mana of any color"). The
// sacrifice is part of the activation cost (CR 602.2b, 605.1a): filtDo takes it
// after the mana is added, on the search's copies, in filtPlay and in the live
// payment window alike (sacFilterSource).

// filtSacScen: p0 has Omni-Cheese Pizza, two Plains and Moonstone Eulogist
// ("Whenever you sacrifice an artifact, put a +1/+1 counter on it and gain 1
// life"), Doom Blade ({1}{B}) in hand; p1 has Grizzly Bears, its target. Only
// the Pizza makes {B}: a Plains funds its {1}, the other pays Doom Blade's {1}.
// quark adds a "you can't sacrifice artifacts you control" static (a test
// card; the printed carriers are creature-, token- or cause-bound).
func filtSacScen(t *testing.T, quark bool) (sc *scen, pizza, eulogist, blade Obj, plains [2]Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Omni-Cheese Pizza", Count: 1}, {Name: "Moonstone Eulogist", Count: 1}, {Name: "Doom Blade", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Set(t, "filtersac")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var qid CardID
	if quark {
		tb := *tbl
		tb.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
		qid = CardID(len(tb.Cards))
		tb.Cards = append(append([]kop.CardDef(nil), tbl.Cards...), kop.CardDef{Name: "Test Quark", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic,
				Static: kop.Static{Kind: kop.StatCantSacrifice, Filter: kop.FilterID(len(tb.Filters) - 1)}}}})
		tbl = &tb
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("the table has no generated extension")
	}
	for i := range plains {
		plains[i] = sc.perm(id["Plains"], 0)
	}
	pizza = sc.perm(id["Omni-Cheese Pizza"], 0)
	eulogist = sc.perm(id["Moonstone Eulogist"], 0)
	if quark {
		sc.perm(qid, 0)
	}
	sc.perm(id["Grizzly Bears"], 1)
	blade = sc.hand(id["Doom Blade"], 0)
	return sc, pizza, eulogist, blade, plains
}

// TestFilterSacrificesItsSource: Doom Blade's {B} is made only by the Pizza's
// filter. The cast is offered (the kernel's prefix search sacrifices the
// Pizza on its State copies, not on the real one); played, the Pizza is in
// its owner's graveyard, both Plains are tapped, the pool is empty and the
// Eulogist's Sacrificed trigger fired (a counter and a life).
func TestFilterSacrificesItsSource(t *testing.T) {
	sc, pizza, eulogist, blade, plains := filtSacScen(t, false)
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(pizza)) {
		t.Fatal("the Pizza is no census gap")
	}
	if !sc.s.subs[kop.TrigSacrificed].any() {
		t.Fatal("no Sacrificed listener: the watcher is not in play")
	}
	a, ok := bladeCast(sc, blade)
	if !ok {
		t.Fatal("Doom Blade's {1}{B} not offered over the Pizza's filter")
	}
	if !sc.onBF(pizza) {
		t.Fatal("the offer sacrificed the Pizza: the search must act on copies")
	}
	life := sc.s.pl[0].life
	sc.s.Apply(a)
	for i := 0; i < 4; i++ { // the target (the Bears), then the payment
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[blade].zone != kop.ZoneStack {
		t.Fatalf("Doom Blade in zone %d, want the stack", sc.s.objs[blade].zone)
	}
	if z := sc.s.objs[pizza].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the Pizza in zone %d, want the graveyard (sacrificed as part of the cost)", z)
	}
	for _, o := range plains {
		if !sc.tapped(o) {
			t.Error("a Plains is untapped after the cast")
		}
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", pool)
	}
	sc.resolveAll(t)
	if got := sc.s.pl[0].life; got != life+1 {
		t.Errorf("life %d, want %d: the Eulogist's Sacrificed trigger did not resolve once", got, life+1)
	}
	if n := sc.pceCount(eulogist, kop.CtrP1P1); n != 1 {
		t.Errorf("the Eulogist has %d +1/+1 counters, want 1", n)
	}
}

// TestFilterSacBlockedIsNoCost: the same board with a "can't sacrifice
// artifacts you control" static offers no cast over the Pizza (a cost that
// cannot be paid is never activated, CR 602.2b, 601.2h); without the static
// it is offered (the precondition).
func TestFilterSacBlockedIsNoCost(t *testing.T) {
	sc, _, _, blade, _ := filtSacScen(t, false)
	if _, ok := bladeCast(sc, blade); !ok {
		t.Fatal("precondition: Doom Blade is not offered without the static")
	}
	sc, pizza, _, blade, _ := filtSacScen(t, true)
	if !sc.s.sacBlocked(sc.k, sc.slot(pizza)) {
		t.Fatal("precondition: the static does not block sacrificing the Pizza")
	}
	if _, ok := bladeCast(sc, blade); ok {
		t.Fatal("Doom Blade offered over a Pizza that can't be sacrificed")
	}
	// the static alone is the cause (actForbidden: manaSacBlocked reads the
	// filter, the Pizza's first mana ability)
	DebugCantSacOff = true
	defer func() { DebugCantSacOff = false }()
	if _, ok := bladeCast(sc, blade); !ok {
		t.Fatal("Doom Blade not offered with sacBlocked switched off: the block is not the static's")
	}
}

// TestWindowFilterSacrifices: the live payment window activates the Pizza
// (the floating {B} pays its {1}, the filter makes {G}) and the Pizza leaves
// the battlefield.
func TestWindowFilterSacrifices(t *testing.T) {
	sc, pizza, _, _, _ := filtSacScen(t, false)
	s := sc.s
	s.pl[0].pool[kop.B] = 1
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
	if !sc.onBF(pizza) {
		t.Fatal("precondition: the Pizza is not on the battlefield")
	}
	ok, _ := s.windowTapRider(sc.k, 0, &cost, true, false)
	if !ok {
		t.Fatal("the window did not pay through the filter")
	}
	if z := s.objs[pizza].zone; z != kop.ZoneGraveyard {
		t.Errorf("the Pizza in zone %d after the window activation, want the graveyard", z)
	}
	if s.pl[0].pool[kop.B] != 0 || s.pl[0].pool[kop.G] != 1 {
		t.Errorf("pool B=%d G=%d, want the fee off {B} and {G} made", s.pl[0].pool[kop.B], s.pl[0].pool[kop.G])
	}
}
