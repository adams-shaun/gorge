package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The manasecond fixture's per-card kernel facts, pinned without a game:
//
//   - Haunted Screen's life ability is a last-resort census alternative
//     (lifePainLandAb), not a census gap. With gfGapOther set, gapOthers is
//     non-empty, filtOnly is false, and scriptOffers/availScript withhold a
//     filter-paid play that gorge's prefix search offers (gen.go,
//     script_gen.go). Baxter Building is the filter on the same side.
//   - A payment window's colour ask carries no ManaPayment, so the
//     oracle's window answer (payexec.WindowAnswer, TestPayWindowTapsLowestColour)
//     takes an owed colour if there is one and otherwise the lowest listed
//     colour: Haunted Screen's {W}, never its painful {G} unowed. The same
//     answer taps it for W in a failed payment (tapAllForMana,
//     tapOneForMana), so no life is paid.
//   - Baxter Building's combination activation taps a land, so a live land
//     tap-extra must stall it (StallTapExtra), not silently drop the extra.

func manasecondScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "manasecond")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, da[0], Config{}), id
}

func TestHauntedScreenIsNoManaCensusGap(t *testing.T) {
	sc, id := manasecondScen(t, []cardset.Entry{
		{Name: "Haunted Screen", Count: 1}, {Name: "Baxter Building", Count: 1},
	})
	hs, bb := id["Haunted Screen"], id["Baxter Building"]
	if !sc.k.painMana {
		t.Fatalf("Haunted Screen's life ability is not prepped as a pain alternative")
	}
	if sc.k.gen.flags[hs]&gfGapOther != 0 {
		t.Fatalf("Haunted Screen is a census gap (gapOther %d) beside Baxter Building's filter: filtOnly is false and the filter's scripted plays are withheld", sc.k.gen.gapOther(hs))
	}
	if sc.k.gen.filt(bb) < 0 {
		t.Fatalf("Baxter Building is not prepped as a filter (filt %d)", sc.k.gen.filt(bb))
	}
}

func TestHauntedScreenWindowAnswers(t *testing.T) {
	deck := []cardset.Entry{{Name: "Haunted Screen", Count: 1}}

	t.Run("noOwed", func(t *testing.T) {
		// A ward/unless window's ask carries no payment: the {G} goes unpaid
		// with W floating, and no life is paid.
		sc, id := manasecondScen(t, deck)
		hs := sc.perm(id["Haunted Screen"], 0)
		c := cost(0, kop.G)
		_, short := sc.s.payWindow(sc.k, 0, &c)
		var want [kop.NumManaKinds]uint8
		want[kop.W] = 1
		if !sc.tapped(hs) || !short || sc.s.pl[0].pool != want || sc.s.pl[0].life != 20 {
			t.Fatalf("Haunted Screen tapped %v, short %v, pool %v, life %d: want W made, life 20, the {G} unpaid",
				sc.tapped(hs), short, sc.s.pl[0].pool, sc.s.pl[0].life)
		}
	})

	t.Run("castNoOwedColour", func(t *testing.T) {
		// A cast window with only generic owed lists every colour and takes
		// the lowest, W, not the painful G (no life).
		sc, id := manasecondScen(t, deck)
		hs := sc.perm(id["Haunted Screen"], 0)
		c := cost(1)
		ok, _ := sc.s.windowTap(sc.k, 0, &c, true)
		var want [kop.NumManaKinds]uint8
		want[kop.W] = 1
		if !ok || !sc.tapped(hs) || sc.s.pl[0].pool != want || sc.s.pl[0].life != 20 {
			t.Fatalf("windowTap ok %v, tapped %v, pool %v, life %d: want W made, life 20",
				ok, sc.tapped(hs), sc.s.pl[0].pool, sc.s.pl[0].life)
		}
	})

	t.Run("castOwedPain", func(t *testing.T) {
		// An owed {G} is the painful ability's colour: W is not it, so the
		// window pays 1 life for G.
		sc, id := manasecondScen(t, deck)
		hs := sc.perm(id["Haunted Screen"], 0)
		c := cost(0, kop.G)
		ok, _ := sc.s.windowTap(sc.k, 0, &c, true)
		var want [kop.NumManaKinds]uint8
		want[kop.G] = 1
		if !ok || !sc.tapped(hs) || sc.s.pl[0].pool != want || sc.s.pl[0].life != 19 {
			t.Fatalf("windowTap ok %v, tapped %v, pool %v, life %d: want G made, life 19",
				ok, sc.tapped(hs), sc.s.pl[0].pool, sc.s.pl[0].life)
		}
	})
}

func TestHauntedScreenManualTapColour(t *testing.T) {
	// A failed payment taps every source for its lowest colour (tapAllForMana
	// -> tapOneForMana): Haunted Screen's W is the normal ability's, so the
	// tap pays no life.
	sc, id := manasecondScen(t, []cardset.Entry{{Name: "Haunted Screen", Count: 1}})
	hs := sc.perm(id["Haunted Screen"], 0)
	sc.s.tapAllForMana(sc.k, 0)
	var want [kop.NumManaKinds]uint8
	want[kop.W] = 1
	if !sc.tapped(hs) || sc.s.pl[0].pool != want || sc.s.pl[0].life != 20 {
		t.Fatalf("tapAllForMana tapped %v, pool %v, life %d: want W made, life 20",
			sc.tapped(hs), sc.s.pl[0].pool, sc.s.pl[0].life)
	}
}

func TestBaxterCombinationFilterStallsOnTapExtra(t *testing.T) {
	sc, id := manasecondScen(t, []cardset.Entry{
		{Name: "Baxter Building", Count: 1}, {Name: "Groundchuck & Dirtbag", Count: 1},
	})
	bb := sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Groundchuck & Dirtbag"], 0)
	sc.s.pl[0].pool[kop.C] = 4 // the {4} fee, as the plan would float it
	var alloc [kop.NumManaKinds]uint8
	alloc[kop.W] = 4
	sc.s.filtDo(sc.k, 0, filtStep{sl: sc.slot(bb), ans: 1, gap: true, alloc: alloc})
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("combination activation stalled %d, want StallTapExtra (%d): a land tap-extra must not be dropped", sc.s.g.stalled, StallTapExtra)
	}
}
