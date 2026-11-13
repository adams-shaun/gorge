package oracle

import (
	"strings"
	"testing"
)

// An announced X beside depletion lands in lockstep with gorge (CR 601.2b-g).
// Peat Bog and Sandstone Needle cost {T} and a depletion counter, so gorge's
// window census (windowManaUnits takes a free cost only; castWindowProbeUnits
// withholds SubCounter) never counts their mana in the X announcement: the
// ask for Skeletal Scrying's or Lunar Frenzy's X is bounded by the other
// sources. The kernel's availXWindow follows it (dirac/lockstepconfig.go).

var mbcDepletionXWatch = map[string]bool{"Skeletal Scrying": true, "Lunar Frenzy": true}

var mbcDepletionXDecks = []fixtureDecks{{
	{
		{Name: "Skeletal Scrying", Count: 4}, {Name: "Lunar Frenzy", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Terror", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Peat Bog", Count: 5}, {Name: "Sandstone Needle", Count: 3}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Skeletal Scrying", Count: 3}, {Name: "Lunar Frenzy", Count: 4}, {Name: "Terror", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Peat Bog", Count: 4}, {Name: "Sandstone Needle", Count: 4}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
	},
}}

// TestMBCXWindowDepletion plays the fixture until the first divergence. A depletion
// land that gorge's autopay taps where the kernel taps a basic (a projection
// divergence after the cast's target, Borderland Ranger's too on the dr-mbc
// pools) is a separate class this test does not cover, so it asks only that the
// first divergence is no X ask: without the availXWindow rule seeds 1 and 5
// offer X above gorge's.
func TestMBCXWindowDepletion(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "mbcdepletionx", mbcDepletionXDecks, mbcDepletionXWatch, 150, false, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, first %q", run.games, run.divs, run.stalls, run.casts, run.first)
	if run.casts["Skeletal Scrying"]+run.casts["Lunar Frenzy"] < 5 {
		t.Errorf("only %v casts of the X spells: the X ask was barely reached", run.casts)
	}
	if strings.HasPrefix(run.first, "options") {
		t.Errorf("the first divergence is an options one: %s", run.first)
	}
}
