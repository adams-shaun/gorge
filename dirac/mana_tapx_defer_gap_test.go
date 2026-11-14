package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A real tap-another source (Springleaf Drum) beside a depletion-rider land
// (Peat Bog, a Taps-deferred gap) and a counter-cost source (a sick Wall of
// Roots): gorge's ONE prefix search (scriptSearch.visit) visits every census
// gap in battlefield order, so the tap-another search (tapXScript,
// mana_tapanother.go), which owns the board's prefix, must branch on the
// Taps-deferred gap too. Relic Crush {4}{G} is paid only by the Island, the
// Drum (tapping the Bears), the Bog's {B}{B} and the Wall's {G}.
func tapXDeferScen(t *testing.T) (*scen, map[string]Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Springleaf Drum", Count: 1}, {Name: "Peat Bog", Count: 1}, {Name: "Wall of Roots", Count: 1},
		{Name: "Relic Crush", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Island", Count: 15}}
	tbl, _, _, err := cardsettest.Set(t, "tapxdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeLowering: true})
	objs := map[string]Obj{}
	for _, n := range []string{"Island", "Springleaf Drum", "Grizzly Bears", "Peat Bog", "Wall of Roots"} {
		objs[n] = sc.perm(id[n], 0)
	}
	objs["Relic Crush"] = sc.hand(id["Relic Crush"], 0)
	bog, wall := sc.slot(objs["Peat Bog"]), sc.slot(objs["Wall of Roots"])
	a, ok := sc.s.riderAbilityOf(sc.k, bog)
	if !ok {
		t.Fatal("precondition: Peat Bog is not the depletion-rider shape")
	}
	if n := sc.s.perms[bog].ctrN(sc.s, int(a.ManaRiderCtr)); n != 2 {
		t.Fatalf("precondition: Peat Bog holds %d depletion counters after its ETB, want 2", n)
	}
	sc.s.perms[bog].flags &^= pfTapped // its ETB tapped it; it has untapped since
	sc.s.perms[wall].flags |= pfSick   // it entered this turn: only the counter route pays it
	return sc, objs
}

// tapXDeferPreconditions asserts the board is the coexisting one: each of the
// three gap kinds is live, the plain sources pay one mana, and the board is
// the tap-another search's (scriptOffers' tx arm, script_gen.go).
func tapXDeferPreconditions(t *testing.T, sc *scen, objs map[string]Obj) {
	t.Helper()
	s, k := sc.s, sc.k
	drum, bog, wall := sc.slot(objs["Springleaf Drum"]), sc.slot(objs["Peat Bog"]), sc.slot(objs["Wall of Roots"])
	if !s.scriptTapXModelled(k, 0).has(drum) {
		t.Fatal("precondition: Springleaf Drum is not a modelled tap-another gap")
	}
	if !s.scriptTapsDefer(k, 0).has(bog) {
		t.Fatal("precondition: Peat Bog is not in the Taps-deferred gap mask")
	}
	if !s.scriptCtr(k, 0).has(wall) {
		t.Fatal("precondition: the sick Wall of Roots is not in the counter-cost gap mask")
	}
	if !s.tapXBoard(k, 0) {
		t.Fatal("precondition: this is not a tap-another board")
	}
	var a avail
	s.availFor(k, 0, &a)
	if a.total != 1 {
		t.Fatalf("precondition: plain availability %d, want the Island's 1", a.total)
	}
}

func relicCrushCast(d *Decision, crush Obj) (Action, bool) {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(crush) {
			return d.Options[i], true
		}
	}
	return 0, false
}

func TestTapXBesideTapsDeferGapOffersCast(t *testing.T) {
	sc, objs := tapXDeferScen(t)
	tapXDeferPreconditions(t, sc, objs)
	d := sc.s.Step()
	if _, ok := relicCrushCast(&d, objs["Relic Crush"]); !ok {
		t.Fatalf("the cast of Relic Crush is not offered beside the pass; opts %v", d.Options[:d.N])
	}
}

// TestTapXBesideTapsDeferGapReplaysCast plays the offered cast: the replay
// route (scriptPrefix's tapXPrefix) runs the same search with apply, tapping
// the Drum, the Bog and the Wall before the payment, so the spell reaches the
// stack with every gap spent.
func TestTapXBesideTapsDeferGapReplaysCast(t *testing.T) {
	sc, objs := tapXDeferScen(t)
	tapXDeferPreconditions(t, sc, objs)
	s := sc.s
	d := s.Step()
	cast, ok := relicCrushCast(&d, objs["Relic Crush"])
	if !ok {
		t.Fatalf("the cast of Relic Crush is not offered beside the pass; opts %v", d.Options[:d.N])
	}
	s.Apply(cast)
	for guard := 0; guard < 16 && s.objs[objs["Relic Crush"]].zone == kop.ZoneHand; guard++ {
		d = s.Step()
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0]) // the cast's own asks (its targets), first answer
	}
	if s.over {
		t.Fatal("the game ended during the cast")
	}
	if z := s.objs[objs["Relic Crush"]].zone; z != kop.ZoneStack {
		t.Fatalf("Relic Crush is not on the stack after its cast (zone %d)", z)
	}
	if drum := sc.slot(objs["Springleaf Drum"]); s.perms[drum].flags&pfTapped == 0 {
		t.Fatal("Springleaf Drum was not tapped by the replayed prefix")
	}
	bog := sc.slot(objs["Peat Bog"])
	if s.occ.has(bog) && s.perms[bog].flags&pfTapped == 0 {
		t.Fatal("Peat Bog was not tapped by the replayed prefix")
	}
	if wall := sc.slot(objs["Wall of Roots"]); s.occ.has(wall) && s.scriptCtr(sc.k, 0).has(wall) {
		t.Fatal("the Wall of Roots' counter ability was not used by the replayed prefix")
	}
}
