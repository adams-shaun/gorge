package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// DB$ SkipPhase, the one-shot grants (Fatigue and Avizoa's Step$ Draw / Untap,
// Blinding Angel and Stonehorn Dignitary's Phase$ BeginCombat, Moment of
// Silence's Duration$ NextThisTurn): the lockstep of dirac's per-seat grant byte
// against gorge's state.Game.PhaseSkips. Deck B also plays Aurelia, the
// Warleader, so an extra combat runs beside the one-combat grant. Seats are
// swapped: each deck plays both seats.

var kskipstepsDecks = [2][]cardset.Entry{
	{ // A: the skip carriers over white and blue
		{Name: "Fatigue", Count: 3}, {Name: "Moment of Silence", Count: 3}, {Name: "Avizoa", Count: 3},
		{Name: "Blinding Angel", Count: 3}, {Name: "Stonehorn Dignitary", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Island", Count: 8},
	},
	{ // B: the carriers beside the extra-combat card
		{Name: "Aurelia, the Warleader", Count: 2}, {Name: "Fatigue", Count: 3}, {Name: "Moment of Silence", Count: 3},
		{Name: "Stonehorn Dignitary", Count: 3}, {Name: "Blinding Angel", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Mountain", Count: 5}, {Name: "Island", Count: 4},
	},
}

// kskipstepsStats is what the policy counts across the games, on gorge's state.
type kskipstepsStats struct {
	untapSeen, drawSeen, combatSeen int // decisions taken with that one-shot grant pending
	thisTurnSeen                    int // decisions taken with a this-turn grant armed
}

type kskipstepsProbe struct {
	st   *kskipstepsStats
	base Policy
	r    *rand.Rand
}

func (p *kskipstepsProbe) observe(gs *GorgeSide) {
	for _, v := range gs.E.G.PhaseSkips {
		if v&state.SkipUntapNext != 0 {
			p.st.untapSeen++
		}
		if v&state.SkipDrawNext != 0 {
			p.st.drawSeen++
		}
		if v&state.SkipCombatOnce != 0 {
			p.st.combatSeen++
		}
		if v&(state.SkipDrawTurn|state.SkipCombatTurn) != 0 {
			p.st.thisTurnSeen++
		}
	}
}

func (p *kskipstepsProbe) pick(c *PickCtx) Canon {
	var casts, attacks []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActCast, ActActivate:
			casts = append(casts, a)
		case ActAttack:
			attacks = append(attacks, a)
		}
	}
	a := p.base.Pick(c)
	// prefer casting and activating (the skip carriers are the point), then attacking
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

// runSkipphaseSteps plays the constructed matchups (both seat orders) and
// returns the games, the divergences, the stalls and the carrier counts.
func runSkipphaseSteps(t *testing.T, seeds int, report bool) (games, divs, stalls int, st kskipstepsStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("skipphasesteps")
	if err != nil {
		t.Fatal(err)
	}
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{kskipstepsDecks[order], kskipstepsDecks[1-order]}
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
			m, err := NewMatchup(reg, [2]string{"kskipsteps-a", "kskipsteps-b"}, decks, 1)
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
			probe := &kskipstepsProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x736b7374))}
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

func TestSkipphaseStepsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runSkipphaseSteps(t, 48, true)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		n, min int
		what   string
	}{
		{st.untapSeen, 5, "decisions with an untap skip pending"}, {st.drawSeen, 5, "decisions with a draw skip pending"},
		{st.combatSeen, 5, "decisions with a combat skip pending"}, {st.thisTurnSeen, 5, "decisions with a this-turn skip armed"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.what, c.n, c.min)
		}
	}
}

// TestSkipphaseStepsMutation: a one-shot grant that is never armed must make
// the lockstep diverge.
func TestSkipphaseStepsMutation(t *testing.T) {
	dirac.DebugSkipOneShotIgnored = true
	defer func() { dirac.DebugSkipOneShotIgnored = false }()
	games, divs, _, _ := runSkipphaseSteps(t, 16, false)
	if divs == 0 {
		t.Fatalf("DebugSkipOneShotIgnored: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
