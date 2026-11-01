package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveyard->battlefield hidden-pick lockstep: Scavenger's
// Talent's level 3 returns a creature card from its controller's graveyard
// with a FINALITY counter (gorge effHiddenPick's WithCountersType$, the
// kernel's ChooseMove B bits 4..11 armed by handGraveArm). Its sacrifice cost
// feeds the Talent's own mill and Food triggers; Bake into a Pie, Deadly Brew
// and the opponent's burn fill the graveyards. Gorge's answer at every
// decision is the expected one.
var kgybfDecks = [2][]cardset.Entry{
	{ // A: W/B, the Talents and Victor: cheap Auras drive Victor's Eerie to its third resolution
		{Name: "Victor, Valgavoth's Seneschal", Count: 4}, {Name: "Scavenger's Talent", Count: 2},
		{Name: "Holy Strength", Count: 4}, {Name: "Unholy Strength", Count: 4}, {Name: "Dead Weight", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8},
	},
	{ // B: R/G creatures and burn, whose creatures die into its graveyard
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Mountain", Count: 17}, {Name: "Forest", Count: 6},
	},
}

type kgybfStats struct {
	games, finality, foreign int
}

func kgybfSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gybf")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kgybfDecks {
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

// kgybfProbe counts, from gorge's battlefield, each object that is seen
// carrying a FINALITY counter (a Talent return; nothing else in the pool
// puts one on), and every battlefield object controlled by a player other
// than its owner (nothing in the pool steals, so such an
// object is an opponent's card Victor returned from a graveyard).
type kgybfProbe struct {
	stats *kgybfStats
	seen  map[state.ObjID]bool
}

func (p *kgybfProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			o := g.Obj(id)
			if o == nil || p.seen[id] {
				continue
			}
			if o.Owner != o.Controller {
				p.seen[id] = true
				p.stats.foreign++
			}
			if o.Counter("FINALITY") != 0 {
				p.seen[id] = true
				p.stats.finality++
			}
		}
	}
	return nil
}

func kgybfRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kgybfStats) {
	t.Helper()
	reg, set, decks := kgybfSetup(t)
	tbl, _, _, err := set.Matchup(kgybfDecks[0], kgybfDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kgybfStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kgybf-a", "kgybf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &kgybfProbe{stats: st, seen: map[state.ObjID]bool{}}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: pr.check})
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

func TestGybfLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kgybfRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d FINALITY returns, %d returns of an opponent's card",
		st.games, divs, stalls, st.finality, st.foreign)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.finality < 10 {
		t.Errorf("%d FINALITY returns, want >= 10", st.finality)
	}
	if st.foreign < 5 {
		t.Errorf("%d returns of an opponent's card under the controller, want >= 5", st.foreign)
	}
}

// TestGybfMutation: each switch must make the lockstep diverge within 64
// seeds. DebugGyBfSacFirst queues a sacrifice's Sacrificed triggers ahead of
// the same departure's dies triggers (the kernel's event-by-event order, not
// gorge's one face walk); DebugGyBfNoCounter returns Scavenger's Talent's
// pick without its FINALITY counter; DebugGyBfOwnerEntry enters a
// GainControl$ pick under the card's owner, not the chooser.
func TestGybfMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name string
		on   *bool
	}{{"DebugGyBfSacFirst", &dirac.DebugGyBfSacFirst}, {"DebugGyBfNoCounter", &dirac.DebugGyBfNoCounter}, {"DebugGyBfOwnerEntry", &dirac.DebugGyBfOwnerEntry}} {
		*m.on = true
		divs, _, _ := kgybfRun(t, 64, false)
		*m.on = false
		if divs == 0 {
			t.Errorf("%s: no divergence in 64 seeds", m.name)
		}
	}
}
