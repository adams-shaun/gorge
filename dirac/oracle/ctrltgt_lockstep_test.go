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
// often each wanted card was cast or had the wanted ability key used.
func kctrltgtRun(t *testing.T, pol func(uint64) Policy, seeds int, used map[string]int) (games, divs, stalls int) {
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
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
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
				used[name]++
			}
			if key != "" && cc.Used[key] > 0 {
				used[name]++
			}
		}
	}
	return games, divs, stalls
}

// TestCtrlTgtLockstep: random and explore policies play the matchup with no
// divergence and no unmodelled stall, and every newly lowered card is cast
// at least once (the two planeswalkers also use the ultimate whose sweep is
// the lowering).
func TestCtrlTgtLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	used := map[string]int{}
	if games, divs, stalls := kctrltgtRun(t, RandomPolicy, 32, used); games != 32 || divs != 0 || stalls != 0 {
		t.Fatalf("random: %d games, %d divergences, %d stalls", games, divs, stalls)
	}
	explore := map[string]int{}
	games, divs, stalls := kctrltgtRun(t, ExplorePolicy, 32, explore)
	t.Logf("%d explore games, %d divergences, %d stalls; usage %v", games, divs, stalls, explore)
	if divs != 0 || stalls != 0 {
		t.Errorf("%d divergences, %d stalls", divs, stalls)
	}
	for name := range kctrltgtWant {
		if explore[name] == 0 {
			t.Errorf("%s was never played on gorge's side", name)
		}
	}
}
