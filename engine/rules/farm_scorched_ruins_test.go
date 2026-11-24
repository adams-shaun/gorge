package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestFarmScorchedRuins(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(`{"name":"xengine window RAND-c0-g1-0-WU vs RAND-c0-g1-1-WU turn 7", "turn":1,"why":"One turn of a recorded gorge game between two explore bots, replayed from its first precombat main phase.","setup":{"p0":{"battlefield": ["Island","Plains","Sheltering Prayers","Island"],"tapped_after_untap": ["Island","Plains"],"hand":["Glimmering Angel","Scorched Ruins","Kjeldoran Home Guard","Tangle Kelp","Teferi's Puzzle Box","Library of Lat-Nam"], "library_top":["Mercadian Atlas","Ghost Town","Defiant Vanguard","Three Wishes","Kjeldoran Skycaptain","Veiled Serpent","Flux","Paradigm Shift","Jeweled Amulet","Alaborn Zealot","Rebound","Plains","Shimmer","Island", "Island","Island","Island","Plains","Island","Acidic Dagger","Plains","Island", "Plains","Island","Plains","Faerie Squadron","Plains","Island","Lat-Nam's Legacy","Lotus Vale"],"life":20},"p1":{"battlefield":["Plains","Island"], "tapped":["Plains","Island"],"hand":["Eightfold Maze","War Tax","Kukemssa Serpent","Defiant Vanguard","Sanctum Guardian","Unstable Shapeshifter"], "graveyard":["Scent of Jasmine","Mana Vapors"],"library_top":["Zanam Djinn","Plains","Phantasmal Mount","Island","Charm Peddler","Bone Mask","Tsabo's Web","Sea Scryer","Island","Flux","Island","Kjeldoran Skycaptain","Plains", "Island","Denied!","Island","Eye Spy","Three Wishes","Plains","Island","Plains", "Goblin Lyre","Convalescence","Plains","Island","Illusionary Terrain","Island", "Island","Time Spiral","Plains"],"life":26}},"steps":[{"op":"play","seat":0, "card":"p0:Scorched Ruins"}]}`))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := last.Players[0].Graveyard; !slices.Equal(got, []string{"Scorched Ruins"}) {
		t.Fatalf("p0 graveyard after the play = %v, want exactly [Scorched Ruins] (XMage: the sacrifice cannot be made, so the ruins go to the graveyard)", got)
	}
	islands := 0
	for _, p := range last.Permanents {
		if p.Name == "Island" && p.Controller == 0 {
			islands++
		}
	}
	if islands != 2 {
		t.Fatalf("p0 Islands on the battlefield = %d, want 2 (the lone untapped land must not be sacrificed)", islands)
	}
}
