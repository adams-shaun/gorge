package effects

import "testing"

// TestGreatestPhasedOutIsOutsideTheComparisonSet pins that a phased-out
// battlefield card is not part of the set greatestPower, greatestCMC_ and
// lowestCMC compare against (CR 702.26b; Forge's game.getCardsIn(Battlefield)
// filters phased-out cards). Disciple of Caelus Nin phases out the opponent's
// bigger creature; Highcliff Felidar's "greatest power among creatures that
// player controls" must then choose the biggest one still phased in.
func TestGreatestPhasedOutIsOutsideTheComparisonSet(t *testing.T) {
	g, id := cmcBoard(t)
	cases := []struct {
		name string
		spec string
		obj  string
		want bool
	}{
		{"power: the biggest phased-in creature is the greatest", "Creature.greatestPower", "myBig", true},
		{"power: a phased-out bigger one never matches", "Creature.greatestPower", "theBig2", false},
		{"cmc: myBig is the max once both dearer creatures phased out", "Creature.greatestCMC_Creature", "myBig", true},
		{"cmc: a phased-out dearer creature stays out", "Creature.greatestCMC_Creature", "theSmall", false},
		{"lowest: the phased-out lowest is skipped", "Permanent.nonLand+lowestCMC", "myRock", true},
	}
	// before phasing: theBig2 (6/6, cmc 5) is the global maximum
	if !MatchesSpec(g, "Creature.greatestPower", id["theBig2"], 0) || MatchesSpec(g, "Creature.greatestPower", id["myBig"], 0) {
		t.Fatal("precondition: the 6/6 is the greatest-power creature before it phases out")
	}
	// theBig (5/5, cmc 5) and mySmall (1/1, cmc 1) phase out too: the 4/4 is
	// then the dearest creature and the 2-cost artifact the cheapest nonland.
	g.Obj(id["theBig2"]).PhasedOut = true
	g.Obj(id["theBig"]).PhasedOut = true
	g.Obj(id["mySmall"]).PhasedOut = true
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchesSpec(g, c.spec, id[c.obj], 0); got != c.want {
				t.Errorf("MatchesSpec(%q, %s) = %v, want %v", c.spec, c.obj, got, c.want)
			}
		})
	}
}
