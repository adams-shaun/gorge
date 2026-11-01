package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Token ValidTgts$ / TokenOwner$ Targeted lockstep:
// Hunted Bonebrute and The Sentry make the targeted opponent create a token,
// and Emeritus of Truce lets any targeted player (the caster included) create
// one. gorge reads the Token SA's own ValidTgts$ through definedSpec
// (effects/token.go tokenOwnerPlayers), so the token's controller is the
// target; the kernel resolves the same player target as the token owner
// (dirac/resolve.go playerRef, interp_gen.go OpCreateToken).
var ktokownerDecks = [2][]cardset.Entry{
	{
		{Name: "Hunted Bonebrute", Count: 4}, {Name: "Emeritus of Truce", Count: 4},
		{Name: "The Sentry, Golden Guardian", Count: 2},
		{Name: "Doom Blade", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		// the mirror with a different removal curve
		{Name: "Hunted Bonebrute", Count: 4}, {Name: "Emeritus of Truce", Count: 4},
		{Name: "The Sentry, Golden Guardian", Count: 2},
		{Name: "Doom Blade", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
}

// ktokownerCarriers names the three token-making cards whose casts the test
// counts.
var ktokownerCarriers = map[string]bool{
	"Hunted Bonebrute": true, "The Sentry, Golden Guardian": true, "Emeritus of Truce": true,
}

// ktokownerPolicy casts the carriers whenever it can, otherwise plays a land
// or picks at random, and counts the carrier casts and the Emeritus target
// answers (the only decision here that offers both players: ValidTgts$
// Player). RefPlayer+seat is a player target.
func ktokownerPolicy(seed uint64, casts, opp, self *int) Policy {
	rnd := RandomPolicy(seed)
	sub := func(c *PickCtx, opts []Canon) Canon {
		d := *c.D
		d.Options = opts
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	countCast := func(c *PickCtx, a Canon) {
		if a.Kind() != ActCast || c.G == nil {
			return
		}
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ktokownerCarriers[ob.Face().Name] {
				*casts++
			}
		}
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			var players []Canon
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && a.Tgt() >= RefPlayer {
					players = append(players, a)
				}
			}
			if len(players) > 0 {
				if len(players) == 1 {
					return players[0] // Hunted Bonebrute / The Sentry: the opponent
				}
				// Emeritus of Truce's ValidTgts$ Player: the caster or the
				// opponent, spread over the seeds.
				own := RefPlayer + Ref(c.D.Player)
				if (seed+uint64(c.Seq))%3 == 0 {
					for _, a := range players {
						if a.Tgt() == own {
							*self++
							return a
						}
					}
				}
				for _, a := range players {
					if a.Tgt() != own {
						*opp++
						return a
					}
				}
			}
		}
		if c.D.Kind == DPriority {
			var carriers, lands []Canon
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast:
					if c.G != nil {
						if o, ok := c.G.GorgeOption(a); ok {
							if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ktokownerCarriers[ob.Face().Name] {
								carriers = append(carriers, a)
							}
						}
					}
				case ActPlayLand:
					lands = append(lands, a)
				}
			}
			if len(carriers) > 0 {
				a := sub(c, carriers)
				countCast(c, a)
				return a
			}
			if len(lands) > 0 {
				return sub(c, lands)
			}
		}
		a := rnd.Pick(c)
		countCast(c, a)
		return a
	})
}

// runTokowner plays seeds mirror games and returns the divergences, the
// carrier casts and Emeritus's two target arms.
func runTokowner(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seeds uint64, failOnDiv bool) (games, divs, casts, opp, self int) {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktokowner-a", "ktokowner-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := ktokownerPolicy(seed, &casts, &opp, &self)
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if failOnDiv {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, casts, opp, self
}

func TestTokownerLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tokowner", ktokownerDecks)
	games, divs, casts, opp, self := runTokowner(t, reg, set, decks, tbl, 128, true)
	t.Logf("%d games, %d divergences, carrier casts %d, Emeritus targets: opponent %d, self %d", games, divs, casts, opp, self)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if casts < 60 {
		t.Errorf("carrier casts = %d, want at least 60", casts)
	}
	if opp < 40 {
		t.Errorf("Emeritus opponent targets = %d, want at least 40", opp)
	}
	if self < 10 {
		t.Errorf("Emeritus self targets = %d, want at least 10", self)
	}
}

// ktokownerMutate rewrites each carrier's token-making instruction from the
// targeted player to the resolving controller: gorge gives the token to the
// target, so every game that casts a carrier must diverge.
func ktokownerMutate(t *testing.T, tbl *kop.Table) int {
	t.Helper()
	n := 0
	for ci := range tbl.Cards {
		d := &tbl.Cards[ci]
		if !ktokownerCarriers[d.Name] {
			continue
		}
		for ai := range d.Abilities {
			for ii := range d.Abilities[ai].Prog {
				in := &d.Abilities[ai].Prog[ii]
				if in.Op != kop.OpCreateToken && in.Op != kop.OpCreateTokenEx {
					continue
				}
				if in.Obj >= kop.RefTarget0 && in.Obj <= kop.RefTarget2 {
					in.Obj = kop.RefController
					n++
				}
			}
		}
	}
	return n
}

func TestTokownerMutation(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tokowner", ktokownerDecks)
	n := ktokownerMutate(t, tbl)
	if n == 0 {
		t.Fatal("no carrier token instruction with a target owner to rewrite")
	}
	games, divs, _, _, _ := runTokowner(t, reg, set, decks, tbl, 16, false)
	if divs == 0 {
		t.Fatalf("mutation rewrote %d instructions and %d games did not diverge", n, games)
	}
	t.Logf("mutation rewrote %d instructions and diverged in %d of %d games", n, divs, games)
}
