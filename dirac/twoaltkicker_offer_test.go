package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// twoAltScene builds a Thornscape Battlemage scene (the two-alternative
// Kicker carrier the mbc pool plays, 4 copies in a deck): the battlemage in
// p0's hand and forests Forests, a Mountain and a Plains on p0's battlefield
// (its mana: base {2}{G}, kicker part one {R}, part two {W}).
func twoAltScene(t *testing.T, forests, mountain, plains int) (*scen, Obj, CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Thornscape Battlemage", Count: 1},
		{Name: "Forest", Count: forests},
		{Name: "Mountain", Count: mountain},
		{Name: "Plains", Count: plains},
	}
	set := cardsettest.Set(t, "mbc")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := &tbl.Cards[id["Thornscape Battlemage"]]
	if !hasKicker2(cd) {
		t.Fatal("Thornscape Battlemage lowered without CardDef.Kicker2 (kgen regression)")
	}
	// the split: part one {R} rides Kicker, part two {W} Kicker2
	if cd.Kicker.Pips != [6]uint8{3: 1} || cd.Kicker2.Pips != [6]uint8{0: 1} || cd.HasMultikicker {
		t.Fatalf("Thornscape split wrong: Kicker=%+v Kicker2=%+v multi=%v", cd.Kicker, cd.Kicker2, cd.HasMultikicker)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for i := 0; i < forests; i++ {
		sc.perm(id["Forest"], 0)
	}
	if mountain > 0 {
		sc.perm(id["Mountain"], 0)
	}
	if plains > 0 {
		sc.perm(id["Plains"], 0)
	}
	return sc, sc.hand(id["Thornscape Battlemage"], 0), id["Thornscape Battlemage"]
}

// twoAltCasts reads the priority decision's AkCast actions on o: plain (mode
// 0) and the per-alternative kicked casts (mode 1, the selection markers).
func twoAltCasts(t *testing.T, sc *scen, o Obj) (plain, alt1, alt2, both bool) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkPriority || d.Player != 0 {
		t.Fatalf("decision kind %d player %d, want p0's DkPriority", d.Kind, d.Player)
	}
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkCast || a.Source() != TCode(uint8(o)) {
			continue
		}
		switch {
		case a.Mode() == 0:
			plain = true
		case a.Mode()&1 != 0: // seat-ok: a cast-mode bit
			switch a.Target() {
			case TCode(castKickAlt1Tgt):
				alt1 = true
			case TCode(castKickAlt2Tgt):
				alt2 = true
			case TCode(castKickBothTgt):
				both = true
			}
		}
	}
	return plain, alt1, alt2, both
}

// twoAltCastOf is the priority decision's cast of o with target byte tgt: a
// per-alternative kicked cast (mode 1, the marker) for tgt 4/6/7, the plain
// cast (mode 0, no marker) for tgt 0. 0 when the cast is not offered.
func twoAltCastOf(d Decision, o Obj, tgt uint8) Action {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkCast || a.Source() != TCode(uint8(o)) {
			continue
		}
		if tgt == 0 {
			if a.Mode() == 0 && a.Target() == 0 {
				return a
			}
			continue
		}
		if a.Mode()&1 != 0 && a.Target() == TCode(tgt) { // seat-ok: a cast-mode bit
			return a
		}
	}
	return 0
}

// twoAltResolve applies action a and passes priority until the stack is
// empty; asked reports whether a target decision was posed (a kicked ETB
// trigger's ask). A target ask is answered with its first option that is not
// the cast's own permanent (a self-targeted 2 damage would kill it).
func twoAltResolve(t *testing.T, sc *scen, a Action, avoid Obj) (asked bool) {
	t.Helper()
	sc.s.Apply(a)
	for step := 0; step < 120; step++ {
		d := sc.s.Step()
		if d.Kind == DkTarget {
			asked = true
		}
		if sc.s.sp == 0 && d.Kind == DkPriority {
			return asked
		}
		if d.N == 0 {
			continue
		}
		pick := d.Options[0]
		if d.Kind == DkTarget {
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Target() != TCode(uint8(avoid)) {
					pick = d.Options[i]
					break
				}
			}
		}
		sc.s.Apply(pick)
	}
	t.Fatal("the cast did not resolve")
	return asked
}

// TestTwoAltKickerOffer: a two-alternative Kicker card in hand offers the
// four casts gorge offers (unkicked, kicked 1, kicked 2, both). The
// per-alternative casts ride gorge's floating-pool walk gate (legal_walk_hand.go's
// and/or branch prices them with the hyp-nil offerCastable; its potential
// planner has no kicked1/kicked2/kickedboth row), so they are offered exactly
// when the floating pool pays the selection's total -- the scene floats the
// mana the way a player does by activating mana abilities at priority.
func TestTwoAltKickerOffer(t *testing.T) {
	four := func(sc *scen, o Obj) (plain, alt1, alt2, both bool) {
		return twoAltCasts(t, sc, o)
	}
	// 4 Forests + Mountain + Plains floated: {2}{G}{R}{W} is payable, all four.
	sc, o, _ := twoAltScene(t, 4, 1, 1)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.W: 1, kop.R: 1, kop.G: 4} // W R G: the two pips and the {2} generic
	plain, alt1, alt2, both := four(sc, o)
	if !plain || !alt1 || !alt2 || !both {
		t.Fatalf("full pool: plain=%v alt1=%v alt2=%v both=%v, want all four", plain, alt1, alt2, both)
	}
	// the same board with no red floated: part one {R} is unpayable; plain
	// and part two stay.
	sc, o, _ = twoAltScene(t, 4, 0, 1)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.W: 1, kop.G: 4}
	plain, alt1, alt2, both = four(sc, o)
	if !plain || alt1 || !alt2 || both {
		t.Fatalf("no red: plain=%v alt1=%v alt2=%v both=%v, want plain/alt2 only", plain, alt1, alt2, both)
	}
	// no mana at all: nothing is offered (gorge's pool gate refuses; the
	// potential planner has no per-alternative row to lower).
	sc, o, _ = twoAltScene(t, 0, 0, 0)
	plain, alt1, alt2, both = four(sc, o)
	if plain || alt1 || alt2 || both {
		t.Fatalf("no mana: plain=%v alt1=%v alt2=%v both=%v, want none", plain, alt1, alt2, both)
	}
	// no red or white in the pool: only the plain cast is payable.
	sc, o, _ = twoAltScene(t, 3, 0, 0)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.G: 5}
	plain, alt1, alt2, both = four(sc, o)
	if !plain || alt1 || alt2 || both {
		t.Fatalf("green only: plain=%v alt1=%v alt2=%v both=%v, want plain only", plain, alt1, alt2, both)
	}
	// three units: {2}{G}{R} (4) and {2}{G}{W} (4) and the both total (5) are
	// unpayable; the plain cast stays.
	sc, o, _ = twoAltScene(t, 2, 1, 1)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.R: 1, kop.G: 2}
	plain, alt1, alt2, both = four(sc, o)
	if !plain || alt1 || alt2 || both {
		t.Fatalf("3 units: plain=%v alt1=%v alt2=%v both=%v, want plain only", plain, alt1, alt2, both)
	}
}
