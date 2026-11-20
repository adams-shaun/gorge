package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The eternalize riders the gap-4 wave left refused: Sinuous Striker's and
// Sunscourge Champion's printed eternalize cost carries an extra
// Discard<1/Card> beside the self-exile, and Champion of Wits' enter trigger
// pays a Draw<X/You> cost (X its power) before discarding two. The pod casts
// the carriers, Wrath of God puts them in the graveyard, and the eternalize
// ability brings them back as tokens: the lockstep compares the two sides at
// every decision, and the counts prove the riders actually resolved.

// eternalizeRiderTokenNames are the pod's three eternalize carriers: a copy
// token of one of these that entered is one of the abilities' mints.
var eternalizeRiderTokenNames = map[string]bool{
	"Sinuous Striker": true, "Sunscourge Champion": true, "Champion of Wits": true,
}

// eternalizeRiderCounts counts a finished gorge log's rider events.
type eternalizeRiderCounts struct {
	tokens      map[string]int // eternalize tokens that entered, by face name
	mainDraws   int            // draws outside the draw step (only Champion of Wits' rider draws)
	handToGrave int            // hand-to-graveyard moves (the rider's discard and the cost discards)
}

func countEternalizeRiderEvents(g *GorgeSide) (c eternalizeRiderCounts) {
	c.tokens = map[string]int{}
	step := state.StepDraw // the opening hands draw before the first step
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.StepChange:
			step = e.Step
		case events.Draw:
			if step != state.StepDraw {
				c.mainDraws++
			}
		case events.MoveZone:
			if e.From == state.ZHand && e.To == state.ZGraveyard {
				c.handToGrave++
			}
			if e.To == state.ZBattlefield {
				if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken && o.Face() != nil && eternalizeRiderTokenNames[o.Face().Name] {
					c.tokens[o.Face().Name]++
				}
			}
		}
	}
	return c
}

// eternalizeRiderDecks is the pod's deck: the three carriers, Wrath of God to
// fill the graveyard, Pacifism as discard fodder, and only Plains and Islands
// for mana. There is no other draw or discard source, so a main-phase draw is
// Champion of Wits' rider and every hand-to-graveyard move is one of its
// discards or an eternalize cost's.
func eternalizeRiderDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Sinuous Striker", Count: 3},
		{Name: "Sunscourge Champion", Count: 3},
		{Name: "Champion of Wits", Count: 3},
		{Name: "Wrath of God", Count: 3},
		{Name: "Pacifism", Count: 3},
		{Name: "Plains", Count: 12},
		{Name: "Island", Count: 13},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// TestExpandedKeywordsEternalizeRidersLockstep plays the pod over several
// seeds. Each game must agree decision by decision and reach every rider:
// all three carriers eternalize (Sinuous Striker and Sunscourge Champion pay
// their extra discard), and Champion of Wits' enter trigger draws outside the
// draw step and discards.
func TestExpandedKeywordsEternalizeRidersLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	decks := eternalizeRiderDecks()
	var names [2][]string
	for s := range decks {
		for _, e := range decks[s] {
			for i := 0; i < e.Count; i++ {
				names[s] = append(names[s], e.Name)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var sum eternalizeRiderCounts
	sum.tokens = map[string]int{}
	for _, seed := range []uint64{1, 2, 3, 5, 7, 8, 11, 13} {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: names,
			Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 45
		var c eternalizeRiderCounts
		rc.Finish = func(g *GorgeSide) { c = countEternalizeRiderEvents(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		lead, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			kw4Fail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.mainDraws += c.mainDraws
		sum.handToGrave += c.handToGrave
		for name, n := range c.tokens {
			sum.tokens[name] += n
		}
		t.Logf("seed %d: %d decisions, over %v %s; %d draw(s) outside the draw step, %d hand->grave move(s), tokens %v",
			seed, res.Decisions, res.Over, res.Stall, c.mainDraws, c.handToGrave, c.tokens)
	}
	for _, name := range []string{"Sinuous Striker", "Sunscourge Champion", "Champion of Wits"} {
		if sum.tokens[name] == 0 {
			t.Errorf("no %s eternalize token entered", name)
		}
	}
	if sum.mainDraws == 0 {
		t.Error("no Champion of Wits draw rider fired")
	}
	// Every Sinuous Striker or Sunscourge Champion token cost one discarded
	// card beside the self-exile, so the log must hold at least that many
	// hand-to-graveyard moves (the Champion rider's own discards add more).
	if costDiscards := sum.tokens["Sinuous Striker"] + sum.tokens["Sunscourge Champion"]; sum.handToGrave < costDiscards {
		t.Errorf("%d hand->graveyard move(s) for %d eternalize token(s) that each discard a card",
			sum.handToGrave, costDiscards)
	}
}
