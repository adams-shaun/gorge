package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// Kitt Kanto, Mayhem Diva (kgen lower_delaytrigger_player.go): at the
// beginning of combat on each player's turn its controller may tap two
// untapped creatures, and when they do the active player's creature gets
// +2/+2 and trample and is goaded. Both seats play it, so the pump runs on
// the controller's own turn and on the opponent's.
var kittKantoDecks = [2][]cardset.Entry{
	{
		{Name: "Kitt Kanto, Mayhem Diva", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Kitt Kanto, Mayhem Diva", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
	},
}

func TestKittKantoLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "kittkanto", kittKantoDecks)
	tbl, _, _, err := set.Matchup(kittKantoDecks[0], kittKantoDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 64
	var casts, goads int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"kittkanto-a", "kittkanto-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
			Finish: func(g *GorgeSide) {
				for _, ev := range g.E.L.Events {
					if ev.Kind == events.Goad {
						goads++
					}
				}
			}})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		for _, a := range res.Actions {
			if kd := a.Kind(); kd == ActCast || kd == ActCastFace || kd == ActActivate {
				casts++
			}
		}
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	if casts == 0 {
		t.Fatal("no cast or activation in the games; the lockstep proves nothing")
	}
	if goads == 0 {
		t.Fatal("no goad in any game: Kitt Kanto's pump never resolved, so the target filter was never judged")
	}
	t.Logf("%d seeds, %d casts/activations, %d goads", seeds, casts, goads)
}
