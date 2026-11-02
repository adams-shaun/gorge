package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The scried / surveilled this-turn lockstep: Desperate
// Futurescribe (upkeep trigger condition), Proctor of Potential (graveyard
// ability condition) and Surveillance Phantasm (attacks as though it had no
// defender) read X = Count$YouScryThisTurn/Plus.Count$YouSurveilThisTurn,
// fed by scry and surveil instants (Opt, Consider), ETB scry / surveil
// creatures (Faerie Seer, Augury Owl, Dimir Informant, Proctor itself),
// against a vanilla creature deck. Gorge's answer at every decision is the
// expected one.
var kscrytallyDecks = [2][]cardset.Entry{
	{ // A: W/U
		{Name: "Desperate Futurescribe", Count: 4}, {Name: "Proctor of Potential", Count: 4}, {Name: "Surveillance Phantasm", Count: 4},
		{Name: "Opt", Count: 3}, {Name: "Consider", Count: 3}, {Name: "Faerie Seer", Count: 2}, {Name: "Augury Owl", Count: 2},
		{Name: "Dimir Informant", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 7},
	},
	{ // B: R/G vanilla creatures
		{Name: "Grizzly Bears", Count: 7}, {Name: "Hill Giant", Count: 7}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 11},
	},
}

type kscrytallyStats struct {
	games, scries, surveils int
	reads, resetTurns       int // player-turns with X >= 1 and a carrier live / a later player-turn reading 0
	maxX                    int
}

func kscrytallySetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("scrytally")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kscrytallyDecks {
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

// kscrytallyProbe measures how often the pool reaches the mechanism, from
// gorge's own ScriedThisTurn / SurveilledThisTurn: per game and turn, each
// player's X at the last sample, and whether a carrier was live for it then
// (Futurescribe or Phantasm on its battlefield, Proctor in its graveyard).
type kscrytallyProbe struct {
	stats   *kscrytallyStats
	turn    int
	sc, sv  [2]int
	live    [2]bool
	nonzero [2]bool // the player has had a nonzero turn this game
}

func (p *kscrytallyProbe) flush() {
	if p.turn == 0 {
		return
	}
	st := p.stats
	for i := range 2 {
		x := p.sc[i] + p.sv[i]
		switch {
		case x > 0:
			p.nonzero[i] = true
			if p.live[i] {
				st.reads++
			}
		case p.nonzero[i]:
			st.resetTurns++
		}
		st.scries += p.sc[i]
		st.surveils += p.sv[i]
	}
}

func kscrytallyCarrier(g *state.Game, z state.Zone, p state.PlayerID, names ...string) bool {
	for _, id := range g.Zone(z, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			for _, n := range names {
				if o.Face().Name == n {
					return true
				}
			}
		}
	}
	return false
}

func (p *kscrytallyProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if t := int(g.Turn); t != p.turn {
		p.flush()
		p.turn, p.sc, p.sv, p.live = t, [2]int{}, [2]int{}, [2]bool{}
	}
	for i := range 2 {
		pl := state.PlayerID(i)
		p.sc[i], p.sv[i] = int(ref.E.ScriedThisTurn(pl)), int(ref.E.SurveilledThisTurn(pl))
		x := p.sc[i] + p.sv[i]
		p.stats.maxX = max(p.stats.maxX, x)
		if x > 0 && (kscrytallyCarrier(g, state.ZBattlefield, pl, "Desperate Futurescribe", "Surveillance Phantasm") ||
			kscrytallyCarrier(g, state.ZGraveyard, pl, "Proctor of Potential")) {
			p.live[i] = true
		}
	}
	return nil
}

// kscrytallyRun plays seeds 1..seeds with the probe, reporting divergences
// as test errors when report is set (the mutation run expects one and stays
// quiet).
func kscrytallyRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kscrytallyStats) {
	t.Helper()
	reg, set, decks := kscrytallySetup(t)
	tbl, _, _, err := set.Matchup(kscrytallyDecks[0], kscrytallyDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kscrytallyStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kscrytally-a", "kscrytally-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kscrytallyProbe{stats: st}
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

func TestScrytallyLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kscrytallyRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d scries, %d surveils, max X %d per turn; %d player-turns reading X >= 1 with a carrier live, %d later player-turns reading 0",
		st.games, divs, stalls, st.scries, st.surveils, st.maxX, st.reads, st.resetTurns)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.reads < 30 {
		t.Errorf("%d player-turns with X >= 1 and a carrier live, want >= 30", st.reads)
	}
	if st.resetTurns < 1 {
		t.Errorf("no later turn reads 0: the turn reset is unexercised")
	}
}

// TestScrytallyMutation: a tally that survives the turn boundary must make
// the lockstep diverge.
func TestScrytallyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugScryTallyNoTurnReset = true
	defer func() { dirac.DebugScryTallyNoTurnReset = false }()
	if divs, _, _ := kscrytallyRun(t, 64, false); divs == 0 {
		t.Fatal("DebugScryTallyNoTurnReset: no divergence in 64 seeds")
	}
}
