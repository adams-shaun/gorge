package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Producer-tagged Count$CastTotalManaSpent (CR 601.2h; gorge Object
// .ManaTreasureSpent / ManaCaveSpent / ManaDesertSpent): the kernel records
// the named tag's units the cast's payment spent (kgen lower_snowspent.go,
// dirac/mana_spenttag.go) and each card reads that count (Bat Colony reads
// the Cave tally Hidden Courtyard feeds).
// Played decision by decision over the manaspenttag pool. Marut and Prossh
// (a sacrifice outlet) sit in the same deck: sacrificing Marut, Hired
// Hexblade, Jaded Sell-Sword or Bat Colony in response to its enters trigger
// makes gorge's resolution-time CheckSVar recheck read the departed permanent
// with last-known information (rules' triggerResolvingCheckHolds passes the
// trigger's LKI snapshot), and the kernel keeps the bound spend, so the two
// agree.

var manaSpentTagDecks = []fixtureDecks{{
	{
		{Name: "Strike It Rich", Count: 4}, {Name: "Devour Intellect", Count: 3},
		{Name: "Hired Hexblade", Count: 4}, {Name: "Jaded Sell-Sword", Count: 3},
		{Name: "Price of Loyalty", Count: 3}, {Name: "Spoils of the Hunt", Count: 2},
		{Name: "Marut", Count: 2},
		{Name: "Prossh, Skyraider of Kher", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 1},
		{Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Cataclysmic Prospecting", Count: 4}, {Name: "Desert", Count: 5},
		{Name: "Bat Colony", Count: 4}, {Name: "Hidden Courtyard", Count: 5},
		{Name: "Mountain", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Strike It Rich", Count: 2}, {Name: "Prossh, Skyraider of Kher", Count: 4},
	},
}}

var manaSpentTagWatch = map[string]bool{
	"Strike It Rich": true, "Devour Intellect": true, "Hired Hexblade": true,
	"Jaded Sell-Sword": true, "Price of Loyalty": true, "Spoils of the Hunt": true,
	"Marut": true, "Bat Colony": true, "Cataclysmic Prospecting": true, "Prossh, Skyraider of Kher": true,
}

func manaSpentTagRun(t *testing.T, report bool) fixtureRun {
	t.Helper()
	return runFixtureLockstep(t, "manaspenttag", manaSpentTagDecks, manaSpentTagWatch, 200, report, nil)
}

// TestManaSpentTagLockstep: the kernel and gorge agree over the Treasure,
// Desert and plain-total carriers, both autopay surfaces alternating by seed.
func TestManaSpentTagLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := manaSpentTagRun(t, true)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls", run.divs, run.stalls)
	}
	for _, n := range []string{"Marut", "Cataclysmic Prospecting", "Prossh, Skyraider of Kher", "Bat Colony"} {
		if run.casts[n] < 5 {
			t.Errorf("%s was cast %d times in %d games, want at least 5", n, run.casts[n], run.games)
		}
	}
}

// TestManaSpentTagMutation: recording the cast's total in place of the named
// tag's units must diverge.
func TestManaSpentTagMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSpentTagTotal = true
	defer func() { dirac.DebugSpentTagTotal = false }()
	manaSpentTagRun(t, false).requireDivergence(t, "the total recorded in place of the tagged spend")
}
