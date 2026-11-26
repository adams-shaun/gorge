package kgen

import (
	"strings"
	"testing"
)

// countZonesCarriers lower through three Count$/filter shapes gorge
// already reads: the $CardCounters.ALL aggregate of a battlefield count
// (every counter kind summed, gorge Object.Counter("ALL"); kernel ExValidAgg
// AggSumCounters with Ctr CtrAll), a multi-zone Count$Valid<Z1>,<Z2> head
// with no aggregate (gorge multiCountZones: one count per named zone over
// the same spec), and an Attacks trigger's CheckSVar$ over DefenderCtrl
// (gorge checkSVarGateHolds evaluates it with the trigger's DefendingPlayer).
var countZonesCarriers = []string{
	"Backstreet Bruiser", "Maester Seymour", "Dimension X Pizzasaur",
	"Machinist's Dismissal", "Frenzied Geistblaster", "Candlekeep Inspiration",
	"Aerial Surveyor",
}

// countZonesRefused stay refused by a blocker outside this slice:
// Lumbering Megasloth's PlayerCountPlayers$Counters.ALL, Lux Artillery's
// Sunburst, and Gaea's Liege's DefenderCtrl in a static (gorge binds no
// defending player there).
var countZonesRefused = map[string]string{
	"Lumbering Megasloth": "PlayerCountPlayers$Counters.ALL",
	"Lux Artillery":       "Sunburst",
	"Gaea's Liege":        "DefenderCtrl",
}

func TestFilterHeadsCountZones(t *testing.T) {
	names := append([]string{}, countZonesCarriers...)
	for n := range countZonesRefused {
		names = append(names, n)
	}
	got := buildNamed(t, names)
	for _, n := range countZonesCarriers {
		rs, ok := got[n]
		if !ok {
			t.Errorf("%s: not lowered (corpus missing?)", n)
			continue
		}
		if len(rs) != 0 {
			t.Errorf("%s: refused: %q", n, rs)
		}
	}
	for n, frag := range countZonesRefused {
		if j := strings.Join(got[n], "; "); !strings.Contains(j, frag) {
			t.Errorf("%s: refusals %q, want one naming %q", n, j, frag)
		}
	}
}

// TestMultiZoneCountShape: the multi-zone head sums one count per named zone
// in order, the library walk as vCountHand N 1; a repeated zone word is no
// multi-zone head (gorge multiCountZones returns nil) and is left alone.
func TestMultiZoneCountShape(t *testing.T) {
	lc := &lowerCtx{}
	if _, ok, _ := lc.multiZoneCount("ValidHand,Hand Card.YouOwn"); ok {
		t.Error("a repeated zone lowered as a multi-zone head")
	}
	if _, ok, _ := lc.multiZoneCount("ValidHand,Graveyard Card.YouOwn$CardPower"); ok {
		t.Error("an aggregate lowered as a plain multi-zone count")
	}
}
