package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The ControlledBy TargetedOrController lockstep: the filter word resolves
// to each creature the target player controls, or the controller of the
// target planeswalker. The sweep cards below exercise it through
// DealDamage + DamageAll, and Flaming Gambit exercises the optional
// ChooseCard whose chooser is the target and whose pool is the chooser's
// own creatures (Defined$ TargetedOrController, ControlledByPlayer$ Chooser).
//
// The targets are players, creatures and planeswalkers, so both arms of the
// referent run: a player target names itself, a planeswalker target names
// its controller.
var kctrltgtDecks = [2][]cardset.Entry{
	// A: bodies, targeted removal and two planeswalkers to target.
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Serra Angel", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 2},
		{Name: "Doom Blade", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 4},
	},
	// B: every newly lowered carrier, beside a small red removal package.
	{
		{Name: "Chandra's Fury", Count: 3}, {Name: "Lavalanche", Count: 2},
		{Name: "Flame Wave", Count: 2}, {Name: "Chandra Nalaar", Count: 2},
		{Name: "Chandra, Pyrogenius", Count: 2}, {Name: "Soul of Shandalar", Count: 2},
		{Name: "Flaming Gambit", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 3}, {Name: "Swamp", Count: 2},
	},
}

// kctrltgtWant is every card the change newly lowered. The ultimate keys
// name Chandra Nalaar's and Chandra, Pyrogenius's DamageAll ability, which
// is the lowering the filter word reaches on those planeswalkers.
var kctrltgtWant = map[string]string{
	"Chandra's Fury":      "",
	"Lavalanche":          "",
	"Flame Wave":          "",
	"Flaming Gambit":      "",
	"Soul of Shandalar":   "",
	"Chandra Nalaar":      "f0/a2",
	"Chandra, Pyrogenius": "f0/a2",
}

// kctrltgtRun plays pol over seeds and accumulates, from gorge's side, how
// often each wanted card was cast and how often its wanted ability key was
// used.
func kctrltgtRun(t *testing.T, pol func(uint64) Policy, seeds int, cast, keys map[string]int) (games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ctrltgt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kctrltgtDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kctrltgtDecks[0], kctrltgtDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ctrltgt-a", "ctrltgt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		cov := CoverageOf(g, m)
		CloseSide(k)
		games++
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			stalls++
			t.Logf("seed %d: stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		for name, key := range kctrltgtWant {
			cc := cov.Cards[name]
			if cc == nil {
				continue
			}
			if cc.Cast > 0 {
				cast[name]++
			}
			if key != "" && cc.Used[key] > 0 {
				keys[name]++
			}
		}
	}
	return games, divs, stalls
}

// kctrltgtUltimatePolicy is ExplorePolicy, but a priority decision that can
// activate Chandra, Pyrogenius or Chandra Nalaar takes the +ability while
// the ultimate is not yet offered and the ultimate once it is. The two
// ultimates are the DamageAll lowering this change reaches.
func kctrltgtUltimatePolicy(seed uint64) Policy {
	base := ExplorePolicy(seed)
	ult := map[string]uint8{"Chandra, Pyrogenius": 2, "Chandra Nalaar": 2}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority && c.G != nil {
			var plus, ultimate Canon
			havePlus, haveUlt := false, false
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate {
					continue
				}
				id, ok := kroleRefObj(c.G, a.Src())
				if !ok {
					continue
				}
				o := c.G.E.G.Obj(id)
				if o == nil || o.Face() == nil || o.Face().Name != "Chandra, Pyrogenius" && o.Face().Name != "Chandra Nalaar" {
					continue
				}
				if a.Mode() == ult[o.Face().Name] {
					ultimate, haveUlt = a, true
				} else if a.Mode() == 0 {
					plus, havePlus = a, true
				}
			}
			if haveUlt {
				return ultimate
			}
			if havePlus {
				return plus
			}
		}
		return base.Pick(c)
	})
}

// TestCtrlTgtLockstep: random and explore policies play the matchup with no
// divergence and no unmodelled stall, every newly lowered card is cast at
// least once, and the two planeswalkers also use the ultimate whose sweep is
// the lowering.
func TestCtrlTgtLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	if games, divs, stalls := kctrltgtRun(t, RandomPolicy, 32, map[string]int{}, map[string]int{}); games != 32 || divs != 0 || stalls != 0 {
		t.Fatalf("random: %d games, %d divergences, %d stalls", games, divs, stalls)
	}
	cast, keys := map[string]int{}, map[string]int{}
	games, divs, stalls := kctrltgtRun(t, kctrltgtUltimatePolicy, 32, cast, keys)
	t.Logf("%d explore games, %d divergences, %d stalls; cast %v; ultimates %v", games, divs, stalls, cast, keys)
	if divs != 0 || stalls != 0 {
		t.Errorf("%d divergences, %d stalls", divs, stalls)
	}
	for name, key := range kctrltgtWant {
		if cast[name] == 0 {
			t.Errorf("%s was never cast on gorge's side", name)
		}
		if key != "" && keys[name] == 0 {
			t.Errorf("%s never used the ultimate %s on gorge's side", name, key)
		}
	}
}
