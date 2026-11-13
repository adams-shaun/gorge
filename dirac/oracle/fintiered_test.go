package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The FIN Tiered lockstep (CR 702.183a-b, 601.2b, 601.2f):
// Tiered is Spree's one-mode sibling. A Tiered card prints no CharmNum$ or
// MinCharmNum$, so exactly one mode is chosen; the chosen mode's ModeCost$
// is an additional cost, and a mode whose own cost cannot be paid is
// withheld at the ask. The tier tally comes from gorge's events.
var kfintieredDecks = [2][]cardset.Entry{
	{
		{Name: "Fire Magic", Count: 3}, {Name: "Thunder Magic", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 12},
		{Name: "Forest", Count: 12},
	},
	{
		{Name: "Restoration Magic", Count: 3}, {Name: "Vincent's Limit Break", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
	},
}

// kfintieredModes lists each carrier's three modes by a distinctive part of
// their SpellDescription (gorge's ModeChosen Text), in printed order: tier 1
// is the {0} mode, tier 3 the dearest.
var kfintieredModes = map[string][3]string{
	"Fire Magic":            {"deals 1 damage to each creature", "deals 2 damage to each creature", "deals 3 damage to each creature"},
	"Thunder Magic":         {"deals 2 damage to target creature", "deals 4 damage to target creature", "deals 8 damage to target creature"},
	"Restoration Magic":     {"Target permanent gains", "You gain 3 life", "Permanents you control gain"},
	"Vincent's Limit Break": {"3/2.", "5/2.", "7/2."},
}

type kfintieredTally struct {
	evAt     int
	cur      state.ObjID
	tier     int // 0..2, -1 when the label is not a tier
	seen     [3]int
	resolved [3]int
	reversed int
}

func (p *kfintieredTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.ModeChosen:
			ms, ok := kfintieredModes[chName(g, e.Obj)]
			if !ok || e.Text == "" {
				continue
			}
			p.cur, p.tier = e.Obj, -1
			for i, s := range ms {
				if strings.Contains(e.Text, s) {
					p.tier = i
					break
				}
			}
			if p.tier >= 0 {
				p.seen[p.tier]++
			}
		case events.SpellCastReversed:
			if p.cur != 0 {
				p.reversed++
			}
			p.cur, p.tier = 0, -1
		case events.Resolve:
			if p.cur != 0 && e.Obj == p.cur {
				if p.tier >= 0 {
					p.resolved[p.tier]++
				}
				p.cur, p.tier = 0, -1
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// kfintieredPolicy plays a land, else casts, and answers a mode ask with the
// seed's rotating slot among the offered modes, so every affordable tier is
// chosen across the sweep; the rest is random.
func kfintieredPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			for _, k := range [...]CanonKind{ActPlayLand, ActCast} {
				for _, a := range c.D.Options {
					if a.Kind() == k {
						return a
					}
				}
			}
		case DModes:
			var modes []Canon
			for _, a := range c.D.Options {
				if a.Kind() == ActMode {
					modes = append(modes, a)
				}
			}
			if len(modes) > 0 {
				return modes[int(seed)%len(modes)]
			}
		}
		return rnd.Pick(c)
	})
}

type kfintieredRunResult struct {
	games, divergences, stalls int
	seen, resolved             [3]int
	reversed                   int
	firstDiv                   string
}

func kfintieredRun(t *testing.T, seeds int) kfintieredRunResult {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fintiered")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfintieredDecks {
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
	tbl, _, _, err := set.Matchup(kfintieredDecks[0], kfintieredDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var out kfintieredRunResult
	var p kfintieredTally
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fintiered-a", "fintiered-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		p.evAt, p.cur, p.tier = 0, 0, -1
		p.seen, p.resolved, p.reversed = [3]int{}, [3]int{}, 0
		// MaxTurns 100: these are 40-card creature decks with no reach, so
		// most games go past a 20-turn sketch before either deck
		// ends (a 20-turn cap stalled 230 of 256 seeds). 100 covers decking
		// (~66 player turns) and keeps the 0-stall assertion meaningful.
		res := Lockstep(m, seed, k, kfintieredPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 100, Check: p.check})
		CloseSide(k)
		out.games++
		for i := range out.seen {
			out.seen[i] += p.seen[i]
			out.resolved[i] += p.resolved[i]
		}
		out.reversed += p.reversed
		if res.Stall != "" {
			out.stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			out.divergences++
			if out.firstDiv == "" {
				lo := max(d.At-12, 0)
				out.firstDiv = fmt.Sprintf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v",
					seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions,
					actionStrings(res.Actions[lo:d.At]))
			}
		}
	}
	return out
}

func TestFintieredLockstep(t *testing.T) {
	t.Parallel()
	st := kfintieredRun(t, 256)
	t.Logf("%d games: modes announced %v; resolved casts per tier %v; reversed at payment %d; %d divergences, %d stalls",
		st.games, st.seen, st.resolved, st.reversed, st.divergences, st.stalls)
	if st.divergences != 0 {
		t.Errorf("%d divergences; first %s", st.divergences, st.firstDiv)
	}
	for i := 0; i < 3; i++ {
		if st.resolved[i] < 20 {
			t.Errorf("tier %d: %d resolved casts, want at least 20", i+1, st.resolved[i])
		}
	}
}

// TestFintieredMutation: without the chosen mode's ModeCost$ the Tiered
// lockstep must diverge.
func TestFintieredMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTieredNoModeCost = true
	defer func() { dirac.DebugTieredNoModeCost = false }()
	if st := kfintieredRun(t, 64); st.divergences == 0 {
		t.Fatal("DebugTieredNoModeCost: no divergence in 64 seeds")
	}
}
