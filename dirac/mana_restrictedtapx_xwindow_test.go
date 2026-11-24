package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrTapXXScen is seat 0 with Grand Architect, Prodigal Sorcerer (a blue
// creature its cost can tap) and two Islands on the battlefield, Walking
// Ballista and a non-artifact {X} sorcery in hand: an announced-X cast pays
// through gorge's manual CR 601.2g window (xWindowPay), whose answer is the
// first listed source.
func restrTapXXScen(t *testing.T) (sc *scen, ga, sorc Obj, isles [2]Obj, ballista, sorcery Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Grand Architect", Count: 1}, {Name: "Prodigal Sorcerer", Count: 1},
		{Name: "Walking Ballista", Count: 1}, {Name: "Island", Count: 5}}
	tbl, da, _, err := cardsettest.Set(t, "restrictedmana").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	xs := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test X Sorcery", Types: kop.TSorcery, Cost: kop.ManaCost{X: 1}})
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	sc = newScenTableLib(t, tbl, id["Island"], cfg)
	ga = sc.perm(id["Grand Architect"], 0)
	sorc = sc.perm(id["Prodigal Sorcerer"], 0)
	isles = [2]Obj{sc.perm(id["Island"], 0), sc.perm(id["Island"], 0)}
	ballista = sc.hand(id["Walking Ballista"], 0)
	sorcery = sc.hand(xs, 0)
	if !sc.s.scriptTapX(sc.k, 0).has(sc.slot(ga)) {
		t.Fatal("precondition: Grand Architect is no tap-another source on the board")
	}
	if !kop.RestrictedTapXOneShape(sc.k.tapXAbOf(sc.s.cardAt(sc.slot(ga)))) {
		t.Fatal("precondition: Grand Architect's mana ability is not the restricted tap-another shape")
	}
	return
}

// castX casts o and announces X = x, then steps until the spell is on the
// stack or a decision other than the payment's arrives; it returns the last
// decision stepped.
func (sc *scen) castX(t *testing.T, o Obj, x uint8) Decision {
	t.Helper()
	sc.do(t, AkCast, TCode(o), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == x {
			sc.s.Apply(d.Options[i])
			return sc.s.Step()
		}
	}
	t.Fatalf("X = %d not offered: %v", x, d.Options[:d.N])
	return d
}

// TestRestrictedTapXXWindowPaysArtifact: the announced-X window of an artifact
// spell (Walking Ballista, X = 1) lists Grand Architect, whose {C}{C} its
// restriction admits (CR 106.6), first in battlefield order: gorge's window
// answer takes it and asks which blue creature to tap (the first, Grand
// Architect itself), and its mana pays the {2}, the Islands untapped. Before,
// the window left every spend-restricted tap-another source out and tapped the
// Islands.
func TestRestrictedTapXXWindowPaysArtifact(t *testing.T) {
	sc, ga, sorc, isles, ballista, _ := restrTapXXScen(t)
	d := sc.castX(t, ballista, 1)
	if d.Kind != DkChoose || d.Sub != ChWindowTap {
		t.Fatalf("decision kind %d sub %d (Islands tapped %v %v): want the window's pick of the blue creature Grand Architect's cost taps",
			d.Kind, d.Sub, sc.tapped(isles[0]), sc.tapped(isles[1]))
	}
	if d.N != 2 || d.Options[0].Source() != TCode(ga) || d.Options[1].Source() != TCode(sorc) {
		t.Fatalf("pick options %v: want Grand Architect then Prodigal Sorcerer", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	sc.s.Step()
	if sc.s.objs[ballista].zone != kop.ZoneStack {
		t.Fatalf("Walking Ballista is in zone %d, want the stack", sc.s.objs[ballista].zone)
	}
	if sc.tapped(isles[0]) || sc.tapped(isles[1]) {
		t.Errorf("Islands tapped %v %v: Grand Architect's {C}{C} pays the {2}", sc.tapped(isles[0]), sc.tapped(isles[1]))
	}
	if !sc.tapped(ga) || sc.tapped(sorc) {
		t.Errorf("Grand Architect tapped %v, Prodigal Sorcerer tapped %v: want the picked Grand Architect only", sc.tapped(ga), sc.tapped(sorc))
	}
}

// TestRestrictedTapXXWindowRefusesNonArtifact: a non-artifact {X} spell's window
// does not list Grand Architect (its restriction refuses the payment, CR 106.6):
// the Islands pay X = 2 and no creature is tapped.
func TestRestrictedTapXXWindowRefusesNonArtifact(t *testing.T) {
	sc, ga, sorc, isles, _, sorcery := restrTapXXScen(t)
	d := sc.castX(t, sorcery, 2)
	if d.Kind == DkChoose && d.Sub == ChWindowTap {
		t.Fatal("the non-artifact spell's window posed Grand Architect's pick")
	}
	if sc.s.objs[sorcery].zone != kop.ZoneStack {
		t.Fatalf("the sorcery is in zone %d, want the stack", sc.s.objs[sorcery].zone)
	}
	if !sc.tapped(isles[0]) || !sc.tapped(isles[1]) || sc.tapped(ga) || sc.tapped(sorc) {
		t.Errorf("tapped Islands %v %v, Grand Architect %v, Sorcerer %v: want the Islands only",
			sc.tapped(isles[0]), sc.tapped(isles[1]), sc.tapped(ga), sc.tapped(sorc))
	}
}
