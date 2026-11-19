package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// setSnowPool floats w units of colour c, n of them snow (CR 106.3), for seat 0.
func setSnowPool(sc *scen, c int, w, n uint8) {
	sc.s.pl[0].pool[c] = w
	sc.s.snowSetAt(0, c, n)
	if n > 0 {
		sc.s.g.poolArt[0] |= poolArtSnow
	}
}

// TestSnowCostPays: a {S} cost (CR 107.4h) is paid from the snow units of the
// floating pool (gorge ResolveManaWith alt.Snow): snowPays reserves the
// coloured pips and the generic around them, a {S} activation is offered only
// while the pool holds enough snow, and payFromPool takes each pip from the
// first of W U B R G C with a snow unit.
func TestSnowCostPays(t *testing.T) {
	sc, c := snowScen(t)
	s := sc.s
	ask := sc.perm(c.ask, 0)

	t.Run("snowPays", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			pool    [2]uint8 // W units, of them snow
			cost    kop.ManaCost
			payable bool
		}{
			{"one snow unit pays {S}", [2]uint8{1, 1}, kop.ManaCost{Snow: 1}, true},
			{"a plain unit does not", [2]uint8{1, 0}, kop.ManaCost{Snow: 1}, false},
			{"two {S} need two snow units", [2]uint8{2, 1}, kop.ManaCost{Snow: 2}, false},
			{"{S} and a generic from a plain unit", [2]uint8{2, 1}, kop.ManaCost{Snow: 1, Generic: 1}, true},
			{"the {S} pip's unit cannot also pay the generic", [2]uint8{1, 1}, kop.ManaCost{Snow: 1, Generic: 1}, false},
			{"the coloured pip takes the plain unit first", [2]uint8{2, 1}, kop.ManaCost{Snow: 1, Pips: [kop.NumManaKinds]uint8{kop.W: 1}}, true},
			{"the coloured pip leaves no snow unit", [2]uint8{1, 1}, kop.ManaCost{Snow: 1, Pips: [kop.NumManaKinds]uint8{kop.W: 1}}, false},
		} {
			s.pl[0].pool = [kop.NumManaKinds]uint8{}
			s.snowClearAll()
			setSnowPool(sc, kop.W, tc.pool[0], tc.pool[1])
			if got := s.poolPays(0, &tc.cost); got != tc.payable {
				t.Errorf("%s: poolPays = %v, want %v", tc.name, got, tc.payable)
			}
		}
	})

	t.Run("offered_only_with_snow", func(t *testing.T) {
		s.pl[0].pool = [kop.NumManaKinds]uint8{}
		s.snowClearAll()
		if sc.grantCapTapXOffers(ask) != 0 {
			t.Fatal("the {S} activation is offered with an empty pool")
		}
		setSnowPool(sc, kop.W, 1, 0) // a plain unit
		if sc.grantCapTapXOffers(ask) != 0 {
			t.Fatal("the {S} activation is offered on a plain unit")
		}
		setSnowPool(sc, kop.W, 1, 1)
		if sc.grantCapTapXOffers(ask) == 0 {
			t.Fatal("the {S} activation is not offered on a snow unit")
		}
	})

	t.Run("payFromPool_order", func(t *testing.T) {
		for _, any := range []bool{false, true} {
			DebugSnowAnyUnit = any
			s.pl[0].pool = [kop.NumManaKinds]uint8{}
			s.snowClearAll()
			setSnowPool(sc, kop.W, 1, 1)
			s.pl[0].pool[kop.R] = 1 // a plain red unit beside the snow white one
			s.pl[0].pool[kop.C] = 1
			s.payFromPool(0, &kop.ManaCost{Snow: 1}, 0)
			DebugSnowAnyUnit = false
			if any {
				if s.pl[0].pool[kop.C] != 0 || s.pl[0].pool[kop.W] != 1 {
					t.Errorf("DebugSnowAnyUnit: pool %v, want the {C} unit taken", s.pl[0].pool)
				}
				continue
			}
			if s.pl[0].pool[kop.W] != 0 || s.pl[0].pool[kop.R] != 1 || s.pl[0].pool[kop.C] != 1 {
				t.Errorf("pool %v, want only the snow {W} unit taken", s.pl[0].pool)
			}
			if tally(s, 0, kop.W) != 0 {
				t.Errorf("snow tally %d after the {S} pip, want 0", tally(s, 0, kop.W))
			}
		}
	})

	t.Run("no_snow_unit_panics", func(t *testing.T) {
		s.pl[0].pool = [kop.NumManaKinds]uint8{}
		s.snowClearAll()
		setSnowPool(sc, kop.W, 1, 0)
		defer func() {
			if recover() == nil {
				t.Fatal("paying {S} from a plain unit did not panic")
			}
		}()
		s.payFromPool(0, &kop.ManaCost{Snow: 1}, 0)
	})

	t.Run("activation_spends_the_snow_unit", func(t *testing.T) {
		s.pl[0].pool = [kop.NumManaKinds]uint8{}
		s.snowClearAll()
		setSnowPool(sc, kop.W, 2, 1)
		if !sc.activateAb(ask, 0) {
			t.Fatal("the {S} activation is not offered")
		}
		s.Step() // the build finishes: the cost is paid
		if got := s.pl[0].pool[kop.W]; got != 1 {
			t.Fatalf("pool W %d after the activation, want 1", got)
		}
		if got := tally(s, 0, kop.W); got != 0 {
			t.Fatalf("snow tally %d after the activation, want 0: the snow unit was spent", got)
		}
	})

	// Coldsteel Heart's shape (a non-snow typed artifact made Snow only after prep)
	// (effects.ManaProducerTag), so it never pays {S}; DebugSnowTypedIsSnow
	// (the mutation switch) tags it snow.
	t.Run("typed_source_pays_no_snow", func(t *testing.T) {
		sc.k.t.Cards[c.typed].Snow = true // snowScen's relic is a typed artifact; make it a Snow one (CardDef.Snow)
		for _, mutate := range []bool{false, true} {
			DebugSnowTypedIsSnow = mutate
			s.pl[0].pool = [kop.NumManaKinds]uint8{}
			s.snowClearAll()
			s.g.poolArt[0] = 0
			relic := sc.perm(c.typed, 0)
			if !s.tapOneForMana(sc.k, 0, sc.slot(relic)) {
				t.Fatal("the snow artifact does not tap for mana")
			}
			DebugSnowTypedIsSnow = false
			if got := s.poolPays(0, &kop.ManaCost{Snow: 1}); got != mutate {
				t.Errorf("DebugSnowTypedIsSnow=%v: poolPays({S}) = %v, want %v", mutate, got, mutate)
			}
		}
	})
}
