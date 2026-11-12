package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The per-permanent mark lockstep: constructed W/B and G/U/R
// decks over the generated permmark pool, so attackedThisTurn (The Lunar
// Whale, Hexhaven Dueling Arena), dealtDamageThisTurn (Treacherous Greed's
// sacrifice cost, Red Guardian's ETB target) and the game-long dealtDamagetoAny
// / dealtCombatDamagetoAny (Karakyk Guardian's and Ruric Thar's hexproof) are
// compared with gorge through combat, burn, fight and bite damage.

var permMarkDecks = [2][]cardset.Entry{
	{ // A: W/B
		{Name: "Treacherous Greed", Count: 4}, {Name: "Red Guardian, Super-Soldier", Count: 4}, {Name: "Hexhaven Dueling Arena", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Walking Corpse", Count: 3},
		{Name: "Serra Angel", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 8},
	},
	{ // B: G/U/R
		{Name: "The Lunar Whale", Count: 2}, {Name: "Ruric Thar, Magecrusher", Count: 2}, {Name: "Karakyk Guardian", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Runeclaw Bear", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 2}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Prey Upon", Count: 2}, {Name: "Bite Down", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 5},
	},
}

type permMarkStats struct {
	greed, guardian, hexproofed int
}

// permMarkProbe wraps a seeded random policy biased to cast and tallies, from
// gorge's state between picks, the situations the marks decide.
type permMarkProbe struct {
	st    *permMarkStats
	base  Policy
	r     *rand.Rand
	onBF  map[state.ObjID]bool // objects already seen on the battlefield
	onStk map[state.ObjID]bool // Treacherous Greed objects already seen on the stack
}

func (p *permMarkProbe) observe(g *state.Game) {
	if p.onBF == nil {
		p.onBF, p.onStk = map[state.ObjID]bool{}, map[state.ObjID]bool{}
	}
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Face() == nil {
			continue
		}
		name := o.Face().Name
		if name == "Treacherous Greed" && o.Zone == state.ZStack && !p.onStk[o.ID] {
			p.onStk[o.ID] = true
			p.st.greed++
		}
		if o.Zone != state.ZBattlefield {
			continue
		}
		switch name {
		case "Red Guardian, Super-Soldier":
			if !p.onBF[o.ID] {
				p.onBF[o.ID] = true
				for j := range g.Objs {
					q := &g.Objs[j]
					if q.Zone == state.ZBattlefield && q.Controller != o.Controller && len(q.DamageDealtThisTurn) > 0 {
						p.st.guardian++
						break
					}
				}
			}
		case "Karakyk Guardian":
			if o.DealtDamageToAnyGame {
				p.st.hexproofed++
			}
		case "Ruric Thar, Magecrusher":
			if o.DealtCombatDamageToAnyGame {
				p.st.hexproofed++
			}
		}
	}
}

// damagedOpp reports whether a creature pl's opponent controls has dealt
// damage this turn.
func damagedOpp(g *state.Game, pl state.PlayerID) bool {
	for i := range g.Objs {
		if q := &g.Objs[i]; q.Zone == state.ZBattlefield && q.Controller != pl && len(q.DamageDealtThisTurn) > 0 {
			return true
		}
	}
	return false
}

func (p *permMarkProbe) pick(c *PickCtx) Canon {
	if g := c.G; g != nil {
		p.observe(g.E.G)
	}
	a := p.base.Pick(c)
	var casts []Canon
	for _, x := range c.D.Options {
		if x.Kind() == ActCast {
			casts = append(casts, x)
		}
	}
	if len(casts) > 0 && p.r.IntN(4) != 0 {
		a = casts[p.r.IntN(len(casts))]
	}
	// Red Guardian (flash) is cast whenever an opposing creature has dealt
	// damage this turn, so its ETB finds a target; while one is in hand and
	// none has, the player holds its mana (no other spell)
	if g := c.G; g != nil && kbargainHellkiteZone(g, state.ZHand, c.D.Player)["Red Guardian, Super-Soldier"] > 0 {
		if damagedOpp(g.E.G, state.PlayerID(c.D.Player)) {
			for _, x := range casts {
				if refCardName(g, x.Src()) == "Red Guardian, Super-Soldier" {
					return x
				}
			}
		} else if a.Kind() == ActCast {
			var rest []Canon
			for _, x := range c.D.Options {
				if x.Kind() != ActCast {
					rest = append(rest, x)
				}
			}
			if len(rest) > 0 {
				return rest[p.r.IntN(len(rest))]
			}
		}
	}
	return a
}

func permMarkSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("permmark")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range permMarkDecks {
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
	tbl, _, _, err := set.Matchup(permMarkDecks[0], permMarkDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// permMarkRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies.
func permMarkRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st permMarkStats) {
	t.Helper()
	reg, set, tbl, decks := permMarkSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"permmark-a", "permmark-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &permMarkProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x706d61726b))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
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
	}
	return
}

func TestPermmarkLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := permMarkRun(t, 256, true)
	t.Logf("256 games, perm marks %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.greed, 20, "Treacherous Greed casts"},
		{st.guardian, 15, "Red Guardian entries with a damaged opposing creature"},
		{st.hexproofed, 20, "decisions with Karakyk or Ruric on the battlefield after it dealt damage"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestPermmarkMutation: keeping the per-turn marks across turns must make
// the lockstep diverge.
func TestPermmarkMutation(t *testing.T) {
	dirac.DebugPermMarkNoReset = true
	defer func() { dirac.DebugPermMarkNoReset = false }()
	if diverged, _, _ := permMarkRun(t, 64, false); !diverged {
		t.Fatal("DebugPermMarkNoReset: no divergence in 64 seeds")
	}
}
