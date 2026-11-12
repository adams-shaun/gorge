package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Rebound lockstep (CR 702.88a): a hand-cast instant or sorcery with the
// keyword is exiled as it resolves and, at its controller's next upkeep, may
// be cast from exile without paying its mana cost; that recast resolves to the
// graveyard (it does not rebound again). Both seats play the same deck over
// the generated rebound pool, so every carrier is cast by both engines. Each
// deck pairs carriers with what they need: creatures to blink and copy, cheap
// spells for Cascade to find, burn for Fevered Suspicion's opponents.

var reboundCheapDeck = []cardset.Entry{
	{Name: "Ephemerate", Count: 2}, {Name: "Invisible Force Field", Count: 2},
	{Name: "Transpose", Count: 2}, {Name: "Survival Cache", Count: 2},
	{Name: "Terramorph", Count: 2}, {Name: "Consuming Vapors", Count: 2},
	{Name: "Fantastic Elasticity", Count: 2}, {Name: "Flame On!", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
	{Name: "Lightning Bolt", Count: 2}, {Name: "Island", Count: 4},
	{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 4},
	{Name: "Mountain", Count: 1},
}

var reboundBigDeck = []cardset.Entry{
	{Name: "Quantum Misalignment", Count: 2}, {Name: "Into the Time Vortex", Count: 2},
	{Name: "Nomads' Assembly", Count: 2}, {Name: "Recurring Insight", Count: 1},
	{Name: "It's Clobberin' Time!", Count: 2}, {Name: "Fevered Suspicion", Count: 2},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
	{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
	{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 3},
	{Name: "Swamp", Count: 3}, {Name: "Forest", Count: 4},
}

const reboundMin = 3

var reboundCarriers = map[string]bool{
	"Ephemerate": true, "Invisible Force Field": true, "Transpose": true, "Survival Cache": true,
	"Terramorph": true, "Consuming Vapors": true, "Fantastic Elasticity": true, "Flame On!": true,
	"Quantum Misalignment": true, "Into the Time Vortex": true, "Nomads' Assembly": true,
	"Recurring Insight": true, "It's Clobberin' Time!": true, "Fevered Suspicion": true,
}

// reboundScan counts, in gorge's event log, each carrier's hand casts and its
// casts from exile (the recast the upkeep delayed trigger offered).
func reboundScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.PutOnStack {
			continue
		}
		ob := g.E.G.Obj(ev.Obj)
		if ob == nil || ob.Face() == nil || !reboundCarriers[ob.Face().Name] {
			continue
		}
		switch ev.From {
		case state.ZHand:
			n[ob.Face().Name+" cast"]++
		case state.ZExile:
			n[ob.Face().Name+" recast"]++
			n["recast"]++
		}
	}
}

func runRebound(t *testing.T, deck []cardset.Entry, seeds uint64) (games int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rebound")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = deck
		for _, e := range deck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"reb-a", "reb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		reboundScan(gs, branch)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func reboundSummary(branch map[string]int) string {
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	return strings.Join(bs, " ")
}

func TestReboundLockstep(t *testing.T) {
	t.Parallel()

	for name, deck := range map[string][]cardset.Entry{"cheap": reboundCheapDeck, "big": reboundBigDeck} {
		games, branch := runRebound(t, deck, 150)
		t.Logf("%s: %d games; %s", name, games, reboundSummary(branch))
		if branch["recast"] < reboundMin {
			t.Errorf("%s: %d recasts from exile, want at least %d", name, branch["recast"], reboundMin)
		}
	}
}
