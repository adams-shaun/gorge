package azmcts

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// twoSeatSumLeaf is the pre-multiplayer heuristic leaf verbatim: every other
// seat's score subtracted, no Lost term. With two seats it is the equality
// oracle for the new max/skip leaf.
func twoSeatSumLeaf(e *rules.Engine, actor state.PlayerID) float64 {
	g := e.G
	if g.Over {
		switch {
		case g.Draw:
			return 0.5
		case int(g.Winner) < len(g.Players) && g.Winner == actor:
			return 1
		default:
			return 0
		}
	}
	value := 0.0
	for i := range g.Players {
		p := &g.Players[i]
		score := float64(p.Life) + 2*float64(len(g.Zone(state.ZHand, p.ID)))
		for _, id := range g.Zone(state.ZBattlefield, p.ID) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Ephemeral() || o.PhasedOut {
				continue
			}
			score += leafMaterial(e, o, actor)
		}
		if p.ID == actor {
			value += score
		} else {
			value -= score
		}
	}
	return 1 / (1 + math.Exp(-value/20))
}

// TestTwoSeatLeafUnchangedAtRecordedPositions replays whole two-seat bot games
// and requires the new leaf to equal twoSeatSumLeaf bit for bit at every
// recorded position: with one living opponent, max and sum coincide.
func TestTwoSeatLeafUnchangedAtRecordedPositions(t *testing.T) {
	pairs := [][2]string{{"uw-tempo", "mono-red-prowess"}, {"mono-white-equipment", "mono-green-stompy"}}
	checked := 0
	compare := func(e *rules.Engine, where string) {
		t.Helper()
		for actor := state.PlayerID(0); actor < 2; actor++ {
			got, want := heuristicLeafValue(e, actor), twoSeatSumLeaf(e, actor)
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Fatalf("%s actor %d: new leaf %v, two-seat sum leaf %v", where, actor, got, want)
			}
			checked++
		}
	}
	for gi, pr := range pairs {
		cfg := testConfig(t, pr[0], pr[1], testSeed+uint64(gi))
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(cfg.Seed, 2)
		board := botpolicy.NewBoard(2)
		for step := 0; step < 3000 && !e.G.Over; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			compare(e, pr[0]+" vs "+pr[1])
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
		compare(e, pr[0]+" vs "+pr[1]+" final")
	}
	if checked < 300 {
		t.Fatalf("precondition: only %d two-seat leaves compared", checked)
	}
}

// TestHeuristicLeafPodStrongestLivingOpponent pins the exact four-seat value:
// the actor leads one living opponent and trails another, while a lost
// opponent's higher score is skipped.
func TestHeuristicLeafPodStrongestLivingOpponent(t *testing.T) {
	g := state.NewGameLife([]string{"a", "b", "c", "d"}, 40)
	g.Players[1].Life = 30
	g.Players[2].Life = 45
	g.Players[3].Life = 100
	g.Players[3].Lost = true
	if g.Over {
		t.Fatal("precondition: the pod position must be non-terminal")
	}
	if !(g.Players[0].Life > g.Players[1].Life && g.Players[0].Life < g.Players[2].Life) {
		t.Fatal("precondition: the actor must lead seat 1 and trail seat 2")
	}
	e := &rules.Engine{G: g}
	got := heuristicLeafValue(e, 0)
	want := 1 / (1 + math.Exp(-(40.0-45.0)/20))
	if math.Float64bits(got) != math.Float64bits(want) {
		t.Fatalf("pod leaf %v, want %v (life minus the strongest living opponent)", got, want)
	}
}

// TestHeuristicLeafLostActorIsZero pins a lost actor at 0 even though the
// pod plays on for the other seats.
func TestHeuristicLeafLostActorIsZero(t *testing.T) {
	g := state.NewGameLife([]string{"a", "b", "c", "d"}, 40)
	g.Players[0].Lost = true
	if g.Over {
		t.Fatal("precondition: a lost actor in a running pod is not g.Over")
	}
	e := &rules.Engine{G: g}
	if got := heuristicLeafValue(e, 0); got != 0 {
		t.Fatalf("lost actor leaf %v, want 0", got)
	}
}

// TestHeuristicLeafPodMatchesTheView keeps the direct leaf bit-identical to the
// view leaf in a four-seat pod with a lost seat and uneven life totals.
func TestHeuristicLeafPodMatchesTheView(t *testing.T) {
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
	cfg.Names = []string{cfg.Names[0], cfg.Names[1], cfg.Names[0], cfg.Names[1]}
	cfg.Decks = [][]*cards.Card{cfg.Decks[0], cfg.Decks[1], cfg.Decks[0], cfg.Decks[1]}
	e := rules.New(cfg)
	e.Advance()
	c := e.Clone()
	c.G.Players[1].Life = 12
	c.G.Players[2].Life = 55
	c.G.Players[3].Lost = true
	for actor := state.PlayerID(0); int(actor) < len(c.G.Players); actor++ {
		got := heuristicLeafValue(c, actor)
		want := searchprobe.LeafValue(view.Project(c.G, c, actor, c.Pending()), actor)
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Fatalf("actor %d: direct leaf %v, view leaf %v", actor, got, want)
		}
	}
}
