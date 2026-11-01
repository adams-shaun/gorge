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

// The copy-ability lockstep (CR 707.10): Echo, Perceptive
// Prodigy (an activated or triggered ability from a creature source),
// Scientist Supreme of A.I.M. (from an artifact source) and Kirol, Attentive
// First-Year (a triggered ability) copy the abilities of creatures with
// targeted enters triggers and activated abilities (Prodigal Sorcerer, Rod of
// Ruin), choosing new targets for the copy.

var copyabDecks = [2][]cardset.Entry{
	{ // A: U/R
		{Name: "Echo, Perceptive Prodigy", Count: 3}, {Name: "Kirol, Attentive First-Year", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 2}, {Name: "Fire Imp", Count: 2}, {Name: "Flametongue Kavu", Count: 1},
		{Name: "Man-o'-War", Count: 2}, {Name: "Rod of Ruin", Count: 2}, {Name: "Prodigal Pyromancer", Count: 1},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 1}, {Name: "Strionic Resonator", Count: 2}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B: U/B/R
		{Name: "Scientist Supreme of A.I.M.", Count: 3}, {Name: "Echo, Perceptive Prodigy", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 2}, {Name: "Man-o'-War", Count: 2}, {Name: "Rod of Ruin", Count: 3},
		{Name: "Fire Imp", Count: 2}, {Name: "Flametongue Kavu", Count: 2}, {Name: "Prodigal Pyromancer", Count: 1},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 1}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
}

type copyabStats struct {
	copies, retargeted, triggered, oppWindows, asks int
}

// copyabProbe wraps a seeded random policy biased to activate the copiers and
// cast, counting the ability copies gorge puts on the stack, the copy asks
// answered with a new target, and the priority windows where only an
// opponent's abilities were on the stack.
type copyabProbe struct {
	st   *copyabStats
	base Policy
	r    *rand.Rand
	seen map[state.ObjID]bool
}

var copyabCopiers = map[string]bool{"Echo, Perceptive Prodigy": true, "Scientist Supreme of A.I.M.": true, "Kirol, Attentive First-Year": true}

func (p *copyabProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	g := c.G
	if g == nil {
		return a
	}
	gm := g.E.G
	var acts, casts []Canon
	offered := false
	for _, x := range c.D.Options {
		o, ok := g.GorgeOption(x)
		if !ok {
			continue
		}
		switch {
		case x.Kind() == ActActivate:
			if so := gm.Obj(o.Obj); so != nil && so.Face() != nil && copyabCopiers[so.Face().Name] {
				acts = append(acts, x)
				offered = true
			}
		case x.Kind() == ActCast:
			casts = append(casts, x)
		}
	}
	if c.D.Kind == DPriority {
		switch {
		case len(acts) > 0 && p.r.IntN(8) != 0:
			a = acts[p.r.IntN(len(acts))]
		case len(casts) > 0 && p.r.IntN(2) != 0:
			a = casts[p.r.IntN(len(casts))]
		}
	}
	// the ability copies on the stack, each counted once
	onlyOpp, nAb := len(gm.Stack) > 0, 0
	for _, id := range gm.Stack {
		o := gm.Obj(id)
		if o == nil || o.Ability == nil {
			onlyOpp = false
			continue
		}
		nAb++
		if int(o.Controller) == int(c.D.Player) {
			onlyOpp = false
		}
		if o.IsCopy && !p.seen[id] {
			p.seen[id] = true
			p.st.copies++
			if state.StackKindOf(gm, o) == state.StackKindTriggered {
				p.st.triggered++
			}
		}
	}
	if c.D.Kind == DPriority && onlyOpp && nAb > 0 && !offered {
		for _, id := range gm.Zone(state.ZBattlefield, 0) {
			if o := gm.Obj(id); o != nil && int(o.Controller) == int(c.D.Player) && o.Face() != nil && copyabCopiers[o.Face().Name] {
				p.st.oppWindows++
				break
			}
		}
	}
	if d := g.Pending(); d != nil && d.ResumeKind == "copy_targets" {
		p.st.asks++
		if o, ok := g.GorgeOption(a); ok && len(gm.Stack) > 0 {
			if top := gm.Obj(gm.Stack[len(gm.Stack)-1]); top != nil && top.Ability != nil && o.Index >= len(top.Targets) {
				p.st.retargeted++
			}
		}
	}
	return a
}

// copyabSetup builds the matchup's corpus decks and table.
func copyabSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copyab")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range copyabDecks {
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
	tbl, _, _, err := set.Matchup(copyabDecks[0], copyabDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// copyabRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func copyabRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st copyabStats) {
	t.Helper()
	reg, set, tbl, decks := copyabSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"copyab-a", "copyab-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &copyabProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x636f7079)), seen: map[state.ObjID]bool{}}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestCopyabLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := copyabRun(t, 256, true)
	t.Logf("256 games, copyab %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.copies, 30, "ability copies"},
		{st.retargeted, 10, "copies with a new target chosen"},
		{st.triggered, 5, "copies of a triggered ability"},
		{st.oppWindows, 5, "windows with only an opponent's ability on the stack"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestCopyabMutation: a copy that ignores MayChooseTarget$ and keeps the
// original's targets must make the lockstep diverge.
func TestCopyabMutation(t *testing.T) {
	dirac.DebugCopyAbNoRetarget = true
	defer func() { dirac.DebugCopyAbNoRetarget = false }()
	if diverged, _, _ := copyabRun(t, 64, false); !diverged {
		t.Fatal("DebugCopyAbNoRetarget: no divergence in 64 seeds")
	}
}
