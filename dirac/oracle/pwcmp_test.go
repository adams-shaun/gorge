package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The power-compare lockstep: Technodrome's "can't attack or
// block unless its power is 6 or greater" (a literal powerLT6), and Formation
// Breaker's / Prehistoric Pet's CantBlockBy (ValidBlocker$ powerLTX / powerGTX
// with X = Count$CardPower: gorge resolves X against the static's host, so
// the host's CURRENT power) beside vanilla bodies of power 1-6, Ornithopters
// for Technodrome to sacrifice and Dragonscale Boon to move powers. The
// expected answer at every decision is gorge's.
var kpwcmpDecks = [2][]cardset.Entry{
	{
		{Name: "Formation Breaker", Count: 4}, {Name: "Technodrome", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Craw Wurm", Count: 2},
		{Name: "Dragonscale Boon", Count: 3}, {Name: "Forest", Count: 11}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Prehistoric Pet", Count: 4}, {Name: "Formation Breaker", Count: 2}, {Name: "Technodrome", Count: 2},
		{Name: "Elite Vanguard", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Giant Spider", Count: 3},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Dragonscale Boon", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 3},
	},
}

// kpwcmpTally is what the gorge side saw over the run.
type kpwcmpTally struct {
	windows int // declare-blockers windows: a Breaker / Pet attacks with an untapped defender creature on each side of the compare
	techno  int // turns with Technodrome on the battlefield below power 6
	lastWin int32
	lastTec int32
}

func (c *kpwcmpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if g.Turn < c.lastWin || g.Turn < c.lastTec { // a fresh game
		c.lastWin, c.lastTec = 0, 0
	}
	if g.Step == state.StepDeclareBlockers && g.Turn != c.lastWin {
		def := g.Active ^ 1
		for _, id := range g.Zone(state.ZBattlefield, g.Active) {
			o := g.Obj(id)
			if o == nil || !o.IsAttacking || o.Face() == nil || (o.Face().Name != "Formation Breaker" && o.Face().Name != "Prehistoric Pet") {
				continue
			}
			hp := ref.E.Derived(id).Power
			below, level := false, false
			for _, bid := range g.Zone(state.ZBattlefield, def) {
				b := g.Obj(bid)
				if b == nil || b.Tapped || b.Face() == nil || !b.Face().IsCreature() {
					continue
				}
				if bp := ref.E.Derived(bid).Power; bp < hp {
					below = true
				} else {
					level = true
				}
			}
			if below && level {
				c.windows++
				c.lastWin = g.Turn
				break
			}
		}
	}
	if g.Turn != c.lastTec {
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Technodrome" && ref.E.Derived(id).Power < 6 {
					c.techno++
					c.lastTec = g.Turn
				}
			}
		}
	}
	return nil
}

func kpwcmpRun(t *testing.T, n uint64, report bool) (div, stalls int, c *kpwcmpTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pwcmp")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kpwcmpDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kpwcmpDecks[0], kpwcmpDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	c = &kpwcmpTally{}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"pw-a", "pw-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: c.check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, stalls, c
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, stalls, c
}

// TestPwcmpLockstep: 300 games, 0 divergences, with real blocker windows
// on both sides of the power compare and Technodrome below power 6.
func TestPwcmpLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, c := kpwcmpRun(t, 300, true)
	t.Logf("300 games: %d divergences, %d kernel stalls, %d declare-blockers windows, %d Technodrome turns below power 6", div, stalls, c.windows, c.techno)
	if c.windows < 30 {
		t.Errorf("%d declare-blockers windows; want at least 30", c.windows)
	}
	if c.techno < 10 {
		t.Errorf("%d Technodrome turns below power 6; want at least 10", c.techno)
	}
}

// TestPwcmpMutation: with the blocker-below-host compare read as <=, some
// game diverges, so the pool puts a blocker level with the host's power.
func TestPwcmpMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPwVsSrcLE = true
	defer func() { dirac.DebugPwVsSrcLE = false }()
	if div, _, _ := kpwcmpRun(t, 128, false); div > 0 {
		return
	}
	t.Fatal("no seed diverged with powerLTX read as <=: the pool does not put a blocker level with the host's power")
}
