package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Suppressor Skyguard (kgen lower_delaytrigger_player.go): "whenever a player
// attacks you, if that player has another opponent who isn't being attacked".
// With two seats the attacker's only opponent is the attacked one, so the
// trigger must stay silent on both sides; the positive arm (a second,
// unattacked opponent) needs three seats and is proven in engine/rules
// (TestSuppressorSkyguardOpponentGate).
var suppressorSkyguardDecks = [2][]cardset.Entry{
	{
		{Name: "Elite Vanguard", Count: 20}, {Name: "Plains", Count: 20},
	},
	{
		{Name: "Suppressor Skyguard", Count: 8}, {Name: "Elite Vanguard", Count: 8},
		{Name: "Plains", Count: 12}, {Name: "Island", Count: 12},
	},
}

func TestSuppressorSkyguardLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "suppressorskyguard", suppressorSkyguardDecks)
	// The silent trigger is the 2026-11-20 spelling's reading
	// (CheckDefinedPlayer$ OpponentOf TriggeredAttackingPlayer+!Defending).
	// A corpus that predates that pin carries Condition$
	// AttackerHasUnattackedOpp, which both sides read as "always", so the
	// trigger fires there and this assertion cannot hold: skip, and let the
	// pin's own run (the lander's) prove it.
	sky, ok := reg.Lookup("Suppressor Skyguard")
	if !ok || len(sky.Faces) == 0 || len(sky.Faces[0].Triggers) == 0 {
		t.Fatal("Suppressor Skyguard is not in the corpus with a trigger")
	}
	if _, ok := sky.Faces[0].Triggers[0].Param(cards.PKCheckDefinedPlayer); !ok {
		t.Skip("the corpus predates the 2026-11-20 Forge pin: Suppressor Skyguard still spells its gate Condition$ AttackerHasUnattackedOpp")
	}
	tbl, _, _, err := set.Matchup(suppressorSkyguardDecks[0], suppressorSkyguardDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 64
	var casts, resolved, pushes int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"suppressorskyguard-a", "suppressorskyguard-b"}, gd, 1)
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
				onBoard := false
				for i := range g.E.G.Objs {
					o := &g.E.G.Objs[i]
					if o.Face() != nil && o.Face().Name == "Suppressor Skyguard" && o.Zone == state.ZBattlefield {
						onBoard = true
					}
				}
				if onBoard {
					resolved++
				}
				for _, ev := range g.E.L.Events {
					if ev.Kind != events.TriggerPush {
						continue
					}
					if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Suppressor Skyguard" {
						pushes++
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
	if resolved == 0 {
		t.Fatal("no game resolved a Suppressor Skyguard onto the battlefield; the silent trigger proves nothing")
	}
	if pushes != 0 {
		t.Fatalf("Suppressor Skyguard's trigger fired %d times in two-seat games: the attacker's only opponent is the attacked seat", pushes)
	}
	t.Logf("%d seeds, %d casts/activations, %d games with a Skyguard in play, 0 trigger pushes", seeds, casts, resolved)
}
