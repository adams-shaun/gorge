package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func card(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

func mountainDeck(t *testing.T, n int) []*cards.Card {
	m := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	out := make([]*cards.Card, n)
	for i := range out {
		out[i] = m
	}
	return out
}

func newSeats(t *testing.T, n int) *Engine {
	t.Helper()
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	e := New(Config{Seed: 42, Names: names, Decks: decks})
	e.Advance()
	return e
}

// passAll answers the pending decision with its "pass" option until the engine
// asks something that is not a priority decision, or the game ends.
func passAll(t *testing.T, e *Engine, limit int) int {
	t.Helper()
	n := 0
	for ; n < limit && !e.G.Over; n++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			return n
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	return n
}

func TestOpeningHandsAndStartingLife(t *testing.T) {
	e := newSeats(t, 4)
	for p := state.PlayerID(0); p < 4; p++ {
		if got := len(e.G.Zone(state.ZHand, p)); got != 7 {
			t.Errorf("player %d hand = %d, want 7", p, got)
		}
		if got := len(e.G.Zone(state.ZLibrary, p)); got != 33 {
			t.Errorf("player %d library = %d, want 33", p, got)
		}
		if e.G.Players[p].Life != 20 {
			t.Errorf("player %d life = %d", p, e.G.Players[p].Life)
		}
	}
}

func TestPriorityVisitsEverySeatInAPNAPOrder(t *testing.T) {
	e := newSeats(t, 4)
	var seen []state.PlayerID
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("decision %d = %+v", i, d)
		}
		seen = append(seen, d.Player)
		idx := 0
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}})
	}
	active := e.G.Active
	for i, p := range seen {
		want := state.PlayerID((int(active) + i) % 4)
		if p != want {
			t.Fatalf("priority order %v, wanted APNAP from seat %d", seen, active)
		}
	}
}

func TestPriorityRoundAdvancesTheStep(t *testing.T) {
	e := newSeats(t, 4)
	start := e.G.Step
	passAll(t, e, 4)
	if e.G.Step == start {
		t.Fatalf("step did not advance past %s after a full pass round", start)
	}
}

func TestTurnsRotateThroughEverySeat(t *testing.T) {
	e := newSeats(t, 4)
	seen := map[state.PlayerID]bool{}
	for i := 0; i < 4000 && len(seen) < 4; i++ {
		seen[e.G.Active] = true
		if passAll(t, e, 1) == 0 {
			// A non-priority decision (attackers with no creatures cannot
			// happen here, so this means the game ended).
			break
		}
	}
	if len(seen) != 4 {
		t.Fatalf("only seats %v ever became active", seen)
	}
}

func TestEliminatedSeatsAreSkipped(t *testing.T) {
	e := newSeats(t, 4)
	e.emit(events.Event{Kind: events.PlayerLost, Player: 1, Text: "test"})
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Player == 1 {
			t.Fatalf("eliminated seat 1 was given a decision: %+v", d)
		}
		idx := 0
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}})
	}
}

func TestSameSeedSameOpeningHands(t *testing.T) {
	a, b := newSeats(t, 4), newSeats(t, 4)
	for p := state.PlayerID(0); p < 4; p++ {
		ah, bh := a.G.Zone(state.ZHand, p), b.G.Zone(state.ZHand, p)
		for i := range ah {
			if ah[i] != bh[i] {
				t.Fatalf("seat %d opening hand differs between runs at the same seed", p)
			}
		}
	}
	if a.L.Head() != b.L.Head() {
		t.Fatalf("chain heads differ: %s vs %s", a.L.Head(), b.L.Head())
	}
}
