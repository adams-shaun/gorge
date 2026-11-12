package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Idol of Oblivion, Bennie Bracks and Thalisse read "a token you created this
// turn" through Count$ThisTurnEntered_Battlefield_Card.tokenCreated, gorge's
// IsToken marker over the per-add entry list. Random games of two decks built
// around them (token makers, removal to kill a token the turn it was made, a
// Serra Angel to fly over) must agree decision by decision, and must reach
// each card's payoff: a draw outside the draw step (Idol at main, Bennie at
// the end step) and a Spirit token Thalisse made at the end step.
func TestTokenCreatedThisTurnLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	decks := [2][]cardset.Entry{
		{
			{Name: "Idol of Oblivion", Count: 4}, {Name: "Bennie Bracks, Zoologist", Count: 4},
			{Name: "Thalisse, Reverent Medium", Count: 4}, {Name: "Raise the Alarm", Count: 4},
			{Name: "Hunted Witness", Count: 3}, {Name: "Doom Blade", Count: 3},
			{Name: "Go for the Throat", Count: 2}, {Name: "Serra Angel", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Idol of Oblivion", Count: 2}, {Name: "Bennie Bracks, Zoologist", Count: 2},
			{Name: "Thalisse, Reverent Medium", Count: 2}, {Name: "Raise the Alarm", Count: 4},
			{Name: "Hunted Witness", Count: 4}, {Name: "Doom Blade", Count: 3},
			{Name: "Go for the Throat", Count: 3}, {Name: "Serra Angel", Count: 2},
			{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 6},
		},
	}
	var names [2][]string
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	mainDraws, spirits, evAt := 0, 0, 0
	step := state.StepDraw // the opening hands draw before the first step
	check := func(ref *GorgeSide, _ Side) *Divergence {
		evs := ref.E.L.Events
		if len(evs) < evAt {
			evAt, step = 0, state.StepDraw
		}
		for _, e := range evs[evAt:] {
			switch e.Kind {
			case events.StepChange:
				step = e.Step
			case events.Draw:
				if step != state.StepDraw {
					mainDraws++
				}
			case events.TokenCreate:
				if strings.Contains(e.Text, "spirit") {
					spirits++
				}
			}
		}
		evAt = len(evs)
		return nil
	}
	games := 0
	for seed := uint64(1); seed <= 150; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		evAt, step = 0, state.StepDraw
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		games++
	}
	t.Logf("%d games: %d draws outside the draw step, %d Thalisse spirits", games, mainDraws, spirits)
	if mainDraws < 20 || spirits < 5 {
		t.Fatalf("the fixture does not reach the payoffs: %d draws outside the draw step, %d spirits", mainDraws, spirits)
	}
}
