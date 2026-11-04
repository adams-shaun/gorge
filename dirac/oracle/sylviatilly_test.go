package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Sylvia Tilly lockstep: the beginning-of-combat trigger's own "{1} and
// exile a card from your graveyard" cost. gorge offers the pay (the pool alone
// covers {1}, a card is in the graveyard) or decline election, poses the
// exile_cost pick over the graveyard when it holds more than one card, then
// charges the mana; the kernel's chained PayMana / PayExileGrave must pose the
// same asks in the same order, and its reflexive counter trigger must follow
// only a paid cost.

var ksylviaDecks = [][2][]cardset.Entry{{
	{
		{Name: "Sylvia Tilly, Joy of Discovery", Count: 8}, {Name: "Ambush Viper", Count: 6}, {Name: "Sol Ring", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 18},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// sylviaPolicy casts whenever it can, except that it holds Dark Ritual for the
// beginning of combat, with a trigger waiting on the stack: gorge pays a trigger
// cost that has an exile part from the floating pool alone (no mana window
// opens for it), and the canonical surface offers no bare mana-ability taps, so
// the ritual's mana in the pool is what makes the cost payable. picks counts
// the decisions that pose an exile_cost pick.
func sylviaPolicy(seed uint64, picks *int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		ritual := func(a Canon) bool {
			if a.Kind() != ActCast || c.G == nil {
				return false
			}
			o, ok := c.G.GorgeOption(a)
			ob := c.G.E.G.Obj(o.Obj)
			return ok && ob != nil && ob.Face() != nil && ob.Face().Name == "Ambush Viper"
		}
		if c.G != nil {
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && string(o.Kind) == "exile_cost" {
					*picks++
					break
				}
			}
		}
		pick := func(keep func(Canon) bool) (a Canon, ok bool) {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if keep(a) {
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 {
				return a, false
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq}), true
		}
		window := false
		if c.G != nil {
			g := c.G.E.G
			window = g.Step == state.StepBeginCombat && len(g.Stack) > 0
		}
		if a, ok := pick(func(a Canon) bool { return a.Kind() == ActCast && ritual(a) == window }); ok {
			return a
		}
		if a, ok := pick(func(a Canon) bool { return a.Kind() != ActActivate && a.Kind() != ActCast }); ok {
			return a
		}
		return rnd.Pick(c)
	})
}

// runSylvia plays the matchups for seeds 1..seeds and returns the games,
// divergences, stalls and the decisions in which gorge posed an exile_cost
// pick over several cards.
func runSylvia(t *testing.T, seeds int, mutate bool) (games, divs, stalls, picks int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sylviatilly")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugSylviaNoExile
	dirac.DebugSylviaNoExile = mutate
	defer func() { dirac.DebugSylviaNoExile = old }()
	for mi, md := range ksylviaDecks {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksylvia-a", "ksylvia-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := sylviaPolicy(seed, &picks)
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return
}

func TestSylviaTillyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, picks := runSylvia(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d exile_cost picks", games, divs, stalls, picks)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if picks < 10 {
		t.Errorf("only %d exile_cost picks over several graveyard cards (want at least 10)", picks)
	}
}

// TestSylviaTillyMutation drops the graveyard exile: the games must diverge. A
// green mutation (no divergence) means the lockstep never pays the cost.
func TestSylviaTillyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runSylvia(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every exile in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
