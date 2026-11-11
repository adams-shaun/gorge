package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// DB$ SkipPhase, all combat phases of the player's next turn (False Peace,
// Empty City Ruse): the lockstep of dirac's pending/live bits against gorge's
// state.Game.PhaseSkips. Both decks cast the two spells at a random player;
// deck A also plays Aurelia, the Warleader, so the phase ledger that counts
// combats (addPhaseNext) runs beside the skip. Seats are swapped: each deck
// plays both seats.

var kskipphaseDecks = [2][]cardset.Entry{
	{ // A: the two skip spells and the extra-combat carrier
		{Name: "False Peace", Count: 4}, {Name: "Empty City Ruse", Count: 4}, {Name: "Aurelia, the Warleader", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 10},
	},
	{ // B: the skip spells beside plain attackers
		{Name: "False Peace", Count: 3}, {Name: "Empty City Ruse", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9},
	},
}

// kskipphaseStats is what the policy counts across the games, on gorge's state.
type kskipphaseStats struct {
	skippedTurns int // turns whose postcombat main phase came with no combat begun while a skip was live
	pendingSeen  int // decisions taken with a skip pending (cast, not yet live)
}

type kskipphaseProbe struct {
	st      *kskipphaseStats
	base    Policy
	r       *rand.Rand
	turn    int32
	counted bool
}

func (p *kskipphaseProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	if int32(g.Turn) != p.turn {
		p.turn, p.counted = int32(g.Turn), false
	}
	for _, v := range g.PhaseSkips {
		if v&state.SkipCombatPending != 0 {
			p.st.pendingSeen++
			break
		}
	}
	if g.CombatSkipped(g.Active) && g.Step >= state.StepMain2 && g.CombatsThisTurn == 0 && !p.counted {
		p.counted = true
		p.st.skippedTurns++
	}
}

func (p *kskipphaseProbe) pick(c *PickCtx) Canon {
	var casts, attacks []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActCast:
			casts = append(casts, a)
		case ActAttack:
			attacks = append(attacks, a)
		}
	}
	a := p.base.Pick(c)
	// prefer casting (the skip spells are the point), then attacking
	if len(casts) > 0 && p.r.IntN(3) != 0 {
		a = casts[p.r.IntN(len(casts))]
	} else if len(attacks) > 0 && p.r.IntN(8) != 0 {
		a = attacks[p.r.IntN(len(attacks))]
	}
	if c.G != nil {
		p.observe(c.G)
	}
	return a
}

// runSkipphase plays the constructed matchups (both seat orders) and returns
// the games, the divergences, the stalls and the carrier counts.
func runSkipphase(t *testing.T, seeds int, report bool) (games, divs, stalls int, st kskipphaseStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("skipphase")
	if err != nil {
		t.Fatal(err)
	}
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{kskipphaseDecks[order], kskipphaseDecks[1-order]}
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("order %d deck %d: %d cards", order, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kskipphase-a", "kskipphase-b"}, decks, 1)
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
			probe := &kskipphaseProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x736b6970))}
			res := LockstepLed(m, seed, g, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if report {
					t.Errorf("order %d seed %d: kernel stall %q", order, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if report {
					lo := max(d.At-12, 0)
					t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestSkipphaseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runSkipphase(t, 48, true)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		n, min int
		what   string
	}{
		{st.skippedTurns, 20, "turns whose combat was skipped"}, {st.pendingSeen, 20, "decisions with a skip pending"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.what, c.n, c.min)
		}
	}
}

// TestSkipphaseMutation: a skip that never goes live must make the lockstep
// diverge.
func TestSkipphaseMutation(t *testing.T) {
	dirac.DebugSkipPhaseNeverLive = true
	defer func() { dirac.DebugSkipPhaseNeverLive = false }()
	games, divs, _, _ := runSkipphase(t, 16, false)
	if divs == 0 {
		t.Fatalf("DebugSkipPhaseNeverLive: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
