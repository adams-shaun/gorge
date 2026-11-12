package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Foretell lockstep (CR 702.143a): constructed matchups over the
// fixture pool dirac/cardset/pools/fixtures/foretell. The decks carry every
// arm:
//
//   - the {2} special action from the hand on the foretelling player's own
//     turn, at any priority including instant timing and with a stack
//     (gorge's "foretell" cast option, canonical cast mode 12);
//   - the later cast from exile for the foretell cost (gorge's
//     "foretell_cast", canonical CastModeOther) on a later turn, with the
//     card's own timing: an instant at instant speed, a sorcery or creature
//     only in its owner's main phase with an empty stack;
//   - no foretell cast on the foretelling turn, and none for a card exiled
//     by anything but the action.
//
// The counters come from the policy's picks, so the test fails when the
// kernel stops offering any arm; the mutations relax one rule each and must
// diverge, which proves the counters are not vacuous.
var foretellDecks = []fixtureDecks{{
	{ // blue-red: instants, sorceries and creatures that foretell, and rituals to float the mana for the actions
		{Name: "Behold the Multiverse", Count: 3}, {Name: "Ravenform", Count: 3}, {Name: "Alrund's Epiphany", Count: 1},
		{Name: "Saw It Coming", Count: 2}, {Name: "Spectral Deluge", Count: 1}, {Name: "Surge of Brilliance", Count: 1},
		{Name: "Demon Bolt", Count: 3}, {Name: "Dual Strike", Count: 2}, {Name: "Impending Flux", Count: 2},
		{Name: "Frost Fair Lure Fish", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Pyretic Ritual", Count: 4}, {Name: "Seething Song", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
	},
	{ // white-black-green: removal and a trampler that foretell
		{Name: "Iron Verdict", Count: 3}, {Name: "Warhorn Blast", Count: 3}, {Name: "Tergrid's Shadow", Count: 2},
		{Name: "Return Upon the Tide", Count: 2}, {Name: "Battle Mammoth", Count: 3}, {Name: "Dark Ritual", Count: 4},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 5},
	},
}, {
	{
		{Name: "Behold the Multiverse", Count: 4}, {Name: "Ravenform", Count: 2}, {Name: "Saw It Coming", Count: 2},
		{Name: "Surge of Brilliance", Count: 2}, {Name: "Demon Bolt", Count: 4}, {Name: "Impending Flux", Count: 2},
		{Name: "Frost Fair Lure Fish", Count: 1}, {Name: "Pyretic Ritual", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Behold the Multiverse", Count: 2}, {Name: "Ravenform", Count: 4}, {Name: "Alrund's Epiphany", Count: 2},
		{Name: "Saw It Coming", Count: 3}, {Name: "Spectral Deluge", Count: 2}, {Name: "Dual Strike", Count: 2},
		{Name: "Demon Bolt", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Seething Song", Count: 2},
		{Name: "Pyretic Ritual", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// foretellRituals float the mana the foretell actions and the foretold casts
// need: gorge offers both from the real-pool walk only (its potential planner
// prices no foretell mode), and the lockstep never taps a source at priority,
// so only mana a resolved spell left floating pays them.
var foretellRituals = map[string]bool{"Pyretic Ritual": true, "Seething Song": true, "Dark Ritual": true}

var foretellWatch = map[string]bool{
	"Behold the Multiverse": true, "Ravenform": true, "Battle Mammoth": true, "Alrund's Epiphany": true,
	"Demon Bolt": true, "Dual Strike": true, "Frost Fair Lure Fish": true, "Impending Flux": true,
	"Iron Verdict": true, "Return Upon the Tide": true, "Saw It Coming": true, "Spectral Deluge": true,
	"Surge of Brilliance": true, "Tergrid's Shadow": true, "Warhorn Blast": true,
}

// foretellTally counts the picks of each arm.
type foretellTally struct {
	games, divs, stalls int
	actions             int // foretell special actions taken
	casts               int // foretold cards cast from exile
	instantAct          int // actions taken with a spell on the stack or outside a main phase
	first               string
}

// foretellPolicy plays randomly, but takes a foretell action or a foretold
// cast the moment one is offered (three times in four), and any watched card's
// plain cast otherwise, counting the foretell picks.
func foretellPolicy(seed uint64, w *foretellTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x666f7265)) // "fore"
	foretellMode, _ := castModeCode("foretell")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if foretellRituals[kbargainName(c.G, a.Src())] && a.Mode()&15 == 0 && len(c.G.E.G.Stack) == 0 && prng.IntN(3) != 0 {
					return a // float mana for the actions
				}
				m := a.Mode() & 15
				isAction := m == foretellMode && foretellWatch[kbargainName(c.G, a.Src())]
				isCast := m == 15 && foretellWatch[kbargainName(c.G, a.Src())]
				if !isAction && !isCast || prng.IntN(4) == 0 {
					continue
				}
				if isAction {
					w.actions++
					if g := c.G.E.G; len(g.Stack) > 0 || !g.Step.IsMain() {
						w.instantAct++
					}
				} else {
					w.casts++
				}
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func runForetell(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) *foretellTally {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("foretell")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(foretellDecks))
	tbls := make([]*kop.Table, len(foretellDecks))
	for mi, md := range foretellDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		tbls[mi] = tbl
	}
	w := &foretellTally{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(foretellDecks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"foretell-a", "foretell-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, foretellPolicy(seed, w), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		w.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			w.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			w.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if w.first == "" {
				w.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || w.divs >= 3 {
				break
			}
		}
	}
	return w
}

func TestForetellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	w := runForetell(t, 200, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, %d foretell actions (%d at instant timing), %d foretold casts",
		w.games, w.divs, w.stalls, w.actions, w.instantAct, w.casts)
	if w.actions < 100 {
		t.Errorf("only %d foretell actions", w.actions)
	}
	if w.casts < 40 {
		t.Errorf("only %d foretold casts", w.casts)
	}
	if w.instantAct < 10 {
		t.Errorf("only %d foretell actions outside a main phase or over a stack", w.instantAct)
	}
}

// TestForetellMutation: relaxing the later-turn rule of the foretold cast,
// or making the action wait for sorcery timing, must each make the lockstep
// diverge (gorge offers the cast only on a later turn and the action at any
// priority of its controller's turn).
func TestForetellMutation(t *testing.T) {
	for _, m := range []struct {
		what string
		flag *bool
	}{{"a foretold card is castable on the turn it was foretold", &dirac.DebugForetellSameTurn}, {"the foretell action waits for sorcery timing", &dirac.DebugForetellSorcery}} {
		t.Run(m.what, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			w := runForetell(t, 200, false, nil)
			if w.divs == 0 {
				t.Fatalf("%s in %d games with no divergence", m.what, w.games)
			}
			t.Logf("mutation diverged in %d of %d games: %s", w.divs, w.games, w.first)
		})
	}
}
