package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cards-discarded-this-turn lockstep: Green Goblin,
// Revenant (attack: discard, then draw per card discarded this turn) and
// Misty Knight, Hero for Hire (T, discard a card: draw per card discarded
// this turn) over discard outlets (Rummaging Goblin's cost, Tormenting
// Voice's additional cost, Mind Rot and Faithless Looting on resolution) and a
// cycling card (Gempalm Incinerator). Gorge's answer at every decision is the
// expected one.
var kdiscturnDecks = [2][]cardset.Entry{
	{ // A: B/R
		{Name: "Green Goblin, Revenant", Count: 4}, {Name: "Misty Knight, Hero for Hire", Count: 4}, {Name: "Rummaging Goblin", Count: 3},
		{Name: "Tormenting Voice", Count: 3}, {Name: "Mind Rot", Count: 2}, {Name: "Faithless Looting", Count: 3},
		{Name: "Gempalm Incinerator", Count: 3}, {Name: "Shock", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B: a generic creature deck with a few outlets
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Mind Rot", Count: 2}, {Name: "Rummaging Goblin", Count: 2}, {Name: "Gempalm Incinerator", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 8},
	},
}

type kdiscturnStats struct {
	games, discards           int
	ge2Turns, nonzeroTurns    int // player-turns with >= 2 / >= 1 cards discarded
	resetTurns, maxPerTurn    int // a player-turn reading 0 after a nonzero one of its own; the largest tally
	carrierGE2, carrierLiveGE int // turns with tally >= 2 and a carrier (Green Goblin / Misty Knight) in play; any tally
}

func kdiscturnSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("discturn")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdiscturnDecks {
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
	return reg, set, decks
}

// kdiscturnProbe measures how often the pool reaches the mechanism, from
// gorge's own ledger (Engine.CardsDiscardedThisTurn): per turn and player,
// the largest tally seen.
type kdiscturnProbe struct {
	stats   *kdiscturnStats
	turn    int
	n       [2]int
	live    bool    // a carrier was in play while the tally was nonzero
	nonzero [2]bool // the player has had a nonzero turn this game
}

func (p *kdiscturnProbe) flush() {
	if p.turn == 0 {
		return
	}
	st := p.stats
	for i := 0; i < 2; i++ {
		switch {
		case p.n[i] > 0:
			st.nonzeroTurns++
			p.nonzero[i] = true
			if p.live {
				st.carrierLiveGE++
			}
			if p.n[i] >= 2 {
				st.ge2Turns++
				if p.live {
					st.carrierGE2++
				}
			}
		case p.nonzero[i]:
			st.resetTurns++
		}
		st.discards += p.n[i]
		st.maxPerTurn = max(st.maxPerTurn, p.n[i])
	}
}

func (p *kdiscturnProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if t := int(g.Turn); t != p.turn {
		p.flush()
		p.turn, p.n, p.live = t, [2]int{}, false
	}
	for i := 0; i < 2; i++ {
		p.n[i] = max(p.n[i], int(ref.E.CardsDiscardedThisTurn(state.PlayerID(i))))
	}
	if p.n[0] > 0 || p.n[1] > 0 {
		for _, pid := range []state.PlayerID{0, 1} {
			for _, id := range g.Zone(state.ZBattlefield, pid) {
				if o := g.Obj(id); o != nil && o.Face() != nil {
					if n := o.Face().Name; n == "Green Goblin, Revenant" || n == "Misty Knight, Hero for Hire" {
						p.live = true
					}
				}
			}
		}
	}
	return nil
}

// kdiscturnRun plays seeds 1..seeds with the probe, reporting divergences as
// test errors when report is set (the mutation run expects one and stays
// quiet).
func kdiscturnRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kdiscturnStats) {
	t.Helper()
	reg, set, decks := kdiscturnSetup(t)
	tbl, _, _, err := set.Matchup(kdiscturnDecks[0], kdiscturnDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kdiscturnStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdiscturn-a", "kdiscturn-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kdiscturnProbe{stats: st}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: pr.check})
		pr.flush()
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestDiscturnLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kdiscturnRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d discards, max %d per turn; %d player-turns with a nonzero tally (%d with a carrier in play), %d with >= 2 (%d with a carrier), %d later turns reading 0",
		st.games, divs, stalls, st.discards, st.maxPerTurn, st.nonzeroTurns, st.carrierLiveGE, st.ge2Turns, st.carrierGE2, st.resetTurns)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.ge2Turns < 40 {
		t.Errorf("only %d player-turns discard twice: the pool does not reach a second discard in a turn", st.ge2Turns)
	}
	if st.resetTurns < 10 {
		t.Errorf("only %d later turns read 0: the turn reset is under-exercised", st.resetTurns)
	}
}

// TestDiscturnMutation: a tally that survives the turn boundary must make
// the lockstep diverge.
func TestDiscturnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDiscardedNoTurnReset = true
	defer func() { dirac.DebugDiscardedNoTurnReset = false }()
	if divs, _, _ := kdiscturnRun(t, 64, false); divs == 0 {
		t.Fatal("DebugDiscardedNoTurnReset: no divergence in 64 seeds")
	}
}
