package oracle

import "testing"

// TestMBCZoneMoves5Lowers plays the MBC zone-moves-5 spellings kgen now
// lowers, one carrier each, in lockstep with gorge. The fixture pool is
// dirac/cardset/pools/fixtures/mbczonemoves5:
//
//   - count:TopOfLibraryCMC: Counterbalance (ExMVOf RefLibTop: the top card
//     of the controller's library, 0 when it is empty)
//   - param:RearrangeTopOfLibrary.AIRespondsToOwnAbility: Sensei's Divining
//     Top (an AI hint, read as nothing)
//   - param:ChangeZone.Reorder: Goblin Recruiter (a library-to-top search;
//     gorge places the found cards in the answer order either way)
//   - prop:ThisTurnEnteredFrom_Library: Captain N'ghathrod's end-step target
//     (kop.W7EnteredFromLib). The Weaver King, the MBC carrier, still refuses
//     on its per-opponent pick (DefinedPlayer$ Opponent with Chooser$ You);
//     Kagha, Shadow Archdruid and Coram, the Undertaker refuse on the
//     graveyard MayPlay shapes gorge reads differently (lower_trigger.go
//     laterPermanentAlternative, ownGraveLandsOnly)
//
// Without the lowering each carrier is refused and the pool's matchup fails
// to build.
func TestMBCZoneMoves5Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "mbczonemoves5", mbcZoneMoves5Decks, mbcZoneMoves5Watch, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for name, min := range map[string]int{
		"Counterbalance": 40, "Sensei's Divining Top": 40, "Goblin Recruiter": 40,
		"Captain N'ghathrod": 40,
	} {
		if run.casts[name] < min {
			t.Errorf("only %d %s casts (want %d)", run.casts[name], name, min)
		}
	}
}

var mbcZoneMoves5Watch = map[string]bool{
	"Counterbalance": true, "Sensei's Divining Top": true, "Goblin Recruiter": true,
	"Captain N'ghathrod": true,
}

var mbcZoneMoves5Decks = []fixtureDecks{
	{
		// Counterbalance and Top against a cheap Goblin deck that casts
		// spells of several mana values into it
		{
			{Name: "Counterbalance", Count: 6}, {Name: "Sensei's Divining Top", Count: 6}, {Name: "Opt", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Goblin Recruiter", Count: 8}, {Name: "Raging Goblin", Count: 6}, {Name: "Goblin Piker", Count: 6},
			{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 16},
		},
	},
	{
		// the Captain (a menace Horror) mills the defender for its combat
		// damage, and its end step takes a creature milled that turn
		{
			{Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 14}, {Name: "Forest", Count: 18},
		},
		{
			{Name: "Captain N'ghathrod", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Opt", Count: 4},
			{Name: "Island", Count: 12}, {Name: "Swamp", Count: 12},
		},
	},
}
