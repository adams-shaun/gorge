package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Eaten Alive's {3}{B} part is paid through gorge's manual cast window. Its
// colour ask names no owed colour, so a gain land makes its lowest colour unless
// the clamp keeps only the colour still owed; spent that way the window can
// strand the cost, and the cast reverses (CR 733.1) keeping the taps. Giada's
// mana can't pay this spell, so her window census is complete and an unpayable
// part reverses the cast at the target ask (pairs 198, 778). A pool already paying
// the printed cost taps no witness, but the {3}{B} part still goes through the
// window, in battlefield order.
func TestEatenAliveWindowLockstep(t *testing.T) {
	t.Parallel()
	c, pool := testPoolCorpus(t, "fdn")
	set, err := cardset.RawByName("fdn")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct{ pair, mulligans int }{{72, 0}, {370, 0}, {1294, 0}, {198, 1}, {778, 1}, {1056, 1}} {
		p := PairingAt(pool, g.pair, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, g.mulligans)
		if err != nil {
			t.Fatal(err)
		}
		var cfg Config
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("pair %d: diverged at %d: %s %s %s", g.pair, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		}
	}
}

// A Giada beside Eaten Alive. Her Angel-only {W} goes through the window's clamp
// like any fixed colour, so she is withheld once only {B} is owed (seed 1003).
// Tapped (1083) or summoning sick (1001, 1010) she changes nothing at the target
// ask. Floating, her {W} settles an Angel from the pool (1059).
func TestGiadaEatenAliveLockstep(t *testing.T) {
	t.Parallel()
	c, _ := testPoolCorpus(t, "fdn")
	set, err := cardset.ByName("fdn")
	if err != nil {
		t.Fatal(err)
	}
	a := []cardset.Entry{{Name: "Giada, Font of Hope", Count: 4}, {Name: "Eaten Alive", Count: 6}, {Name: "Dazzling Angel", Count: 2},
		{Name: "Helpful Hunter", Count: 4}, {Name: "Healer's Hawk", Count: 3}, {Name: "Burglar Rat", Count: 3},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 8}}
	b := []cardset.Entry{{Name: "Giada, Font of Hope", Count: 3}, {Name: "Eaten Alive", Count: 3}, {Name: "Felidar Savior", Count: 3},
		{Name: "Serra Angel", Count: 2}, {Name: "Inspiring Paladin", Count: 3}, {Name: "Cathar Commando", Count: 3},
		{Name: "Soul-Shackled Zombie", Count: 3}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 8}}
	deck := func(es []cardset.Entry) []*cards.Card {
		var out []*cards.Card
		for _, e := range es {
			cd, ok := c.Reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				out = append(out, cd)
			}
		}
		return out
	}
	tbl, _, _, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct {
		seed uint64
		swap bool
	}{{1003, false}, {1083, false}, {1001, false}, {1010, true}, {1059, false}} {
		decks := [2][]*cards.Card{deck(a), deck(b)}
		if g.swap {
			decks[0], decks[1] = decks[1], decks[0]
		}
		m, err := NewMatchup(c.Reg, [2]string{"giada-a", "giada-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		var cfg Config
		k, err := SetSide(set, tbl, cfg)(m, g.seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, g.seed, k, RandomPolicy(g.seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at %d: %s %s %s", g.seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		}
	}
}

// An either-or part beside a restricted source that may pay this spell: The Emperor
// of Palamecia's mana spends on a noncreature spell, and gorge's window prices it
// like any other source (CR 106.6), so the target menu is judged on what the pool,
// she and the lands can reach together. A part they cannot reach reverses the cast
// at the target ask, never in the window; one they can reach (a floating mana beside
// her and a land) goes on to the target (CR 601.2g, 733.1).
func TestEitherOrPartBesideAdmittedSourceLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fin")
	entries := []cardset.Entry{{Name: "The Emperor of Palamecia", Count: 10}, {Name: "Louisoix's Sacrifice", Count: 12},
		{Name: "Elixir", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6}}
	var deck []*cards.Card
	for _, e := range entries {
		cd, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, cd)
		}
	}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatal(err)
	}
	var atAsk, inWindow, kept int
	for seed := uint64(1); seed <= 200; seed++ {
		m, err := NewMatchup(reg, [2]string{"emperor-a", "emperor-b"}, [2][]*cards.Card{deck, deck}, 0)
		if err != nil {
			t.Fatal(err)
		}
		var cfg Config
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := eitherOrProbe{base: RandomPolicy(seed), atAsk: &atAsk, inWindow: &inWindow, kept: &kept}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at %d: %s %s %s %v %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	t.Logf("reversed at the target ask %d, in the window %d, went on to the target %d", atAsk, inWindow, kept)
	if atAsk < 3 || kept < 3 || inWindow != 0 {
		t.Errorf("reversed at the target ask %d, in the window %d, went on %d: want 3 and 3, none in the window", atAsk, inWindow, kept)
	}
}

// eitherOrProbe answers an either-or ask with its last option, an unpayable part
// when there is one, and tallies where that cast reversed or went on to its target.
type eitherOrProbe struct {
	base            Policy
	src             state.ObjID
	targeted        bool
	onStack         bool
	atAsk, inWindow *int
	kept            *int
}

func (p *eitherOrProbe) pick(c *PickCtx) Canon {
	if p.src != 0 {
		if c.G.E.G.Obj(p.src).Zone == state.ZHand {
			if p.targeted {
				*p.inWindow++
			} else {
				*p.atAsk++
			}
			p.src = 0
		} else if c.G.E.G.Obj(p.src).Zone != state.ZStack {
			p.src = 0
		} else if !p.onStack {
			p.onStack = true
			*p.kept++
		}
	}
	d := c.G.E.Pending()
	if d != nil && len(d.Options) > 1 && d.Options[0].Kind == "altaddcost" {
		p.src, p.targeted, p.onStack = d.Source, false, false
		return c.D.Options[len(c.D.Options)-1]
	}
	if p.src != 0 && d != nil && d.Kind == decision.KTarget {
		p.targeted = true
	}
	return p.base.Pick(c)
}
