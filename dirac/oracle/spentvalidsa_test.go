package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// spentvalidsa pool, whose carriers read the cast's pool mana spent as a
// fire-time SpellCast filter (ValidSA$ / ValidSAonCard$ Spell.ManaSpent
// <op><n|X>; gorge trigmatch spellCastMatches; kernel State.spentGateOK over
// the spell entry's recorded spend). GE4 / GE8 read a literal; Tokka &
// Rahzar's LTX reads the cast spell's PRINTED mana value. The decks mix the
// carriers with spells at MV 1-4, X spells (Blaze, Crater's Claws), a kicker
// (Burst Lightning) and Goblin Electromancer, whose reduction puts the spend
// below the printed mana value.
//
// The Emperor of Palamecia is in deck A: its restricted "Add {U} or {R}" mana
// reaches a lowered {X} cast's witness, which counts it as gorge's census
// does (lowerWitnessEx).

var kspentvalidsaDecks = [2][]cardset.Entry{
	{
		{Name: "Sahagin", Count: 2}, {Name: "The Emperor of Palamecia", Count: 1}, {Name: "Prompto Argentum", Count: 2},
		{Name: "Ultros, Obnoxious Octopus", Count: 1}, {Name: "Blazing Bomb", Count: 2}, {Name: "The Prima Vista", Count: 1},
		{Name: "Goblin Electromancer", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Burst Lightning", Count: 2},
		{Name: "Blaze", Count: 2}, {Name: "Crater's Claws", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Rite of Replication", Count: 1}, {Name: "Wizard's Lightning", Count: 1}, {Name: "Counterspell", Count: 1},
		{Name: "Opt", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Tokka & Rahzar, Terrible Twos", Count: 2}, {Name: "Sahagin", Count: 1}, {Name: "Ultros, Obnoxious Octopus", Count: 1},
		{Name: "Blazing Bomb", Count: 1}, {Name: "Prompto Argentum", Count: 1}, {Name: "The Prima Vista", Count: 1},
		{Name: "Goblin Electromancer", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Burst Lightning", Count: 2}, {Name: "Blaze", Count: 2}, {Name: "Crater's Claws", Count: 1},
		{Name: "Divination", Count: 2}, {Name: "Rite of Replication", Count: 2}, {Name: "Counterspell", Count: 1},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 7},
	},
}

func kspentvalidsaSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spentvalidsa")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func kspentvalidsaDecksOf(t *testing.T, reg *cards.Registry) [2][]*cards.Card {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kspentvalidsaDecks {
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
	return decks
}

// kspentvalidsaRun plays seeds 1..seeds; it returns whether any diverged
// (the first is logged when stopOnDiv) and the stalls.
func kspentvalidsaRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, n map[string]int, stopOnDiv bool) (div bool, stalls int) {
	t.Helper()
	decks := kspentvalidsaDecksOf(t, reg)
	tbl, _, _, err := set.Matchup(kspentvalidsaDecks[0], kspentvalidsaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kspentvalidsa-a", "kspentvalidsa-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		dirac.SpentGateLog = nil
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			for i, r := range dirac.SpentGateLog {
				t.Logf("gate[%d] spent=%d cmp=%d rhs=%d x=%v admitted=%v", i, r.Spent, r.Cmp, r.Rhs, r.X, r.Admitted)
			}
			return
		}
		for _, r := range dirac.SpentGateLog {
			switch {
			case r.X:
				n["LTX decisions"]++
				if r.Admitted {
					n["LTX admitted"]++
				}
			case r.Cmp == kop.DmgGE && r.Rhs == 4:
				if r.Admitted {
					n["GE4 admitted"]++
				} else {
					n["GE4 rejected"]++
				}
			case r.Cmp == kop.DmgGE && r.Rhs == 8:
				n["GE8 decisions"]++
			}
		}
	}
	return
}

func TestSpentvalidsaLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kspentvalidsaSetup(t)
	dirac.DebugSpentGateLog = true
	defer func() { dirac.DebugSpentGateLog = false }()
	n := map[string]int{}
	div, stalls := kspentvalidsaRun(t, reg, set, 256, n, true)
	t.Logf("diverged %v, stalls %d, counts %v", div, stalls, n)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if got := n["GE8 decisions"] + n["LTX decisions"]; got < 5 {
		t.Errorf("GE8 or LTX decisions reached %d, want at least 5", got)
	}
	for _, b := range []struct {
		key  string
		want int
	}{
		{"GE4 admitted", 40},
		{"GE4 rejected", 40},
	} {
		if n[b.key] < b.want {
			t.Errorf("branch %q reached %d, want at least %d", b.key, n[b.key], b.want)
		}
	}
}

// TestSpentvalidsaMutation: comparing the printed mana value instead of the
// spend (DebugSpentGateUseMV) must make the lockstep diverge on some seed.
func TestSpentvalidsaMutation(t *testing.T) {
	reg, set := kspentvalidsaSetup(t)
	dirac.DebugSpentGateUseMV = true
	defer func() { dirac.DebugSpentGateUseMV = false }()
	div, _ := kspentvalidsaRun(t, reg, set, 64, map[string]int{}, false)
	if !div {
		t.Fatalf("DebugSpentGateUseMV: no divergence in 64 seeds")
	}
}
