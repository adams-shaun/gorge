package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (Forge 91402ac5110):
// `Mandatory$ True` on ChooseCard lifts Fireglass Mentor, Magmatic Channeler
// and You Find Some Prisoners. Each exiles the top cards, makes the player
// choose one of them and lets them play it.
var mandatoryDeck = []cardset.Entry{
	{Name: "Fireglass Mentor", Count: 8}, {Name: "Magmatic Channeler", Count: 6}, {Name: "You Find Some Prisoners", Count: 6},
	{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4},
	{Name: "Memnite", Count: 3}, {Name: "Ornithopter", Count: 3},
	{Name: "Mountain", Count: 14}, {Name: "Swamp", Count: 10},
}

var mandatoryCarriers = []string{"Fireglass Mentor", "Magmatic Channeler", "You Find Some Prisoners"}

// mandatoryCounter counts, per carrier, the ChooseCard decisions gorge poses
// over exiled cards with the carrier's ability (or spell) on top of the
// stack: one per resolution of its ChooseCard.
type mandatoryCounter struct {
	n     [3]int
	asked map[uint64]bool
}

func (c *mandatoryCounter) reset() { c.asked = map[uint64]bool{} }

func (c *mandatoryCounter) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if ref.fr == nil || ref.fr.d == nil || ref.fr.d.Kind != decision.KChoose || len(g.Stack) == 0 {
		return nil
	}
	// the options are the exiled cards (a discard cost's choice is a hand pick)
	for _, o := range ref.fr.d.Options {
		if x := g.Obj(o.Obj); x == nil || x.Zone != state.ZExile {
			return nil
		}
	}
	top := g.Stack[len(g.Stack)-1]
	if c.asked[uint64(top)] {
		return nil
	}
	c.asked[uint64(top)] = true
	name := sourceCardName(g, top)
	for i, n := range mandatoryCarriers {
		if name == n {
			c.n[i]++
		}
	}
	return nil
}

func TestMandatoryLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mandatory")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range mandatoryDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 60 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	decks := [2][]*cards.Card{deck, deck}
	tbl, _, _, err := set.Matchup(mandatoryDeck, mandatoryDeck)
	if err != nil {
		t.Fatal(err)
	}
	var cnt mandatoryCounter
	stalls, divs := 0, 0
	for seed := uint64(1); seed <= 192; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mandatory-a", "mandatory-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := Policy(RandomPolicy(seed))
		if seed%2 == 1 {
			pol = HeuristicPolicy(seed)
		}
		cnt.reset()
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Check: cnt.check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("192 seeds, %d divergences, %d stalls; ChooseCard resolutions %v %v", divs, stalls, mandatoryCarriers, cnt.n)
	if divs > 0 || stalls > 0 {
		return
	}
	for i, n := range cnt.n {
		if n < 10 {
			t.Errorf("%s: %d ChooseCard resolutions, want at least 10", mandatoryCarriers[i], n)
		}
	}
}
