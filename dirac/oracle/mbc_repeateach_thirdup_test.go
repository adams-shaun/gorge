package oracle

import "testing"

// The RepeatEach-over-players /ThirdUp spelling (Pox): every player loses a
// third of their life, discards a third of their hand and sacrifices a third
// of their creatures and lands, each rounded up, each read off the repeated
// player (PlayerCountRemembered$ .../ThirdUp). The fixture pool is
// dirac/cardset/pools/fixtures/mbcrepeateach3.

var mbcRepeatEach3Decks = []fixtureDecks{{
	{
		{Name: "Pox", Count: 8}, {Name: "Walking Corpse", Count: 10}, {Name: "Dark Ritual", Count: 2},
		{Name: "Swamp", Count: 17}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Pox", Count: 4}, {Name: "Walking Corpse", Count: 14}, {Name: "Dark Ritual", Count: 2},
		{Name: "Swamp", Count: 17}, {Name: "Mountain", Count: 3},
	},
}}

var mbcRepeatEach3Watch = map[string]bool{"Pox": true}

func TestMBCRepeatEach3Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "mbcrepeateach3", mbcRepeatEach3Decks, mbcRepeatEach3Watch, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Pox"] < 50 {
		t.Errorf("only %d Pox casts", run.casts["Pox"])
	}
}
