package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The game-over lockstep (CR 104.2b / 104.3e / 700.2): Demonic
// Pact ("choose one that hasn't been chosen", ChoiceRestriction$ ThisGame on
// an upkeep trigger; the fourth mode is DB$ LosesGame) against Herald of
// Eternal Dawn (can't lose / opponents can't win), with Maze's End (DB$
// WinsGame behind ten different Gates) and Gates, cheap creatures and burn.
// The expected answer at every decision is gorge's.
var kgameoverDecks = [2][]cardset.Entry{
	{ // A: the Pact deck
		{Name: "Demonic Pact", Count: 3}, {Name: "Herald of Eternal Dawn", Count: 2}, {Name: "Maze's End", Count: 1},
		{Name: "Orzhov Guildgate", Count: 2}, {Name: "Azorius Guildgate", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 7},
		{Name: "Thraben Inspector", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 2},
		{Name: "Isamaru, Hound of Konda", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Pacifism", Count: 2},
	},
	{ // B
		{Name: "Herald of Eternal Dawn", Count: 3}, {Name: "Maze's End", Count: 1}, {Name: "Boros Guildgate", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Thraben Inspector", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Unsummon", Count: 1}, {Name: "Swords to Plowshares", Count: 2},
	},
}

type kgameoverStats struct {
	pacts    int // Demonic Pact modes picked (gorge's own log)
	allThree int // Pacts that logged DeathPact fourth, after the three other modes
	death    int // Pacts whose DeathPact pick ended in its controller's loss
	herald   int // Pacts whose DeathPact pick left its controller alive (Herald on board)
}

// kgameoverProbe plays cast-heavy so the Pact and Herald are reached, and
// keeps the last gorge side to read its final state.
type kgameoverProbe struct {
	base  Policy
	r     *rand.Rand
	gs    *GorgeSide
	pacts map[state.ObjID]bool // the permanents of Demonic Pact triggers asked so far
	snap  map[state.ObjID]pactSnap
	seq   uint64
}

// pactSnap is a Demonic Pact permanent's pick log and controller as last seen
// (a loser's permanents leave the game with it, CR 800.4a).
type pactSnap struct {
	modes []state.ModeChoice
	ctrl  state.PlayerID
}

func (p *kgameoverProbe) pick(c *PickCtx) Canon {
	if c.G != nil {
		p.gs = c.G
		g := c.G.E.G
		if c.G.fr != nil && c.G.fr.d != nil && string(c.G.fr.d.Kind) == "modes" && c.G.fr.d.Seq != p.seq {
			p.seq = c.G.fr.d.Seq
			if o := g.Obj(c.G.fr.d.Source); o != nil && o.Face() != nil && o.Face().Name == "Demonic Pact" {
				if o.Source != 0 {
					o = g.Obj(o.Source)
				}
				if o != nil {
					p.pacts[o.ID] = true
				}
			}
		}
		for id := range p.pacts {
			if o := g.Obj(id); o != nil {
				p.snap[id] = pactSnap{append([]state.ModeChoice(nil), o.ModeChoices...), o.Controller}
			}
		}
	}
	a := p.base.Pick(c)
	if c.D.Kind == DPriority && p.r.IntN(4) != 0 {
		var acts []Canon
		for _, x := range c.D.Options {
			if k := x.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				acts = append(acts, x)
			}
		}
		if len(acts) > 0 {
			a = acts[p.r.IntN(len(acts))]
		}
	}
	return a
}

func kgameoverRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st kgameoverStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gameover")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kgameoverDecks {
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
	tbl, _, _, err := set.Matchup(kgameoverDecks[0], kgameoverDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kgameover-a", "kgameover-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kgameoverProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x67616d65)), pacts: map[state.ObjID]bool{}, snap: map[state.ObjID]pactSnap{}}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
		if probe.gs == nil {
			continue
		}
		g := probe.gs.E.G
		for _, o := range probe.snap {
			n := len(o.modes)
			st.pacts += n
			dead := -1
			for i, mc := range o.modes {
				if mc.Mode == "DeathPact" {
					dead = i
				}
			}
			switch {
			case dead < 0:
			case g.Players[o.ctrl].Lost:
				st.death++
				if dead == 3 {
					st.allThree++
				}
			default:
				hb := false
				for _, id := range g.Zone(state.ZBattlefield, o.ctrl) {
					if h := g.Obj(id); h != nil && h.Face() != nil && h.Face().Name == "Herald of Eternal Dawn" {
						hb = true
					}
				}
				if hb {
					st.herald++
				}
				if dead == 3 {
					st.allThree++
				}
			}
		}
	}
	return
}

func TestGameoverLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := kgameoverRun(t, 400, true)
	t.Logf("400 games, game-over %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.pacts, 20, "Demonic Pact triggers"},
		{st.allThree, 1, "Pacts with all three other modes before DeathPact"},
		{st.death, 5, "games ended on the DeathPact loss"},
		{st.herald, 3, "Pacts whose DeathPact pick did not end the game (Herald)"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestGameoverMutation: a ThisGame pick recorded as a ThisTurn one is
// forgotten at the turn change, so the lockstep must diverge.
func TestGameoverMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCharmPickNoGameScope = true
	defer func() { dirac.DebugCharmPickNoGameScope = false }()
	if diverged, _, _ := kgameoverRun(t, 128, false); !diverged {
		t.Fatal("DebugCharmPickNoGameScope: no divergence in 128 seeds")
	}
}
